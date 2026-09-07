package transport

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	"github.com/gorilla/websocket"
)

// IncomingCommand representa una orden de control enviada desde el servidor.
type IncomingCommand struct {
	ID          string `json:"id"`
	Action      string `json:"action"`
	ContainerID string `json:"container_id"`
	Timestamp   int64  `json:"timestamp"`
}

// CommandAck representa la confirmación y resultado de la ejecución enviada al servidor.
type CommandAck struct {
	ID        string `json:"id"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// WSClient gestiona el canal WebSocket reverso saliente hacia FastAPI.
type WSClient struct {
	serverURL string
	hostID    string
	secretKey string
	collector ports.CollectorPort
	connLock  sync.Mutex
	conn      *websocket.Conn
}

// NewWSClient crea una nueva instancia del cliente WebSocket reverso.
func NewWSClient(serverURL, hostID, secretKey string, collector ports.CollectorPort) *WSClient {
	return &WSClient{
		serverURL: serverURL,
		hostID:    hostID,
		secretKey: secretKey,
		collector: collector,
	}
}

// Start ejecuta el bucle de conexión y auto-reconexión con backoff exponencial.
func (c *WSClient) Start(ctx context.Context) {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			c.closeConn()
			return
		default:
		}

		err := c.connectAndListen(ctx)
		if err != nil && ctx.Err() == nil {
			log.Printf("[WS-CONTROL] Canal reverso desconectado: %v. Reintentando en %v...", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			// Si la conexión cerró limpiamente, reiniciar backoff
			backoff = 1 * time.Second
		}
	}
}

func (c *WSClient) connectAndListen(ctx context.Context) error {
	wsEndpoint, err := c.buildWebSocketURL()
	if err != nil {
		return fmt.Errorf("error construyendo URL WebSocket: %w", err)
	}

	ts := time.Now().Unix()
	signature := c.generateHMAC(fmt.Sprintf("%s:%d", c.hostID, ts))

	headers := http.Header{}
	headers.Set("X-Solv-Signature", signature)
	headers.Set("X-Solv-Timestamp", fmt.Sprintf("%d", ts))

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.DialContext(ctx, wsEndpoint, headers)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	c.setConn(conn)
	defer c.closeConn()

	log.Printf("[WS-CONTROL] Canal reverso conectado con el Control Plane para host '%s'", c.hostID)

	// Iniciar heartbeat ping cada 20 segundos
	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()

	errChan := make(chan error, 2)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-pingTicker.C:
				c.connLock.Lock()
				err := conn.WriteControl(websocket.PingMessage, []byte("solv-ping"), time.Now().Add(5*time.Second))
				c.connLock.Unlock()
				if err != nil {
					errChan <- fmt.Errorf("ping failed: %w", err)
					return
				}
			}
		}
	}()

	go func() {
		for {
			var cmd IncomingCommand
			err := conn.ReadJSON(&cmd)
			if err != nil {
				errChan <- err
				return
			}
			go c.handleCommand(ctx, cmd)
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

func (c *WSClient) handleCommand(ctx context.Context, cmd IncomingCommand) {
	log.Printf("[WS-CONTROL] Orden de control recibida: %s en contenedor '%s' (ID: %s)", cmd.Action, cmd.ContainerID, cmd.ID)

	remCmd := domain.RemediationCommand{
		HostID:      c.hostID,
		ContainerID: cmd.ContainerID,
		Action:      domain.ActionType(cmd.Action),
		Timestamp:   cmd.Timestamp,
	}

	ack := CommandAck{
		ID:        cmd.ID,
		Timestamp: time.Now().Unix(),
	}

	// Validar contra lista blanca estricta (Cero RCE)
	if err := remCmd.Validate(); err != nil {
		log.Printf("[WS-CONTROL] Orden rechazada por violar lista blanca (D1): %v", err)
		ack.Success = false
		ack.Error = err.Error()
		ack.Message = "Acción prohibida por el modelo de seguridad del Data Plane"
		c.sendAck(ack)
		return
	}

	// Ejecutar acción autorizada en el Collector
	start := time.Now()
	err := c.collector.ExecuteRemediation(ctx, remCmd)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("[WS-CONTROL] Error ejecutando %s en '%s': %v", cmd.Action, cmd.ContainerID, err)
		ack.Success = false
		ack.Error = err.Error()
		ack.Message = fmt.Sprintf("Fallo en ejecución: %v", err)
	} else {
		log.Printf("[WS-CONTROL] Remediacion '%s' ejecutada exitosamente en %v", cmd.Action, elapsed)
		ack.Success = true
		ack.Message = fmt.Sprintf("Accion '%s' ejecutada exitosamente en %v", cmd.Action, elapsed.Round(time.Millisecond))
	}

	c.sendAck(ack)
}

func (c *WSClient) sendAck(ack CommandAck) {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	if c.conn != nil {
		_ = c.conn.WriteJSON(ack)
	}
}

func (c *WSClient) buildWebSocketURL() (string, error) {
	parsed, err := url.Parse(c.serverURL)
	if err != nil {
		return "", err
	}

	scheme := "ws"
	if parsed.Scheme == "https" || parsed.Scheme == "wss" {
		scheme = "wss"
	}

	path := parsed.Path
	if !strings.HasSuffix(path, "/") && path != "" {
		path += "/"
	}
	// Si el URL base ya tiene path o no
	if !strings.Contains(path, "api/v1/ws") {
		path = "/api/v1/ws/agent/" + c.hostID
	}

	endpoint := fmt.Sprintf("%s://%s%s", scheme, parsed.Host, path)
	return endpoint, nil
}

func (c *WSClient) generateHMAC(payload string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *WSClient) setConn(conn *websocket.Conn) {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	c.conn = conn
}

func (c *WSClient) closeConn() {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}
