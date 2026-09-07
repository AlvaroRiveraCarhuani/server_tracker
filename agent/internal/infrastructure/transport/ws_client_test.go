package transport_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/transport"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type mockCollector struct {
	executedCmds []domain.RemediationCommand
	returnErr    error
}

func (m *mockCollector) Collect(ctx context.Context) ([]domain.ContainerMetric, error) {
	return nil, nil
}

func (m *mockCollector) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	return "sample log line", nil
}

func (m *mockCollector) ExecuteRemediation(ctx context.Context, cmd domain.RemediationCommand) error {
	m.executedCmds = append(m.executedCmds, cmd)
	return m.returnErr
}

func TestWSClient_HandshakeAndCommandExecution(t *testing.T) {
	secretKey := "test-ws-secret-123"
	hostID := "test-agent-host"

	cmdReceived := make(chan transport.CommandAck, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validar headers HMAC
		sig := r.Header.Get("X-Solv-Signature")
		ts := r.Header.Get("X-Solv-Timestamp")
		if sig == "" || ts == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}

		// Validar HMAC
		mac := hmac.New(sha256.New, []byte(secretKey))
		mac.Write([]byte(fmt.Sprintf("%s:%s", hostID, ts)))
		expected := hex.EncodeToString(mac.Sum(nil))
		if sig != expected {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Enviar comando de prueba al agente
		cmd := transport.IncomingCommand{
			ID:          "cmd-test-1",
			Action:      "restart",
			ContainerID: "container-abc",
			Timestamp:   time.Now().Unix(),
		}
		if err := conn.WriteJSON(cmd); err != nil {
			t.Errorf("write json error: %v", err)
			return
		}

		// Leer confirmación del agente
		var ack transport.CommandAck
		if err := conn.ReadJSON(&ack); err != nil {
			t.Errorf("read ack error: %v", err)
			return
		}
		cmdReceived <- ack
	}))
	defer server.Close()

	collector := &mockCollector{}
	wsClient := transport.NewWSClient(server.URL, hostID, secretKey, collector)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go wsClient.Start(ctx)

	select {
	case ack := <-cmdReceived:
		if ack.ID != "cmd-test-1" {
			t.Errorf("expected cmd-test-1, got %s", ack.ID)
		}
		if !ack.Success {
			t.Errorf("expected success true, got false. err: %s", ack.Error)
		}
		if len(collector.executedCmds) != 1 {
			t.Fatalf("expected 1 executed command, got %d", len(collector.executedCmds))
		}
		if collector.executedCmds[0].Action != domain.ActionRestart {
			t.Errorf("expected action restart, got %s", collector.executedCmds[0].Action)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for command execution and ack")
	}
}

func TestWSClient_RejectUnauthorizedAction(t *testing.T) {
	secretKey := "test-ws-secret-123"
	hostID := "test-agent-host"

	cmdReceived := make(chan transport.CommandAck, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Enviar comando NO permitido (intento de RCE arbitrario)
		cmd := transport.IncomingCommand{
			ID:          "cmd-malicious-1",
			Action:      "exec_bash",
			ContainerID: "container-abc",
			Timestamp:   time.Now().Unix(),
		}
		_ = conn.WriteJSON(cmd)

		var ack transport.CommandAck
		_ = conn.ReadJSON(&ack)
		cmdReceived <- ack
	}))
	defer server.Close()

	collector := &mockCollector{}
	wsClient := transport.NewWSClient(server.URL, hostID, secretKey, collector)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go wsClient.Start(ctx)

	select {
	case ack := <-cmdReceived:
		if ack.Success {
			t.Error("expected unauthorized action to fail, but succeeded")
		}
		if !strings.Contains(ack.Error, "prohibida") && !strings.Contains(ack.Error, "permitida") {
			t.Errorf("expected error mentioning unauthorized action, got: %s", ack.Error)
		}
		if len(collector.executedCmds) != 0 {
			t.Errorf("collector should not execute malicious actions, executed: %d", len(collector.executedCmds))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for command rejection ack")
	}
}
