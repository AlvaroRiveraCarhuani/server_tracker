package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestRemediateContainerUseCase_AllowedActions(t *testing.T) {
	tests := []struct {
		name   string
		action domain.ActionType
	}{
		{"Restart", domain.ActionRestart},
		{"Stop", domain.ActionStop},
		{"IsolateNetwork", domain.ActionIsolateNetwork},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executed := false
			collector := &mockCollectorPort{
				executeRemediationFn: func(ctx context.Context, cmd domain.RemediationCommand) error {
					if cmd.Action != tt.action {
						t.Errorf("expected action %s, got %s", tt.action, cmd.Action)
					}
					executed = true
					return nil
				},
			}

			uc := NewRemediateContainerUseCase(collector)
			err := uc.Execute(context.Background(), domain.RemediationCommand{
				ContainerID: "cont-1",
				Action:      tt.action,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !executed {
				t.Fatalf("expected command execution")
			}
		})
	}
}

func TestRemediateContainerUseCase_DisallowedAction_ZeroRCE(t *testing.T) {
	collector := &mockCollectorPort{
		executeRemediationFn: func(ctx context.Context, cmd domain.RemediationCommand) error {
			t.Fatalf("collector should NOT be called for disallowed action")
			return nil
		},
	}

	uc := NewRemediateContainerUseCase(collector)
	err := uc.Execute(context.Background(), domain.RemediationCommand{
		ContainerID: "cont-1",
		Action:      domain.ActionType("exec_malicious_bash"),
	})

	if err == nil {
		t.Fatalf("expected error for disallowed action, got nil")
	}
	if !errors.Is(err, ErrDisallowedAction) {
		t.Errorf("expected ErrDisallowedAction, got %v", err)
	}
}

func TestRemediateContainerUseCase_ValidationErrors(t *testing.T) {
	ucNil := NewRemediateContainerUseCase(nil)
	errNil := ucNil.Execute(context.Background(), domain.RemediationCommand{ContainerID: "c1", Action: domain.ActionRestart})
	if !errors.Is(errNil, ErrNilCollector) {
		t.Errorf("expected ErrNilCollector, got %v", errNil)
	}

	collector := &mockCollectorPort{}
	uc := NewRemediateContainerUseCase(collector)
	errEmpty := uc.Execute(context.Background(), domain.RemediationCommand{ContainerID: "  ", Action: domain.ActionRestart})
	if !errors.Is(errEmpty, ErrEmptyContainerID) {
		t.Errorf("expected ErrEmptyContainerID, got %v", errEmpty)
	}
}
