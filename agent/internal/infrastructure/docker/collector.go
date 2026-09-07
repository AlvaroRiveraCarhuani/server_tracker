package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	dockertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// CalculateCPUPercent computa el porcentaje de CPU descartando deltas <= 0 (D4).
func CalculateCPUPercent(stats *dockertypes.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)

	if systemDelta <= 0.0 || cpuDelta <= 0.0 {
		return 0.0
	}

	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0.0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
		if onlineCPUs == 0.0 {
			onlineCPUs = 1.0
		}
	}

	return (cpuDelta / systemDelta) * onlineCPUs * 100.0
}

// CalculateRealRAM descuenta la memoria caché inactiva (inactive_file) para reflejar consumo real.
func CalculateRealRAM(stats *dockertypes.StatsResponse) (realRAM uint64, limit uint64) {
	usage := stats.MemoryStats.Usage
	limit = stats.MemoryStats.Limit

	var inactiveFile uint64
	if val, ok := stats.MemoryStats.Stats["inactive_file"]; ok {
		inactiveFile = val
	} else if val, ok := stats.MemoryStats.Stats["total_inactive_file"]; ok {
		inactiveFile = val
	}

	if usage > inactiveFile {
		realRAM = usage - inactiveFile
	} else {
		realRAM = usage
	}

	return realRAM, limit
}

// CalculateNetworkTotals suma tx_bytes y rx_bytes acumulados en todas las interfaces del contenedor.
func CalculateNetworkTotals(stats *dockertypes.StatsResponse) (rxBytes uint64, txBytes uint64) {
	for _, v := range stats.Networks {
		rxBytes += v.RxBytes
		txBytes += v.TxBytes
	}
	return rxBytes, txBytes
}

type containerPrevState struct {
	txBytes   uint64
	rxBytes   uint64
	timestamp time.Time
}

// DockerCollector implementa ports.CollectorPort interactuando con el socket Unix nativo.
type DockerCollector struct {
	cli        *client.Client
	prevStates map[string]containerPrevState
}

// NewDockerCollector inicializa la conexión con /var/run/docker.sock.
func NewDockerCollector() (*DockerCollector, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error conectando al socket de Docker: %w", err)
	}

	return &DockerCollector{
		cli:        cli,
		prevStates: make(map[string]containerPrevState),
	}, nil
}

// Collect extrae el estado y estadísticas de todos los contenedores en ejecución.
func (d *DockerCollector) Collect(ctx context.Context) ([]domain.ContainerMetric, error) {
	containers, err := d.cli.ContainerList(ctx, dockertypes.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("error listando contenedores Docker: %w", err)
	}

	var metrics []domain.ContainerMetric
	now := time.Now()

	for _, c := range containers {
		if c.State != "running" {
			metrics = append(metrics, domain.ContainerMetric{
				ID:        c.ID[:12],
				Name:      strings.TrimPrefix(c.Names[0], "/"),
				Image:     c.Image,
				Status:    c.State,
				Timestamp: now,
			})
			continue
		}

		statsBody, err := d.cli.ContainerStatsOneShot(ctx, c.ID)
		if err != nil {
			continue
		}

		var stats dockertypes.StatsResponse
		if err := json.NewDecoder(statsBody.Body).Decode(&stats); err != nil {
			statsBody.Body.Close()
			continue
		}
		statsBody.Body.Close()

		cpuPct := CalculateCPUPercent(&stats)
		realRAM, limitRAM := CalculateRealRAM(&stats)
		rxTot, txTot := CalculateNetworkTotals(&stats)

		var egressSec, ingressSec float64
		if prev, ok := d.prevStates[c.ID]; ok {
			elapsed := now.Sub(prev.timestamp).Seconds()
			if elapsed > 0 {
				if txTot >= prev.txBytes {
					egressSec = float64(txTot-prev.txBytes) / elapsed
				}
				if rxTot >= prev.rxBytes {
					ingressSec = float64(rxTot-prev.rxBytes) / elapsed
				}
			}
		}

		d.prevStates[c.ID] = containerPrevState{
			txBytes:   txTot,
			rxBytes:   rxTot,
			timestamp: now,
		}

		metrics = append(metrics, domain.ContainerMetric{
			ID:              c.ID[:12],
			Name:            strings.TrimPrefix(c.Names[0], "/"),
			Image:           c.Image,
			Status:          c.State,
			CPUPercent:      cpuPct,
			RAMBytes:        realRAM,
			RAMLimitBytes:   limitRAM,
			EgressBytesSec:  egressSec,
			IngressBytesSec: ingressSec,
			PIDs:            stats.PidsStats.Current,
			Timestamp:       now,
		})
	}

	return metrics, nil
}

// ExecuteRemediation ejecuta acciones de remediación validadas sin permitir comandos arbitrarios (D1).
func (d *DockerCollector) ExecuteRemediation(ctx context.Context, cmd domain.RemediationCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}

	switch cmd.Action {
	case domain.ActionRestart:
		timeoutSec := 10
		return d.cli.ContainerRestart(ctx, cmd.ContainerID, dockertypes.StopOptions{Timeout: &timeoutSec})
	case domain.ActionStop:
		timeoutSec := 10
		return d.cli.ContainerStop(ctx, cmd.ContainerID, dockertypes.StopOptions{Timeout: &timeoutSec})
	case domain.ActionIsolateNetwork:
		// Desconectar el contenedor de su red principal
		inspect, err := d.cli.ContainerInspect(ctx, cmd.ContainerID)
		if err != nil {
			return err
		}
		for netName := range inspect.NetworkSettings.Networks {
			_ = d.cli.NetworkDisconnect(ctx, netName, cmd.ContainerID, true)
		}
		return nil
	default:
		return domain.ErrUnauthorizedAction
	}
}

// Ensure interface satisfaction
var _ ports.CollectorPort = (*DockerCollector)(nil)
