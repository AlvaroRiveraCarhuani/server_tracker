package ports

import (
	"context"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

// CollectorPort define la interfaz para recolectar métricas del socket de Docker.
type CollectorPort interface {
	Collect(ctx context.Context) ([]domain.ContainerMetric, error)
	GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error)
	ExecuteRemediation(ctx context.Context, cmd domain.RemediationCommand) error
}

// VaultPort define la interfaz para almacenamiento y lectura de credenciales.
type VaultPort interface {
	Save(serverURL, secretToken string) error
	Get() (serverURL string, secretToken string, err error)
	SaveOpenRouterKey(key string) error
	GetOpenRouterKey() (string, error)
	SaveAIConfig(cfg domain.AIConfig) error
	GetAIConfig() (domain.AIConfig, error)
}


// BufferPort define el buffer circular en memoria para retención de telemetría.
type BufferPort interface {
	Push(telemetry domain.HostTelemetry) error
	Pop() (domain.HostTelemetry, bool)
	Drain() []domain.HostTelemetry
	Len() int
}

// TransportPort define el canal saliente seguro con firma HMAC.
type TransportPort interface {
	Send(ctx context.Context, telemetry domain.HostTelemetry) error
}

// TriagePort define la interfaz para análisis y diagnóstico asistido por IA.
type TriagePort interface {
	DiagnoseContainer(ctx context.Context, name, image, status, logs string) string
	DiagnoseContainerWithUsage(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage)
}
