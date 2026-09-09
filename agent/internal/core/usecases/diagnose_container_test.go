package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

type mockCollectorPort struct {
	collectFn            func(ctx context.Context) ([]domain.ContainerMetric, error)
	getContainerLogsFn   func(ctx context.Context, containerID string, tail int) (string, error)
	executeRemediationFn func(ctx context.Context, cmd domain.RemediationCommand) error
}

func (m *mockCollectorPort) Collect(ctx context.Context) ([]domain.ContainerMetric, error) {
	if m.collectFn != nil {
		return m.collectFn(ctx)
	}
	return nil, nil
}

func (m *mockCollectorPort) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	if m.getContainerLogsFn != nil {
		return m.getContainerLogsFn(ctx, containerID, tail)
	}
	return "", nil
}

func (m *mockCollectorPort) ExecuteRemediation(ctx context.Context, cmd domain.RemediationCommand) error {
	if m.executeRemediationFn != nil {
		return m.executeRemediationFn(ctx, cmd)
	}
	return nil
}

type mockTriagePort struct {
	diagnoseFn          func(ctx context.Context, name, image, status, logs string) string
	diagnoseWithUsageFn func(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage)
}

func (m *mockTriagePort) DiagnoseContainer(ctx context.Context, name, image, status, logs string) string {
	if m.diagnoseFn != nil {
		return m.diagnoseFn(ctx, name, image, status, logs)
	}
	return "mock-diag"
}

func (m *mockTriagePort) DiagnoseContainerWithUsage(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage) {
	if m.diagnoseWithUsageFn != nil {
		return m.diagnoseWithUsageFn(ctx, name, image, status, logs)
	}
	return "mock-diag-usage", domain.TokenUsage{TotalTokens: 100, EstimatedCostUSD: 0.001}
}

func TestDiagnoseContainerUseCase_Success(t *testing.T) {
	collector := &mockCollectorPort{
		getContainerLogsFn: func(ctx context.Context, containerID string, tail int) (string, error) {
			if containerID != "c-123" {
				t.Fatalf("expected container ID 'c-123', got %s", containerID)
			}
			return "error: out of memory", nil
		},
	}

	triage := &mockTriagePort{
		diagnoseWithUsageFn: func(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage) {
			if logs != "error: out of memory" {
				t.Fatalf("expected logs forwarded, got %s", logs)
			}
			return "Causa: OOM en el proceso principal", domain.TokenUsage{TotalTokens: 150, EstimatedCostUSD: 0.0003}
		},
	}

	uc := NewDiagnoseContainerUseCase(collector, triage)
	diag, usage, err := uc.Execute(context.Background(), domain.ContainerMetric{
		ID:     "c-123",
		Name:   "api-service",
		Image:  "my-api:v1",
		Status: "exited (137)",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diag != "Causa: OOM en el proceso principal" {
		t.Errorf("unexpected diag: %s", diag)
	}
	if usage.TotalTokens != 150 {
		t.Errorf("expected 150 tokens, got %d", usage.TotalTokens)
	}
}

func TestDiagnoseContainerUseCase_NilTriagePort(t *testing.T) {
	uc := NewDiagnoseContainerUseCase(nil, nil)
	_, _, err := uc.Execute(context.Background(), domain.ContainerMetric{ID: "c-123"})
	if !errors.Is(err, ErrNilTriageClient) {
		t.Fatalf("expected ErrNilTriageClient, got %v", err)
	}
}
