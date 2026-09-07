package domain

import "time"

// ContainerMetric representa la telemetría calculada de un contenedor Docker en un instante dado.
type ContainerMetric struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Image          string    `json:"image"`
	Status         string    `json:"status"`
	CPUPercent     float64   `json:"cpu_percent"`
	RAMBytes       uint64    `json:"ram_bytes"`
	RAMLimitBytes  uint64    `json:"ram_limit_bytes"`
	EgressBytesSec float64   `json:"egress_bytes_sec"`
	IngressBytesSec float64  `json:"ingress_bytes_sec"`
	PIDs           uint64    `json:"pids"`
	Timestamp      time.Time `json:"timestamp"`
}

// HostTelemetry agrupa las métricas de todos los contenedores de un host.
type HostTelemetry struct {
	HostID     string            `json:"host_id"`
	Timestamp  int64             `json:"timestamp"`
	Containers []ContainerMetric `json:"containers"`
}
