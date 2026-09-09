package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestCollectTelemetryUseCase_Success(t *testing.T) {
	collector := &mockCollectorPort{
		collectFn: func(ctx context.Context) ([]domain.ContainerMetric, error) {
			return []domain.ContainerMetric{
				{ID: "c1", Name: "redis", Status: "running"},
			}, nil
		},
	}

	uc := NewCollectTelemetryUseCase(collector)
	metrics, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 1 || metrics[0].Name != "redis" {
		t.Errorf("unexpected metrics result: %+v", metrics)
	}
}

func TestCollectTelemetryUseCase_NilCollector(t *testing.T) {
	uc := NewCollectTelemetryUseCase(nil)
	_, err := uc.Execute(context.Background())
	if !errors.Is(err, ErrNilCollector) {
		t.Errorf("expected ErrNilCollector, got %v", err)
	}
}
