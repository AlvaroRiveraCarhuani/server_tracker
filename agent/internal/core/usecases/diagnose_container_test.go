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
	diagnoseWithSlotFn  func(ctx context.Context, name, image, status, logs string, slot domain.DiagnosisSlot) (string, domain.TokenUsage)
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

func (m *mockTriagePort) DiagnoseContainerWithSlot(ctx context.Context, name, image, status, logs string, slot domain.DiagnosisSlot) (string, domain.TokenUsage) {
	if m.diagnoseWithSlotFn != nil {
		return m.diagnoseWithSlotFn(ctx, name, image, status, logs, slot)
	}
	return m.DiagnoseContainerWithUsage(ctx, name, image, status, logs)
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

func TestDiagnoseContainerUseCase_ExecuteWithCascade(t *testing.T) {
	collector := &mockCollectorPort{
		getContainerLogsFn: func(ctx context.Context, containerID string, tail int) (string, error) {
			if containerID == "c-net" {
				return "ERROR: dial tcp 10.0.0.1: Connection Refused", nil
			}
			return "", nil
		},
	}

	t.Run("Level 0 AI: Valid structured response", func(t *testing.T) {
		triage := &mockTriagePort{
			diagnoseWithUsageFn: func(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage) {
				return `{"root_cause":"OOM detectado","severity":"critical","suggested_action":"restart","confidence":"high"}`, domain.TokenUsage{TotalTokens: 80}
			},
		}
		uc := NewDiagnoseContainerUseCase(collector, triage)
		res := uc.ExecuteWithCascade(context.Background(), domain.ContainerMetric{
			ID:     "c-1",
			Status: "exited (137)",
		}, false, domain.SelectionAuto)

		if res.Level != domain.LevelAI {
			t.Errorf("expected LevelAI, got %v", res.Level)
		}
		if res.SuggestedAction != "restart" {
			t.Errorf("expected suggested action restart, got %s", res.SuggestedAction)
		}
	})

	t.Run("Level 1 AI~: Raw non-structured output", func(t *testing.T) {
		triage := &mockTriagePort{
			diagnoseWithUsageFn: func(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage) {
				return "Lo siento, como modelo de lenguaje no puedo determinar...", domain.TokenUsage{TotalTokens: 30}
			},
		}
		uc := NewDiagnoseContainerUseCase(collector, triage)
		res := uc.ExecuteWithCascade(context.Background(), domain.ContainerMetric{
			ID:     "c-2",
			Status: "exited (1)",
		}, false, domain.SelectionAuto)

		if res.Level != domain.LevelAIPartial {
			t.Errorf("expected LevelAIPartial, got %v", res.Level)
		}
		if res.SuggestedAction != "none" {
			t.Errorf("expected suggested action none for partial AI, got %s", res.SuggestedAction)
		}
	})

	t.Run("Level 2 RULE: AI failure fallback to rule", func(t *testing.T) {
		triage := &mockTriagePort{
			diagnoseWithUsageFn: func(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage) {
				return "Diagnóstico no configurado (sin API key)", domain.TokenUsage{}
			},
		}
		uc := NewDiagnoseContainerUseCase(collector, triage)
		res := uc.ExecuteWithCascade(context.Background(), domain.ContainerMetric{
			ID:     "c-3",
			Status: "exited (137)",
		}, false, domain.SelectionAuto)

		if res.Level != domain.LevelRule {
			t.Errorf("expected LevelRule fallback, got %v", res.Level)
		}
		if res.SuggestedAction != "restart" {
			t.Errorf("expected restart, got %s", res.SuggestedAction)
		}
	})

	t.Run("Level 2 RULE: Manual mode without forceAI bypasses AI", func(t *testing.T) {
		calledAI := false
		triage := &mockTriagePort{
			diagnoseWithUsageFn: func(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage) {
				calledAI = true
				return `{"root_cause":"Error de red","severity":"warning","suggested_action":"none","confidence":"high"}`, domain.TokenUsage{}
			},
		}
		uc := NewDiagnoseContainerUseCase(collector, triage)
		res := uc.ExecuteWithCascade(context.Background(), domain.ContainerMetric{
			ID:     "c-net",
			Status: "exited (1)",
		}, false, domain.SelectionManual)

		if calledAI {
			t.Error("expected AI NOT to be called in manual mode without forceAI")
		}
		if res.Level != domain.LevelRule {
			t.Errorf("expected LevelRule, got %v", res.Level)
		}
		if res.RootCause != "Servicio dependiente no alcanzable" {
			t.Errorf("unexpected root cause: %s", res.RootCause)
		}

		// When forceAI is true, it should call AI
		resForced := uc.ExecuteWithCascade(context.Background(), domain.ContainerMetric{
			ID:     "c-net",
			Status: "exited (1)",
		}, true, domain.SelectionManual)

		if !calledAI {
			t.Error("expected AI to be called when forceAI is true")
		}
		if resForced.Level != domain.LevelAI {
			t.Errorf("expected LevelAI with forceAI, got %v", resForced.Level)
		}
	})

	t.Run("Level 3 SIG: No AI and no rule match", func(t *testing.T) {
		uc := NewDiagnoseContainerUseCase(collector, nil)
		res := uc.ExecuteWithCascade(context.Background(), domain.ContainerMetric{
			ID:     "c-unknown",
			Status: "exited (42)",
		}, false, domain.SelectionAuto)

		if res.Level != domain.LevelSignal {
			t.Errorf("expected LevelSignal, got %v", res.Level)
		}
		if res.RootCause != "Exit code 42 · sin diagnóstico" {
			t.Errorf("unexpected root cause: %s", res.RootCause)
		}
	})

	t.Run("S2: Critical severity in FAST triggers bounded re-execution in DEEP", func(t *testing.T) {
		fastCalls := 0
		deepCalls := 0

		triage := &mockTriagePort{
			diagnoseWithSlotFn: func(ctx context.Context, name, image, status, logs string, slot domain.DiagnosisSlot) (string, domain.TokenUsage) {
				if slot == domain.SlotDeep {
					deepCalls++
					return `{"root_cause":"Leak crítico confirmado en worker","severity":"critical","suggested_action":"restart","confidence":"high"}`, domain.TokenUsage{TotalTokens: 200}
				}
				fastCalls++
				return `{"root_cause":"Posible OOM crítico","severity":"critical","suggested_action":"restart","confidence":"high"}`, domain.TokenUsage{TotalTokens: 80}
			},
		}

		uc := NewDiagnoseContainerUseCase(collector, triage)
		c := domain.ContainerMetric{
			ID:     "c-crit",
			Status: "exited (137)",
		}

		// Primera llamada: FAST -> DEEP
		res1 := uc.ExecuteWithCascade(context.Background(), c, false, domain.SelectionAuto)

		if fastCalls != 1 {
			t.Errorf("expected 1 FAST call, got %d", fastCalls)
		}
		if deepCalls != 1 {
			t.Errorf("expected 1 DEEP call, got %d", deepCalls)
		}
		if !res1.ReanalyzedDeep {
			t.Errorf("expected ReanalyzedDeep to be true")
		}
		if res1.ProcessNote != "re-analizado en DEEP por severidad crítica" {
			t.Errorf("unexpected ProcessNote: %q", res1.ProcessNote)
		}
		if res1.RootCause != "Leak crítico confirmado en worker" {
			t.Errorf("expected DEEP root cause to replace FAST, got %q", res1.RootCause)
		}

		// Segunda llamada para el mismo evento: no debe re-ejecutar DEEP de nuevo
		_ = uc.ExecuteWithCascade(context.Background(), c, false, domain.SelectionAuto)
		if deepCalls != 1 {
			t.Errorf("expected still 1 DEEP call (deduped), got %d", deepCalls)
		}
	})
}


