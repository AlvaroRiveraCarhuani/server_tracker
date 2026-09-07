package transport_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/transport"
)

func TestHMACSignAndVerify(t *testing.T) {
	secret := "secret_key_12345"
	payload := []byte(`{"host_id":"srv-1","cpu_percent":45.2}`)

	sig := transport.SignPayload(payload, secret)
	if sig == "" {
		t.Fatalf("la firma HMAC no debe ser vacía")
	}

	// 1. Verificación exitosa
	if !transport.VerifySignature(payload, secret, sig) {
		t.Fatalf("la verificación HMAC con la clave correcta debe dar true")
	}

	// 2. Verificación con clave errónea
	if transport.VerifySignature(payload, "wrong_secret", sig) {
		t.Fatalf("la verificación con clave incorrecta debe dar false")
	}

	// 3. Verificación con payload alterado (tampering)
	tamperedPayload := []byte(`{"host_id":"srv-1","cpu_percent":99.9}`)
	if transport.VerifySignature(tamperedPayload, secret, sig) {
		t.Fatalf("la verificación de un payload alterado debe dar false")
	}
}

func TestHTTPTransportClient_Send(t *testing.T) {
	secret := "shared_token_999"
	var receivedSignature string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSignature = r.Header.Get("X-Solv-Signature")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := transport.NewHTTPTransportClient(server.URL, secret)

	telemetry := domain.HostTelemetry{
		HostID:    "node-alpha",
		Timestamp: time.Now().Unix(),
		Containers: []domain.ContainerMetric{
			{ID: "c1", Name: "redis", CPUPercent: 5.5},
		},
	}

	err := client.Send(context.Background(), telemetry)
	if err != nil {
		t.Fatalf("error inesperado en Send(): %v", err)
	}

	if receivedSignature == "" {
		t.Fatalf("la petición enviada no incluyó la cabecera X-Solv-Signature")
	}

	if !transport.VerifySignature(receivedBody, secret, receivedSignature) {
		t.Fatalf("la firma recibida en el servidor de pruebas no coincide con el payload enviado")
	}

	var parsed domain.HostTelemetry
	if err := json.Unmarshal(receivedBody, &parsed); err != nil {
		t.Fatalf("error deserializando payload recibido: %v", err)
	}

	if parsed.HostID != "node-alpha" {
		t.Errorf("esperado HostID node-alpha, recibido %s", parsed.HostID)
	}
}
