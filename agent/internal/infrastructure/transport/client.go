package transport

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
)

// SignPayload calcula la firma HMAC-SHA256 en formato hexadecimal.
func SignPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature valida una firma HMAC-SHA256 con protección contra ataques de temporización.
func VerifySignature(payload []byte, secret string, expectedSignature string) bool {
	computed := SignPayload(payload, secret)
	return hmac.Equal([]byte(computed), []byte(expectedSignature))
}

// HTTPTransportClient implementa ports.TransportPort enviando telemetría firmada.
type HTTPTransportClient struct {
	serverURL  string
	secret     string
	httpClient *http.Client
}

// NewHTTPTransportClient crea una nueva instancia del cliente de transporte saliente.
func NewHTTPTransportClient(serverURL, secret string) *HTTPTransportClient {
	return &HTTPTransportClient{
		serverURL: serverURL,
		secret:    secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send serializa, firma y transmite el lote de telemetría hacia FastAPI.
func (c *HTTPTransportClient) Send(ctx context.Context, telemetry domain.HostTelemetry) error {
	payload, err := json.Marshal(telemetry)
	if err != nil {
		return fmt.Errorf("error serializando telemetría: %w", err)
	}

	signature := SignPayload(payload, c.secret)
	endpoint := fmt.Sprintf("%s/api/v1/telemetry/ingest", c.serverURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("error construyendo petición HTTP: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Solv-Signature", signature)
	req.Header.Set("X-Solv-Timestamp", fmt.Sprintf("%d", telemetry.Timestamp))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error conectando con el servidor Control Plane (%s): %w", c.serverURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("el servidor respondió con código de estado HTTP %d", resp.StatusCode)
	}

	return nil
}

// Ensure interface satisfaction
var _ ports.TransportPort = (*HTTPTransportClient)(nil)
