package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	tea "github.com/charmbracelet/bubbletea"
)

type mockCollectorForTUI struct {
	executedCmds []domain.RemediationCommand
	metrics      []domain.ContainerMetric
	logs         string
}

func (m *mockCollectorForTUI) Collect(ctx context.Context) ([]domain.ContainerMetric, error) {
	return m.metrics, nil
}

func (m *mockCollectorForTUI) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	return m.logs, nil
}

func (m *mockCollectorForTUI) ExecuteRemediation(ctx context.Context, cmd domain.RemediationCommand) error {
	m.executedCmds = append(m.executedCmds, cmd)
	return nil
}

func sampleMetrics() []domain.ContainerMetric {
	return []domain.ContainerMetric{
		{
			ID:             "c-111111",
			Name:           "solv_api",
			Image:          "solv/api:latest",
			Status:         "running",
			CPUPercent:     12.5,
			RAMBytes:       256 * 1024 * 1024,
			RAMLimitBytes:  1024 * 1024 * 1024,
			EgressBytesSec: 10240,
		},
		{
			ID:             "c-222222",
			Name:           "solv_db",
			Image:          "postgres:16-alpine",
			Status:         "running",
			CPUPercent:     5.0,
			RAMBytes:       512 * 1024 * 1024,
			RAMLimitBytes:  2048 * 1024 * 1024,
			EgressBytesSec: 512,
		},
	}
}

func TestTUI_RemediationKeyTransitions(t *testing.T) {
	collector := &mockCollectorForTUI{metrics: sampleMetrics()}
	model := NewModel(collector)
	model.metrics = sampleMetrics()

	// 1. Presionar 'r' sobre el primer contenedor debe pasar a stateConfirmRemediation con ActionRestart
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m := newModel.(Model)
	if m.activeState != stateConfirmRemediation {
		t.Fatalf("expected stateConfirmRemediation, got %v", m.activeState)
	}
	if m.pendingAction != domain.ActionRestart {
		t.Errorf("expected pendingAction restart, got %v", m.pendingAction)
	}
	if m.pendingContainer.Name != "solv_api" {
		t.Errorf("expected container solv_api, got %s", m.pendingContainer.Name)
	}

	// 2. Cancelar con 'n' debe volver a stateFleetTable sin ejecutar nada
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)
	if m.activeState != stateFleetTable {
		t.Errorf("expected stateFleetTable after cancel, got %v", m.activeState)
	}
	if len(collector.executedCmds) != 0 {
		t.Errorf("expected 0 executed commands after cancel, got %d", len(collector.executedCmds))
	}
	if !strings.Contains(m.statusMessage, "cancelada") {
		t.Errorf("expected status message mentioning cancellation, got: %s", m.statusMessage)
	}

	// 3. Probar atajo 's' (stop) y confirmar con 'y'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = newModel.(Model)
	if m.activeState != stateConfirmRemediation || m.pendingAction != domain.ActionStop {
		t.Fatalf("expected confirm stop, got state %v action %v", m.activeState, m.pendingAction)
	}

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = newModel.(Model)
	if m.activeState != stateFleetTable {
		t.Errorf("expected stateFleetTable after confirm, got %v", m.activeState)
	}
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd for async remediation execution")
	}

	// Ejecutar el comando producido
	resultMsg := cmd()
	ack, ok := resultMsg.(remediationResultMsg)
	if !ok {
		t.Fatalf("expected remediationResultMsg, got %T", resultMsg)
	}
	if ack.action != domain.ActionStop {
		t.Errorf("expected ActionStop, got %s", ack.action)
	}
	if len(collector.executedCmds) != 1 {
		t.Errorf("expected 1 command executed in collector, got %d", len(collector.executedCmds))
	}

	// Procesar el mensaje de resultado en el Update
	newModel, _ = m.Update(ack)
	m = newModel.(Model)
	if !strings.Contains(m.statusMessage, "[OK]") {
		t.Errorf("expected status message with [OK], got: %s", m.statusMessage)
	}
}

func TestTUI_ConfirmModalRender(t *testing.T) {
	collector := &mockCollectorForTUI{metrics: sampleMetrics()}
	model := NewModel(collector)
	model.metrics = sampleMetrics()
	model.pendingContainer = sampleMetrics()[0]
	model.pendingAction = domain.ActionIsolateNetwork
	model.activeState = stateConfirmRemediation

	rendered := model.View()
	if !strings.Contains(rendered, "CONFIRMAR REMEDIACION") {
		t.Errorf("expected rendered view to contain CONFIRMAR REMEDIACION, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "ISOLATE_NETWORK") {
		t.Errorf("expected rendered view to contain ISOLATE_NETWORK, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "solv_api") {
		t.Errorf("expected rendered view to contain solv_api, got:\n%s", rendered)
	}
}

func TestTUI_ModalArrowNavigation(t *testing.T) {
	collector := &mockCollectorForTUI{metrics: sampleMetrics()}
	model := NewModel(collector)
	model.metrics = sampleMetrics()

	// 1. Abrir modal con 'r'
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m := newModel.(Model)
	if m.confirmModalBtn != 0 {
		t.Fatalf("expected confirm button focused by default (0), got %d", m.confirmModalBtn)
	}

	// 2. Mover con flecha derecha -> debe seleccionar Cancelar (1)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newModel.(Model)
	if m.confirmModalBtn != 1 {
		t.Fatalf("expected cancel button focused (1) after KeyRight, got %d", m.confirmModalBtn)
	}

	// 3. Presionar Enter sobre Cancelar -> debe cancelar sin ejecutar nada
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if m.activeState != stateFleetTable {
		t.Fatalf("expected stateFleetTable after cancel, got %v", m.activeState)
	}
	if len(collector.executedCmds) != 0 {
		t.Errorf("expected 0 executed commands, got %d", len(collector.executedCmds))
	}
}

func TestTUI_MouseLeftRowSelection(t *testing.T) {
	collector := &mockCollectorForTUI{metrics: sampleMetrics()}
	model := NewModel(collector)
	model.metrics = sampleMetrics()
	model.cursor = 0

	// Simular clic izquierdo en la fila 1 (segundo contenedor: solv_db)
	// Y=3 es fila 0, Y=4 es fila 1
	mouseClick := tea.MouseMsg{
		X:    15,
		Y:    4,
		Type: tea.MouseLeft,
	}

	newModel, _ := model.Update(mouseClick)
	m := newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor to move to 1 on mouse click, got %d", m.cursor)
	}
}

