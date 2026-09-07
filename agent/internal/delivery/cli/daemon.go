package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
)

// RunDaemon ejecuta el bucle de recolección en segundo plano con consumo mínimo de CPU.
func RunDaemon(
	collector ports.CollectorPort,
	vault ports.VaultPort,
	ringBuffer ports.BufferPort,
	transport ports.TransportPort,
	interval time.Duration,
) error {
	hostID, err := os.Hostname()
	if err != nil {
		hostID = "solv-host-unknown"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Printf("[DAEMON] Iniciando SOLV Server Tracker [Host: %s, Intervalo: %v]", hostID, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Ejecutar primera recolección inmediata
	collectAndSend(ctx, hostID, collector, ringBuffer, transport)

	for {
		select {
		case <-sigChan:
			log.Println("[DAEMON] Senal de parada recibida. Finalizando daemon...")
			return nil
		case <-ticker.C:
			collectAndSend(ctx, hostID, collector, ringBuffer, transport)
		}
	}
}

func collectAndSend(
	ctx context.Context,
	hostID string,
	collector ports.CollectorPort,
	ringBuffer ports.BufferPort,
	transport ports.TransportPort,
) {
	metrics, err := collector.Collect(ctx)
	if err != nil {
		log.Printf("[WARN] Error recolectando metricas Docker: %v", err)
		return
	}

	telemetry := domain.HostTelemetry{
		HostID:     hostID,
		Timestamp:  time.Now().Unix(),
		Containers: metrics,
	}

	// 1. Encolar en el RingBuffer
	_ = ringBuffer.Push(telemetry)

	// 2. Intentar drenar y enviar lotes acumulados
	items := ringBuffer.Drain()
	for _, batch := range items {
		sendCtx, sendCancel := context.WithTimeout(ctx, 8*time.Second)
		err := transport.Send(sendCtx, batch)
		sendCancel()

		if err != nil {
			// Si falla la red, volvemos a meter el lote al buffer
			_ = ringBuffer.Push(batch)
			log.Printf("[WARN] Error enviando telemetria al Control Plane (re-encolado en buffer): %v", err)
			break
		}
	}

	fmt.Printf("[%s] Telemetria procesada: %d contenedores vigilados\n",
		time.Now().Format("15:04:05"), len(metrics))
}
