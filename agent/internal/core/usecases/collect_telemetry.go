package usecases

import (
	"context"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
)

// CollectTelemetryUseCase orquesta la recolección de métricas de contenedores desde el socket del host.
type CollectTelemetryUseCase struct {
	collector ports.CollectorPort
}

// NewCollectTelemetryUseCase crea una nueva instancia del caso de uso.
func NewCollectTelemetryUseCase(collector ports.CollectorPort) *CollectTelemetryUseCase {
	return &CollectTelemetryUseCase{
		collector: collector,
	}
}

// Execute obtiene la lista instantánea de métricas de contenedores.
func (uc *CollectTelemetryUseCase) Execute(ctx context.Context) ([]domain.ContainerMetric, error) {
	if uc.collector == nil {
		return nil, ErrNilCollector
	}
	return uc.collector.Collect(ctx)
}
