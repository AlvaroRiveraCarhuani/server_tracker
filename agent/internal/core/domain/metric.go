package domain

import (
	"strings"
	"time"
)

// ContainerMetric representa la telemetría calculada de un contenedor Docker en un instante dado.
type ContainerMetric struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Image          string    `json:"image"`
	Status         string    `json:"status"`
	CPUPercent          float64   `json:"cpu_percent"`
	CPUPercentThrottled float64   `json:"cpu_percent_throttled"`
	RAMBytes            uint64    `json:"ram_bytes"`
	RAMLimitBytes       uint64    `json:"ram_limit_bytes"`
	EgressBytesSec      float64   `json:"egress_bytes_sec"`
	IngressBytesSec     float64   `json:"ingress_bytes_sec"`
	PIDs                uint64    `json:"pids"`
	RestartCount        int       `json:"restart_count"`
	RestartPolicy       string    `json:"restart_policy,omitempty"`
	LastStateChange     time.Time `json:"last_state_change,omitempty"`
	Networks            []string  `json:"networks,omitempty"`
	Ports               []string  `json:"ports,omitempty"`
	EnvVars             []string  `json:"env_vars,omitempty"`
	ComposeProject      string    `json:"compose_project,omitempty"`
	VolumeCount         int       `json:"volume_count,omitempty"`
	IPAddress           string    `json:"ip_address,omitempty"`
	NetworkAliases      []string  `json:"network_aliases,omitempty"`
	Timestamp           time.Time `json:"timestamp"`
}

// IsAnomalous evalúa si un contenedor presenta comportamiento anómalo.
func IsAnomalous(c ContainerMetric) bool {
	if strings.ToLower(c.Status) != "running" {
		return true
	}
	if c.RAMLimitBytes > 0 && float64(c.RAMBytes)/float64(c.RAMLimitBytes) >= 0.85 {
		return true
	}
	return false
}

// HostTelemetry agrupa las métricas de todos los contenedores de un host.
type HostTelemetry struct {
	HostID     string            `json:"host_id"`
	Timestamp  int64             `json:"timestamp"`
	Containers []ContainerMetric `json:"containers"`
}
