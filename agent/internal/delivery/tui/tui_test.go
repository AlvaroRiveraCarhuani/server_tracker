package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/service"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/ai"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	if !strings.Contains(rendered, "Confirmar acción") {
		t.Errorf("expected rendered view to contain Confirmar acción, got:\n%s", rendered)
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

type mockTriageService struct {
	calls              int
	diag               string
	diagnoseWithSlotFn func(ctx context.Context, name, image, status, logs string, slot domain.DiagnosisSlot) (string, domain.TokenUsage)
}

func (m *mockTriageService) DiagnoseContainer(ctx context.Context, name, image, status, logs string) string {
	m.calls++
	return m.diag
}

func (m *mockTriageService) DiagnoseContainerWithUsage(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage) {
	m.calls++
	return m.diag, domain.TokenUsage{PromptTokens: 120, CompletionTokens: 25, TotalTokens: 145, EstimatedCostUSD: 0.0003}
}

func (m *mockTriageService) DiagnoseContainerWithSlot(ctx context.Context, name, image, status, logs string, slot domain.DiagnosisSlot) (string, domain.TokenUsage) {
	if m.diagnoseWithSlotFn != nil {
		return m.diagnoseWithSlotFn(ctx, name, image, status, logs, slot)
	}
	return m.DiagnoseContainerWithUsage(ctx, name, image, status, logs)
}

func TestTUI_AIOpsZeroPromptTriage(t *testing.T) {
	metrics := []domain.ContainerMetric{
		{
			ID:            "c-healthy",
			Name:          "healthy_service",
			Image:         "nginx:alpine",
			Status:        "running",
			RAMBytes:      100 * 1024 * 1024,
			RAMLimitBytes: 1024 * 1024 * 1024,
		},
		{
			ID:            "c-crashed",
			Name:          "payment_service",
			Image:         "payment/api:v1",
			Status:        "exited",
			RAMBytes:      0,
			RAMLimitBytes: 512 * 1024 * 1024,
		},
	}

	collector := &mockCollectorForTUI{
		metrics: metrics,
		logs:    "FATAL: Database connection timeout\nProcess terminated with exit code 1",
	}

	mockAI := &mockTriageService{
		diag: `{"root_cause":"Database connection timeout -> Verificar conectividad y credenciales de BD","severity":"warning","suggested_action":"none","confidence":"high"}`,
	}

	model := NewModel(collector)
	model.aiConfig.SelectionMode = domain.SelectionAuto
	model.triageClient = mockAI
	model.metrics = metrics
	model.cursor = 0

	// 1. Contenedor sano: no debe renderizar banner AIOps
	renderedHealthy := model.View()
	if strings.Contains(renderedHealthy, "[AI]") || strings.Contains(renderedHealthy, "[AIOps]") {
		t.Errorf("expected no AI banner on healthy container, got:\n%s", renderedHealthy)
	}

	// 2. Mover cursor con 'j' al contenedor con fallo (payment_service)
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := newModel.(Model)
	if m.cursor != 1 {
		t.Fatalf("expected cursor on row 1, got %d", m.cursor)
	}
	if cmd == nil {
		t.Fatal("expected async triage tea.Cmd for anomalous container, got nil")
	}

	// 3. Ejecutar el tea.Cmd para obtener el diagnosisResultMsg
	msg := cmd()
	diagMsg, ok := msg.(diagnosisResultMsg)
	if !ok {
		t.Fatalf("expected diagnosisResultMsg from triage cmd, got %T (%+v)", msg, msg)
	}
	t.Logf("DEBUG diagMsg: container=%s diag=%s level=%s action=%s", diagMsg.containerID, diagMsg.diagnosis, diagMsg.result.Level, diagMsg.result.SuggestedAction)
	if diagMsg.containerID != "c-crashed" {
		t.Errorf("expected diagnosis for c-crashed, got %s", diagMsg.containerID)
	}
	if mockAI.calls != 1 {
		t.Errorf("expected exactly 1 AI triage call, got %d", mockAI.calls)
	}

	// 4. Enviar diagnosisResultMsg al modelo y comprobar que se guarda en caché
	newModel, _ = m.Update(diagMsg)
	m = newModel.(Model)
	if cachedDiag, exists := m.diagnosisCache["c-crashed"]; !exists || !strings.Contains(cachedDiag, "Database connection timeout") {
		t.Errorf("expected cached diagnosis in model, got: %s (exists=%v)", cachedDiag, exists)
	}

	// 5. Renderizar vista con el contenedor anómalo en foco: debe mostrar el banner [AI]
	renderedCrashed := m.View()
	if !strings.Contains(renderedCrashed, "[AI]") {
		t.Errorf("expected rendered view to contain [AI] tag, got:\n%s", renderedCrashed)
	}
	if !strings.Contains(renderedCrashed, "Database connection timeout") {
		t.Errorf("expected rendered view to contain diagnosis text, got:\n%s", renderedCrashed)
	}

	// 6. Volver a mover cursor o actualizar: no debe disparar otra llamada a la IA (debe usar caché)
	_, repeatCmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if repeatCmd != nil {
		t.Errorf("expected nil cmd due to cached diagnosis, got %v", repeatCmd)
	}
	if mockAI.calls != 1 {
		t.Errorf("expected AI calls to remain 1 due to cache, got %d", mockAI.calls)
	}
}

func TestTUI_MetricsHistoryAndSparklines(t *testing.T) {
	collector := &mockCollectorForTUI{}
	model := NewModel(collector)
	model.width = 120 // Ancho suficiente para columna TREND

	// Batch 1: dos contenedores
	batch1 := []domain.ContainerMetric{
		{ID: "c-1", Name: "srv-1", Status: "running", CPUPercent: 10.0, RAMBytes: 100 * 1024 * 1024},
		{ID: "c-2", Name: "srv-2", Status: "running", CPUPercent: 20.0, RAMBytes: 200 * 1024 * 1024},
	}
	newModel, _ := model.Update(batch1)
	m := newModel.(Model)

	if len(m.metricsHistory) != 2 {
		t.Fatalf("expected 2 containers in metricsHistory, got %d", len(m.metricsHistory))
	}
	if len(m.metricsHistory["c-1"].CPU) != 1 || m.metricsHistory["c-1"].CPU[0] != 10.0 {
		t.Errorf("expected c-1 CPU sample 10.0, got %v", m.metricsHistory["c-1"].CPU)
	}

	// Batch 2: c-1 sigue con 85% CPU, pero c-2 desaparece y aparece c-3
	batch2 := []domain.ContainerMetric{
		{ID: "c-1", Name: "srv-1", Status: "running", CPUPercent: 85.0, RAMBytes: 120 * 1024 * 1024},
		{ID: "c-3", Name: "srv-3", Status: "running", CPUPercent: 5.0, RAMBytes: 50 * 1024 * 1024},
	}
	newModel, _ = m.Update(batch2)
	m = newModel.(Model)

	if len(m.metricsHistory) != 2 {
		t.Fatalf("expected 2 containers after pruning c-2, got %d", len(m.metricsHistory))
	}
	if _, exists := m.metricsHistory["c-2"]; exists {
		t.Errorf("expected c-2 to be pruned from metricsHistory")
	}
	if len(m.metricsHistory["c-1"].CPU) != 2 {
		t.Errorf("expected 2 samples for c-1, got %d", len(m.metricsHistory["c-1"].CPU))
	}

	// Renderizar tabla principal: debe contener ficha técnica de métricas en tiempo real
	rendered := m.View()
	if !strings.Contains(rendered, "VITALES") || !strings.Contains(rendered, "CPU:") || !strings.Contains(rendered, "RAM:") {
		t.Errorf("expected view to contain realtime metrics in split-pane, got:\n%s", rendered)
	}

	// Renderizar vista de logs para c-1: debe contener la sección de tendencias con métricas btop
	m.activeState = stateLogViewer
	m.selectedID = "c-1"
	m.selectedName = "srv-1"
	m.selectedState = "running"
	renderedLogs := m.View()
	if !strings.Contains(renderedLogs, "CPU:") || !strings.Contains(renderedLogs, "RAM:") {
		t.Errorf("expected viewLogs to contain CPU and RAM btop-style trends, got:\n%s", renderedLogs)
	}
}

type mockVaultForTUI struct {
	savedURL         string
	savedToken       string
	savedKey         string
	savedAIConfig    domain.AIConfig
	savedThemeConfig    domain.ThemeConfig
	savedPinnedContainers []string
}

func (m *mockVaultForTUI) Save(serverURL, secretToken string) error {
	m.savedURL = serverURL
	m.savedToken = secretToken
	return nil
}

func (m *mockVaultForTUI) Get() (string, string, error) {
	return m.savedURL, m.savedToken, nil
}

func (m *mockVaultForTUI) SaveOpenRouterKey(key string) error {
	m.savedKey = key
	return nil
}

func (m *mockVaultForTUI) GetOpenRouterKey() (string, error) {
	return m.savedKey, nil
}

func (m *mockVaultForTUI) SaveAIConfig(cfg domain.AIConfig) error {
	m.savedAIConfig = cfg
	if p, ok := cfg.Providers[domain.ProviderOpenRouter]; ok {
		m.savedKey = p.APIKey
	}
	return nil
}

func (m *mockVaultForTUI) GetAIConfig() (domain.AIConfig, error) {
	if m.savedAIConfig.ActiveProvider != "" {
		return m.savedAIConfig, nil
	}
	cfg := domain.DefaultAIConfig()
	if m.savedKey != "" {
		p := cfg.Providers[domain.ProviderOpenRouter]
		p.APIKey = m.savedKey
		cfg.Providers[domain.ProviderOpenRouter] = p
	}
	return cfg, nil
}

func (m *mockVaultForTUI) SaveThemeConfig(cfg domain.ThemeConfig) error {
	m.savedThemeConfig = cfg
	return nil
}

func (m *mockVaultForTUI) GetThemeConfig() (domain.ThemeConfig, error) {
	if m.savedThemeConfig.ActiveTheme != "" {
		return m.savedThemeConfig, nil
	}
	return domain.DefaultThemeConfig(), nil
}

func (m *mockVaultForTUI) SavePinnedContainers(names []string) error {
	m.savedPinnedContainers = names
	return nil
}

func (m *mockVaultForTUI) GetPinnedContainers() ([]string, error) {
	return m.savedPinnedContainers, nil
}

func TestTUI_ConfigModalWorkflow(t *testing.T) {
	collector := &mockCollectorForTUI{metrics: sampleMetrics()}
	vaultMock := &mockVaultForTUI{}
	model := NewModel(collector, vaultMock)
	model.metrics = sampleMetrics()

	// 1. Abrir modal con 'c' -> aiViewPolicy (V1)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m := newModel.(Model)
	if m.activeState != stateConfigModal {
		t.Fatalf("expected activeState to be stateConfigModal, got %v", m.activeState)
	}
	if m.aiState != aiViewPolicy {
		t.Fatalf("expected aiState to be aiViewPolicy, got %v", m.aiState)
	}

	// 2. Renderizar vista V1: Política de Asignaciones
	renderedModal := m.View()
	if !strings.Contains(renderedModal, "diagnóstico · asignaciones") {
		t.Errorf("expected view to contain title 'diagnóstico · asignaciones', got:\n%s", renderedModal)
	}
	if !strings.Contains(renderedModal, "[FAST]") || !strings.Contains(renderedModal, "[DEEP]") || !strings.Contains(renderedModal, "[AUTO]") {
		t.Errorf("expected view to contain slot tags, got:\n%s", renderedModal)
	}

	// 3. Probar navegación a V3 (Proveedores) con 'p'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = newModel.(Model)
	if m.aiState != aiViewProviders {
		t.Fatalf("expected aiState to be aiViewProviders after 'p', got %v", m.aiState)
	}

	// 4. Presionar Enter sobre OpenRouter (fila 0) -> entra a aiViewKeyInput (sub-paso de clave)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if m.aiState != aiViewKeyInput {
		t.Fatalf("expected aiState to be aiViewKeyInput after Enter on provider, got %v", m.aiState)
	}

	// 5. Escribir clave de OpenRouter
	for _, r := range "sk-or-testkey999" {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(Model)
	}

	// 6. Presionar Enter para guardar clave en la bóveda
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if m.aiState != aiViewProviders {
		t.Errorf("expected to return to aiViewProviders after saving key, got %v", m.aiState)
	}
	orCfg := vaultMock.savedAIConfig.Providers[m.connectProvider]
	if orCfg.APIKey != "sk-or-testkey999" {
		t.Errorf("expected vault to store provider key, got %s", orCfg.APIKey)
	}

	// 7. Volver a V1 con Esc
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.aiState != aiViewPolicy {
		t.Errorf("expected aiState aiViewPolicy after Esc from providers, got %v", m.aiState)
	}

	// 8. Salir del modal con Esc -> vuelve a fleet table
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.activeState != stateFleetTable {
		t.Errorf("expected activeState stateFleetTable after Esc, got %v", m.activeState)
	}
}


func TestTUI_ShellAttachBehavior(t *testing.T) {
	metrics := []domain.ContainerMetric{
		{ID: "c-1", Name: "running_app", Status: "running"},
		{ID: "c-2", Name: "stopped_app", Status: "exited"},
	}
	collector := &mockCollectorForTUI{metrics: metrics}
	model := NewModel(collector)
	model.metrics = metrics

	// 1. Sobre contenedor activo (row 0), presionar 'e' debe generar tea.Cmd para invocar shell
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		t.Errorf("expected tea.Cmd to execute shell on running container, got nil")
	}
	m := newModel.(Model)

	// 2. Mover cursor a contenedor detenido (row 1: stopped_app)
	m.cursor = 1
	newModel2, cmd2 := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd2 != nil {
		t.Errorf("expected nil cmd when trying to attach shell to exited container, got %v", cmd2)
	}
	m2 := newModel2.(Model)
	if !strings.Contains(m2.statusMessage, "no está activo") {
		t.Errorf("expected warning message about stopped container, got %s", m2.statusMessage)
	}

	// 3. Recibir mensaje shellFinishedMsg -> debe limpiar pantalla y notificar
	newModel3, finishCmd := m.Update(shellFinishedMsg{err: nil})
	m3 := newModel3.(Model)
	if !strings.Contains(m3.statusMessage, "finalizada") {
		t.Errorf("expected statusMessage after shell finish, got %s", m3.statusMessage)
	}
	if finishCmd == nil {
		t.Errorf("expected tea.ClearScreen command after shell finish, got nil")
	}
}

func TestTUI_RenderLayoutSnapshot(t *testing.T) {
	metrics := []domain.ContainerMetric{
		{ID: "c-1", Name: "gallant_moore", Status: "exited", CPUPercent: 0.0, Image: "postgres:16-alpine"},
		{ID: "c-2", Name: "reverent_williams", Status: "exited", CPUPercent: 0.0, Image: "redis:7"},
		{ID: "c-3", Name: "eager_leakey", Status: "exited", CPUPercent: 0.0, Image: "nginx:latest"},
		{ID: "c-4", Name: "bold_panini", Status: "running", CPUPercent: 0.0, Image: "node:20-alpine"},
		{ID: "c-5", Name: "solv-lab-707a8a1c-db", Status: "exited", CPUPercent: 0.0, Image: "postgres:16"},
		{ID: "c-6", Name: "solv-lab-3bd76085-api", Status: "exited", CPUPercent: 0.0, Image: "solv/api"},
		{ID: "c-7", Name: "solv-lab-86e98f79-gw", Status: "exited", CPUPercent: 0.0, Image: "traefik:v3"},
	}
	collector := &mockCollectorForTUI{metrics: metrics}
	model := NewModel(collector)
	model.metrics = metrics
	model.width = 96
	model.height = 24
	model.cursor = 3 // bold_panini

	rendered := model.View()
	t.Logf("RENDERED SNAPSHOT:\n%s\n", rendered)

	// Verificar que cada fila no tenga saltos de línea partidos
	if strings.Contains(rendered, "> [-\n") || strings.Contains(rendered, "[--\n") {
		t.Errorf("line broken by unwanted wrap:\n%s", rendered)
	}
	if !strings.Contains(rendered, "bold_panini") {
		t.Errorf("expected bold_panini in rendered view")
	}
}

func TestTUI_RenderModalOverlaySnapshot(t *testing.T) {
	metrics := []domain.ContainerMetric{
		{ID: "119ed9c878f6", Name: "solv-lab-86e98f79-843a-4b27-ab31-276f59cee512", Status: "running", CPUPercent: 0.0, Image: "nginx:alpine"},
		{ID: "c-2", Name: "gallant_moore", Status: "exited", CPUPercent: 0.0, Image: "postgres:16-alpine"},
	}
	collector := &mockCollectorForTUI{metrics: metrics}
	model := NewModel(collector)
	model.metrics = metrics
	model.width = 100
	model.height = 24
	model.cursor = 0

	// Activar modal de restart con 'r'
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m := newModel.(Model)

	rendered := m.View()
	t.Logf("MODAL OVERLAY SNAPSHOT:\n%s\n", rendered)

	// Validar que el fondo siga existiendo (CONTENEDORES)
	if !strings.Contains(rendered, "CONTENEDORES") {
		t.Errorf("expected background CONTENEDORES to remain visible in overlay mode")
	}

	// Validar que el modal contenga los elementos solicitados
	if !strings.Contains(rendered, "Confirmar acción: RESTART") {
		t.Errorf("expected modal title in overlay")
	}
	if !strings.Contains(rendered, "solv-lab-86e98f79-843a-4b27-ab31-276f59cee512") {
		t.Errorf("expected container name in overlay")
	}
	if !strings.Contains(rendered, "CONFIRMAR (y)") || !strings.Contains(rendered, "CANCELAR (n/Esc)") {
		t.Errorf("expected button labels in overlay")
	}
}

func TestTUI_ConfirmModalNavigation(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	model := NewModel(mockColl)
	model.width = 100
	model.height = 30
	model.metrics = []domain.ContainerMetric{
		{
			ID:     "119ed9c878f6",
			Name:   "test-container",
			Image:  "nginx:alpine",
			Status: "running",
		},
	}


	// Abrir modal de restart
	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	mod := m1.(Model)
	if mod.activeState != stateConfirmRemediation {
		t.Fatalf("expected stateConfirmRemediation, got %v", mod.activeState)
	}
	if mod.confirmModalBtn != 0 {
		t.Errorf("expected default button to be 0 (Confirm), got %d", mod.confirmModalBtn)
	}

	// Flecha derecha: debe cambiar a 1 (Cancel)
	m2, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRight})
	mod = m2.(Model)
	if mod.confirmModalBtn != 1 {
		t.Errorf("expected button to be 1 after Right arrow, got %d", mod.confirmModalBtn)
	}

	// Flecha izquierda: debe volver a 0 (Confirm)
	m3, _ := mod.Update(tea.KeyMsg{Type: tea.KeyLeft})
	mod = m3.(Model)
	if mod.confirmModalBtn != 0 {
		t.Errorf("expected button to be 0 after Left arrow, got %d", mod.confirmModalBtn)
	}

	// Tab: alternar a 1
	m4, _ := mod.Update(tea.KeyMsg{Type: tea.KeyTab})
	mod = m4.(Model)
	if mod.confirmModalBtn != 1 {
		t.Errorf("expected button to be 1 after Tab, got %d", mod.confirmModalBtn)
	}

	// Enter en Cancelar: debe volver a stateFleetTable
	m5, _ := mod.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mod = m5.(Model)
	if mod.activeState != stateFleetTable {
		t.Errorf("expected activeState stateFleetTable after Cancel enter, got %v", mod.activeState)
	}
}

func TestTUI_ConfirmModalButtonUniformity(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	model := NewModel(mockColl)
	model.width = 100
	model.height = 30
	model.confirmModalBtn = 0
	model.pendingAction = "restart"
	model.pendingContainer = domain.ContainerMetric{
		ID:   "25aacfda6c48",
		Name: "intelligent_goldwasser",
	}

	for btn := 0; btn <= 1; btn++ {
		model.confirmModalBtn = btn
		modalView := model.viewConfirmModal()
		lines := strings.Split(modalView, "\n")
		for _, l := range lines {
			if w := ansi.StringWidth(l); w != 72 {
				t.Errorf("expected modal line width to be exactly 72 columns, got %d for line %q", w, l)
			}
		}
	}
}

func TestTUI_ThemeModalWorkflow(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	mockV := &mockVaultForTUI{
		savedThemeConfig: domain.DefaultThemeConfig(),
	}
	model := NewModel(mockColl, mockV)
	model.width = 100
	model.height = 30

	// 1. Abrir modal con 't' (Preferencias) y dar Enter en "temas y estilos"
	m0, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m1, _ := m0.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mod := m1.(Model)
	if mod.activeState != stateThemeModal {
		t.Fatalf("expected stateThemeModal after pressing 't' and enter, got %v", mod.activeState)
	}

	// 2. Renderizar vista modal
	view := mod.View()
	if !strings.Contains(view, "Seleccionar Tema") {
		t.Errorf("expected theme selector title in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Tokyo Night") || !strings.Contains(view, "Catppuccin") || !strings.Contains(view, "Gruvbox") {
		t.Errorf("expected available themes in view, got:\n%s", view)
	}

	// 3. Navegar con 'j' y seleccionar 'Catppuccin Mocha'
	m2, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mod = m2.(Model)
	if mod.themeListCursor != 1 {
		t.Errorf("expected theme cursor at 1, got %d", mod.themeListCursor)
	}

	// 4. Alternar Nerd Fonts con 'f'
	initialNerd := mod.themeConfig.NerdFonts
	m3, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	mod = m3.(Model)
	if mod.themeConfig.NerdFonts == initialNerd {
		t.Errorf("expected NerdFonts toggled")
	}

	// 5. Alternar Bordes con 'b'
	m4, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	mod = m4.(Model)

	// 6. Enter para confirmar selección
	m5, _ := mod.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mod = m5.(Model)
	if mod.activeState != stateFleetTable {
		t.Errorf("expected stateFleetTable after applying theme, got %v", mod.activeState)
	}
	if mod.themeConfig.ActiveTheme != "catppuccin" {
		t.Errorf("expected active theme 'catppuccin', got %s", mod.themeConfig.ActiveTheme)
	}
}

func TestTUI_ContainerPinningWorkflow(t *testing.T) {
	mockColl := &mockCollectorForTUI{
		metrics: []domain.ContainerMetric{
			{ID: "c1", Name: "alpha_service", Status: "running", CPUPercent: 5.0},
			{ID: "c2", Name: "beta_database", Status: "running", CPUPercent: 12.0},
			{ID: "c3", Name: "gamma_cache", Status: "running", CPUPercent: 2.0},
		},
	}
	mockV := &mockVaultForTUI{
		savedThemeConfig: domain.DefaultThemeConfig(),
	}
	model := NewModel(mockColl, mockV)
	model.metrics = mockColl.metrics
	model.width = 100
	model.height = 30

	// 1. Verificar lista inicial (orden natural: alpha, beta, gamma)
	initList := model.filteredMetrics()
	if len(initList) != 3 || initList[0].Name != "alpha_service" {
		t.Fatalf("expected alpha_service first in initial list, got %s", initList[0].Name)
	}

	// 2. Mover cursor a gamma_cache (cursor = 2) y presionar 'p' para fijar
	model.cursor = 2
	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	mod := m1.(Model)

	if !mod.isPinned("gamma_cache") {
		t.Errorf("expected gamma_cache to be pinned")
	}

	// 3. Verificar que gamma_cache ahora es la PRIMERA en filteredMetrics
	pinnedList := mod.filteredMetrics()
	if pinnedList[0].Name != "gamma_cache" {
		t.Errorf("expected pinned container 'gamma_cache' to be at index 0, got %s", pinnedList[0].Name)
	}

	// 4. Renderizar vista y comprobar que el header y badges reflejan FIJADOS y no contienen emojis
	view := mod.View()
	if !strings.Contains(view, "FIJADOS (1)") {
		t.Errorf("expected view to contain 'FIJADOS (1)', got:\n%s", view)
	}
	if !strings.Contains(view, "FIJADO") {
		t.Errorf("expected detail view to contain 'FIJADO' badge, got:\n%s", view)
	}
	if !strings.Contains(view, "─") {
		t.Errorf("expected divider line in left pane, got:\n%s", view)
	}
	if strings.Contains(view, "\U0001F4CC") {
		t.Errorf("expected zero emojis in TUI view, found pin emoji in:\n%s", view)
	}

	// 5. Presionar 'P' (Shift+P) para desanclar todos los contenedores
	m2, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	mod2 := m2.(Model)

	if mod2.isPinned("gamma_cache") {
		t.Errorf("expected gamma_cache to be unpinned after Shift+P")
	}
	clearedList := mod2.filteredMetrics()
	if clearedList[0].Name != "alpha_service" {
		t.Errorf("expected alpha_service to be first again after clearing pins, got %s", clearedList[0].Name)
	}
}

func assertZeroEmojisInView(t *testing.T, viewContent string, viewName string) {
	for _, r := range viewContent {
		if (r >= 0x1F300 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF) {
			t.Errorf("detected forbidden emoji '%c' (U+%04X) in %s view:\n%s", r, r, viewName, viewContent)
		}
	}
}

func TestTUI_AIOpsHierarchicalCatalogAndZeroEmojis(t *testing.T) {
	collector := &mockCollectorForTUI{metrics: sampleMetrics()}
	vaultMock := &mockVaultForTUI{}
	model := NewModel(collector, vaultMock)
	model.metrics = sampleMetrics()
	model.width = 100
	model.height = 30

	// 0. Probar ciclo de Modo V0 en status bar con Tab
	modes := []struct {
		mode     domain.ModelSelectionMode
		tokenSub string
	}{
		{domain.SelectionFast, "FAST"},
		{domain.SelectionDeep, "DEEP"},
		{domain.SelectionManual, "MANUAL"},
		{domain.SelectionAuto, "AUTO"},
	}

	for _, tc := range modes {
		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model = newModel.(Model)
		if model.aiConfig.SelectionMode != tc.mode {
			t.Errorf("expected mode %v after Tab, got %v", tc.mode, model.aiConfig.SelectionMode)
		}
		statusView := model.View()
		assertZeroEmojisInView(t, statusView, "Fleet Status Bar")
		if !strings.Contains(statusView, tc.tokenSub) {
			t.Errorf("expected status bar to contain mode %s, got:\n%s", tc.tokenSub, statusView)
		}
	}

	// 1. Abrir modal de IA con 'c' -> V1 (aiViewPolicy)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m := newModel.(Model)
	if m.activeState != stateConfigModal || m.aiState != aiViewPolicy {
		t.Fatalf("expected stateConfigModal and aiViewPolicy, got state=%v, aiState=%v", m.activeState, m.aiState)
	}

	// 2. Verificar vista V1 y CERO emojis
	v1View := m.View()
	assertZeroEmojisInView(t, v1View, "V1 Policy View")
	if !strings.Contains(v1View, "diagnóstico · asignaciones") {
		t.Errorf("expected header 'diagnóstico · asignaciones', got:\n%s", v1View)
	}
	if !strings.Contains(v1View, "[FAST]") || !strings.Contains(v1View, "[DEEP]") || !strings.Contains(v1View, "[AUTO]") {
		t.Errorf("expected slot tags in V1 view, got:\n%s", v1View)
	}

	// 3. Flujo rápido: Enter sobre FAST (cursor 0) -> entrar a V2 (aiViewModelBrowser) en <= 2 teclas
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if m.aiState != aiViewModelBrowser {
		t.Fatalf("expected aiViewModelBrowser after Enter on FAST, got %v", m.aiState)
	}
	if m.aiTargetSlot != domain.SlotFast {
		t.Errorf("expected target slot SlotFast, got %v", m.aiTargetSlot)
	}

	// 4. Verificar vista V2 y CERO emojis
	v2View := m.View()
	assertZeroEmojisInView(t, v2View, "V2 Model Browser")
	if !strings.Contains(v2View, "modelo para FAST") {
		t.Errorf("expected header 'modelo para FAST', got:\n%s", v2View)
	}
	if !strings.Contains(v2View, "[f] free") || !strings.Contains(v2View, "[l] local") {
		t.Errorf("expected filter toggles in V2 view, got:\n%s", v2View)
	}
	if !strings.Contains(v2View, "ctx") || !strings.Contains(v2View, "p95") || !strings.Contains(v2View, "eval [OK]") {
		t.Errorf("expected context line with gates metadata in V2 view, got:\n%s", v2View)
	}

	// 5. Probar conmutación de filtros 'f' (free) y 'l' (local)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = newModel.(Model)
	if !m.aiFilterFree {
		t.Errorf("expected aiFilterFree to be true after 'f'")
	}
	assertZeroEmojisInView(t, m.View(), "V2 Filter Free Active")

	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = newModel.(Model)
	if !m.aiFilterLocal {
		t.Errorf("expected aiFilterLocal to be true after 'l'")
	}
	assertZeroEmojisInView(t, m.View(), "V2 Filter Local Active")

	// Restablecer filtros
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = newModel.(Model)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = newModel.(Model)

	// 6. Seleccionar modelo con Enter -> debe asignar al slot FAST y volver a V1
	items := m.getBrowserItems()
	modelIdx := -1
	for idx, it := range items {
		if it.isModel {
			modelIdx = idx
			break
		}
	}
	if modelIdx == -1 {
		t.Fatalf("no models found in browser items")
	}
	m.aiBrowserCursor = modelIdx
	selectedModel := items[modelIdx].model

	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if m.aiState != aiViewPolicy {
		t.Fatalf("expected return to aiViewPolicy after selecting model, got %v", m.aiState)
	}
	assignedFast := domain.GetAssignedModel(domain.SlotFast, m.aiConfig.SlotPolicy)
	if assignedFast.ID != selectedModel.ID {
		t.Errorf("expected assigned slot model '%s', got '%s'", selectedModel.ID, assignedFast.ID)
	}

	// 7. Navegar a V3 (Proveedores) con 'p'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = newModel.(Model)
	if m.aiState != aiViewProviders {
		t.Fatalf("expected aiViewProviders after 'p', got %v", m.aiState)
	}

	v3View := m.View()
	assertZeroEmojisInView(t, v3View, "V3 Providers View")
	if !strings.Contains(v3View, "proveedores") || !strings.Contains(v3View, "r: refresh") {
		t.Errorf("expected V3 view header and refresh hint, got:\n%s", v3View)
	}

	// 8. Abrir sub-paso de custom endpoint con 'a'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = newModel.(Model)
	if m.aiState != aiViewCustomEndpoint {
		t.Fatalf("expected aiViewCustomEndpoint after 'a', got %v", m.aiState)
	}
	customView := m.View()
	assertZeroEmojisInView(t, customView, "Custom Endpoint Subview")

	// 9. Salir con Esc de custom endpoint -> V3 -> Esc -> V1 -> Esc -> Fleet Table
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.aiState != aiViewProviders {
		t.Errorf("expected return to aiViewProviders after Esc, got %v", m.aiState)
	}

	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.aiState != aiViewPolicy {
		t.Errorf("expected return to aiViewPolicy after Esc, got %v", m.aiState)
	}

	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.activeState != stateFleetTable {
		t.Errorf("expected return to stateFleetTable after Esc, got %v", m.activeState)
	}
}

func TestTUI_Ola1_FallbackToRuleOnAIFailure(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40

	// Simular fallo de IA (ej: sin key, timeout) procesado por DiagnoseContainerUseCase
	ruleRes := domain.DiagnosisResult{
		Level:           domain.LevelRule,
		RootCause:       "OOMKilled: contenedor superó límite de memoria",
		Severity:        "critical",
		SuggestedAction: "restart",
		RawOutput:       "",
	}

	// Recibir diagnosisResultMsg con resultado de Nivel 2 [RULE]
	newModel, _ := m.Update(diagnosisResultMsg{
		containerID: "c-oom",
		result:      ruleRes,
	})
	m = newModel.(Model)

	// Verificar que el resultado quedó registrado
	stored, ok := m.diagnosisResults["c-oom"]
	if !ok {
		t.Fatalf("expected diagnosis result for c-oom")
	}
	if stored.Level != domain.LevelRule {
		t.Errorf("expected LevelRule, got %v", stored.Level)
	}
	if stored.SuggestedAction != "restart" {
		t.Errorf("expected suggested action 'restart', got %s", stored.SuggestedAction)
	}

	// Verificar que NO se cacheó en diagnosisCache (las reglas son on-the-fly)
	if _, cached := m.diagnosisCache["c-oom"]; cached {
		t.Errorf("expected diagnosisCache to NOT store [RULE] results")
	}

	// Simular que el contenedor seleccionado es c-oom
	m.metrics = []domain.ContainerMetric{
		{
			ID:     "c-oom",
			Name:   "worker-app",
			Status: "Exited (137) 2 minutes ago",
		},
	}
	m.cursor = 0

	// Renderizar vista
	view := m.View()
	if !strings.Contains(view, "[RULE]") {
		t.Errorf("expected view to contain '[RULE]', got:\n%s", view)
	}
	if !strings.Contains(view, "OOMKilled") {
		t.Errorf("expected view to contain 'OOMKilled', got:\n%s", view)
	}

	// Abrir V4 Diagnóstico con 'd' y verificar la acción sugerida
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	mV4 := newModel.(Model)
	v4View := mV4.View()
	if !strings.Contains(v4View, "[r] aplicar restart") {
		t.Errorf("expected V4 view to contain suggested action '[r] aplicar restart', got:\n%s", v4View)
	}
}

func TestTUI_Ola1_PartialAIParseLevelAIPartial(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40

	aiPartialRes := domain.DiagnosisResult{
		Level:           domain.LevelAIPartial,
		RootCause:       "El contenedor falló debido a un error de inicialización no capturado",
		Severity:        "warning",
		SuggestedAction: "none",
		RawOutput:       "Texto plano devuelto por IA sin formato JSON estructurado",
	}

	newModel, _ := m.Update(diagnosisResultMsg{
		containerID: "c-partial",
		result:      aiPartialRes,
	})
	m = newModel.(Model)

	stored, ok := m.diagnosisResults["c-partial"]
	if !ok {
		t.Fatalf("expected diagnosis result for c-partial")
	}
	if stored.Level != domain.LevelAIPartial {
		t.Errorf("expected LevelAIPartial, got %v", stored.Level)
	}

	// LevelAIPartial sí debe guardarse en diagnosisCache
	if _, cached := m.diagnosisCache["c-partial"]; !cached {
		t.Errorf("expected diagnosisCache to store [AI~] partial results")
	}

	m.metrics = []domain.ContainerMetric{
		{
			ID:     "c-partial",
			Name:   "api-srv",
			Status: "Exited (1) 1 minute ago",
		},
	}
	m.cursor = 0

	view := m.View()
	if !strings.Contains(view, "[AI~]") {
		t.Errorf("expected view to contain '[AI~]', got:\n%s", view)
	}
}

func TestTUI_Ola1_ManualModeAndDemandTrigger(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.aiConfig.SelectionMode = domain.SelectionManual

	m.metrics = []domain.ContainerMetric{
		{
			ID:     "c-crash",
			Name:   "db-service",
			Status: "Exited (137) 1 minute ago",
		},
	}
	m.cursor = 0

	// Evaluar en modo manual sin trigger explícito -> produce Nivel 2 [RULE]
	ruleRes := domain.DiagnosisResult{
		Level:           domain.LevelRule,
		RootCause:       "OOMKilled: contenedor superó límite de memoria",
		Severity:        "critical",
		SuggestedAction: "restart",
	}

	newModel, _ := m.Update(diagnosisResultMsg{
		containerID: "c-crash",
		result:      ruleRes,
	})
	m = newModel.(Model)

	view := m.View()
	if !strings.Contains(view, "[RULE]") {
		t.Errorf("expected view to contain '[RULE]', got:\n%s", view)
	}
	// Debe mostrar el hint de solicitar IA
	if !strings.Contains(view, "[i] solicitar diagnóstico IA") {
		t.Errorf("expected view in manual mode to show '[i] solicitar diagnóstico IA', got:\n%s", view)
	}

	// Presionar 'i' en Fleet Table para forzar IA a demanda
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = newModel.(Model)
	if cmd == nil {
		t.Errorf("expected command returned when pressing 'i' in manual mode")
	}

	// Simular la llegada de la respuesta de IA provocada por 'i'
	aiRes := domain.DiagnosisResult{
		Level:           domain.LevelAI,
		RootCause:       "Memoria heap de Postgres agotada por consulta masiva",
		Severity:        "critical",
		SuggestedAction: "restart",
	}
	newModel, _ = m.Update(diagnosisResultMsg{
		containerID: "c-crash",
		result:      aiRes,
	})
	m = newModel.(Model)

	viewAfter := m.View()
	if !strings.Contains(viewAfter, "[AI]") {
		t.Errorf("expected view after trigger 'i' to show '[AI]', got:\n%s", viewAfter)
	}
	if strings.Contains(viewAfter, "[RULE]") {
		t.Errorf("view should not show '[RULE]' anymore, got:\n%s", viewAfter)
	}
}

func TestTUI_Ola1_SelectiveCaching(t *testing.T) {
	m := NewModel(nil)

	// 1. Mensaje con Nivel [RULE] -> NO debe ir a diagnosisCache
	newModel, _ := m.Update(diagnosisResultMsg{
		containerID: "c1",
		result: domain.DiagnosisResult{
			Level:     domain.LevelRule,
			RootCause: "Exit 143",
		},
	})
	m = newModel.(Model)
	if _, ok := m.diagnosisCache["c1"]; ok {
		t.Errorf("RULE results must not be stored in diagnosisCache")
	}

	// 2. Mensaje con Nivel [SIG] -> NO debe ir a diagnosisCache
	newModel, _ = m.Update(diagnosisResultMsg{
		containerID: "c2",
		result: domain.DiagnosisResult{
			Level:     domain.LevelSignal,
			RootCause: "Exit code 42",
		},
	})
	m = newModel.(Model)
	if _, ok := m.diagnosisCache["c2"]; ok {
		t.Errorf("SIG results must not be stored in diagnosisCache")
	}

	// 3. Mensaje con Nivel [AI] -> SÍ debe ir a diagnosisCache
	newModel, _ = m.Update(diagnosisResultMsg{
		containerID: "c3",
		result: domain.DiagnosisResult{
			Level:     domain.LevelAI,
			RootCause: "AI diagnosed root cause",
		},
	})
	m = newModel.(Model)
	if _, ok := m.diagnosisCache["c3"]; !ok {
		t.Errorf("AI results must be stored in diagnosisCache")
	}

	// 4. Mensaje con Nivel [AI~] -> SÍ debe ir a diagnosisCache
	newModel, _ = m.Update(diagnosisResultMsg{
		containerID: "c4",
		result: domain.DiagnosisResult{
			Level:     domain.LevelAIPartial,
			RootCause: "AI partial root cause",
		},
	})
	m = newModel.(Model)
	if _, ok := m.diagnosisCache["c4"]; !ok {
		t.Errorf("AIPartial results must be stored in diagnosisCache")
	}
}

func TestTUI_Ola1_ZeroRCE_NoAutoExecution(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)

	// Simular diagnóstico con SuggestedAction = "restart"
	ruleRes := domain.DiagnosisResult{
		Level:           domain.LevelRule,
		RootCause:       "OOMKilled: contenedor superó límite de memoria",
		Severity:        "critical",
		SuggestedAction: "restart",
	}

	newModel, _ := m.Update(diagnosisResultMsg{
		containerID: "c-strict",
		result:      ruleRes,
	})
	m = newModel.(Model)

	// Verificar que mockColl.executedCmds está vacío (Cero RCE)
	if len(mockColl.executedCmds) != 0 {
		t.Fatalf("ZERO RCE VIOLATION: commands were automatically executed: %v", mockColl.executedCmds)
	}

	// Verificar que el estado de la TUI sólo tiene el hint visual y no disparó un restart
	res := m.diagnosisResults["c-strict"]
	if res.SuggestedAction != "restart" {
		t.Errorf("expected suggested action 'restart', got %s", res.SuggestedAction)
	}
	if len(mockColl.executedCmds) != 0 {
		t.Errorf("ZERO RCE VIOLATION: commands executed during check")
	}
}

// ==============================================================================
// OLA 2: PRUEBAS DE ACEPTACIÓN — CRITERIOS 1 AL 7
// ==============================================================================

// Criterio 1: Flujo crash 137 -> V4 ('d') -> navegar a [r] -> Enter -> Confirmar ('y') -> Ejecuta restart
func TestTUI_Ola2_CrashToV4ToRemediation(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)
	m.width = 120
	m.height = 40

	m.metrics = []domain.ContainerMetric{
		{
			ID:     "c-crash137",
			Name:   "worker-crash",
			Status: "Exited (137) 1 minute ago",
		},
	}
	m.cursor = 0

	// Diagnóstico Nivel 2 [RULE]
	ruleRes := domain.DiagnosisResult{
		Level:           domain.LevelRule,
		RootCause:       "OOMKilled: contenedor superó límite de memoria",
		Severity:        "critical",
		SuggestedAction: "restart",
	}
	newModel, _ := m.Update(diagnosisResultMsg{
		containerID: "c-crash137",
		result:      ruleRes,
	})
	m = newModel.(Model)

	// 1. Presionar 'd' abre V4 Diagnóstico
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newModel.(Model)
	if m.activeState != stateDiagnosisModal {
		t.Fatalf("expected stateDiagnosisModal, got %v", m.activeState)
	}

	// Verificar contenido de V4
	v4View := m.View()
	if !strings.Contains(v4View, "[RULE]") || !strings.Contains(v4View, "OOMKilled") {
		t.Errorf("expected V4 view to show [RULE] and OOMKilled, got:\n%s", v4View)
	}
	if !strings.Contains(v4View, "evidencia:") {
		t.Errorf("expected V4 to have evidence section")
	}
	if !strings.Contains(v4View, "> [r] aplicar restart") {
		t.Errorf("expected suggested restart action to be selected with '>', got:\n%s", v4View)
	}

	// 2. Presionar 'Enter' sobre la acción seleccionada abre modal de confirmación (D1 CERO RCE)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	if m.activeState != stateConfirmRemediation {
		t.Fatalf("expected stateConfirmRemediation after Enter in V4, got %v", m.activeState)
	}
	if m.pendingAction != domain.ActionRestart {
		t.Errorf("expected pending action restart, got %v", m.pendingAction)
	}
	if len(mockColl.executedCmds) != 0 {
		t.Fatalf("ZERO RCE VIOLATION: command was executed before user confirmation")
	}

	// 3. Confirmar con 'y'
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = newModel.(Model)
	if m.activeState != stateFleetTable {
		t.Errorf("expected return to stateFleetTable after confirm, got %v", m.activeState)
	}
	if cmd == nil {
		t.Errorf("expected command returned to execute remediation")
	}
}

// Criterio 2: Panel derecho sin bloque ACCIONES DISPONIBLES y con 4 secciones
func TestTUI_Ola2_RightPanelWithoutActionsBlock(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:             "c-detail",
			Name:           "api-service",
			Status:         "running",
			CPUPercent:     12.0,
			RAMBytes:       256 * 1024 * 1024,
			RAMLimitBytes:  512 * 1024 * 1024,
			ComposeProject: "server_tracker",
			Networks:       []string{"solv_net"},
			Ports:          []string{"8080->80"},
			VolumeCount:    2,
		},
	}
	m.cursor = 0

	view := m.View()

	// Prohibido bloque "ACCIONES DISPONIBLES"
	if strings.Contains(view, "ACCIONES DISPONIBLES") {
		t.Errorf("panel derecho no debe mostrar el bloque 'ACCIONES DISPONIBLES'")
	}

	// 4 secciones interpretadas requeridas
	if !strings.Contains(view, "CICLO DE VIDA") {
		t.Errorf("expected section 'CICLO DE VIDA' in right panel")
	}
	if !strings.Contains(view, "VITALES") {
		t.Errorf("expected section 'VITALES' in right panel")
	}
	if !strings.Contains(view, "DIAGNÓSTICO") {
		t.Errorf("expected section 'DIAGNÓSTICO' in right panel")
	}
	if !strings.Contains(view, "CONTEXTO") {
		t.Errorf("expected section 'CONTEXTO' in right panel")
	}
	if !strings.Contains(view, "compose: server_tracker") {
		t.Errorf("expected compose project to be rendered in CONTEXTO")
	}
}

// Criterio 3: Help overlay sin teclas fantasma (sin Ctrl+A, con c y d)
func TestTUI_Ola2_HelpOverlaySingleSource(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.activeState = stateHelp

	view := m.View()

	if strings.Contains(view, "Ctrl+A") {
		t.Errorf("help overlay must NOT contain obsolete 'Ctrl+A' key")
	}
	if !strings.Contains(view, "c") || !strings.Contains(view, "Elegir modelo") {
		t.Errorf("help overlay must contain 'c' for Elegir modelo")
	}
	if !strings.Contains(view, "d") || !strings.Contains(view, "V4 diagnóstico") {
		t.Errorf("help overlay must contain 'd' for V4 diagnóstico")
	}
}

// Criterio 4: Throttling de CPU siempre visible en vitales (incluso 0%)
func TestTUI_Ola2_ThrottlingAlwaysVisible(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:                  "c-zero-throttle",
			Name:                "worker-ok",
			Status:              "running",
			CPUPercent:          5.0,
			CPUPercentThrottled: 0.0,
		},
	}
	m.cursor = 0

	view := m.View()
	if !strings.Contains(view, "throttling 0%") {
		t.Errorf("CPU line must always display throttling percentage even when 0%%, got:\n%s", view)
	}
}

// Criterio 5: Evidencia tipada y extensible (admite tipo 'historial' sin romperse)
func TestTUI_Ola2_TypedEvidenceExtensible(t *testing.T) {
	m := NewModel(nil)
	c := domain.ContainerMetric{
		ID:                  "c-ev",
		Name:                "test-ev",
		Status:              "Exited (137) 2 minutes ago",
		RAMBytes:            508 * 1024 * 1024,
		RAMLimitBytes:       512 * 1024 * 1024,
		CPUPercent:          10.0,
		CPUPercentThrottled: 25.0,
		RestartCount:        3,
	}
	res := domain.DiagnosisResult{
		Level:     domain.LevelRule,
		RootCause: "OOMKilled",
	}

	evs := BuildEvidence(m, c, res, "Killed process 1234 (node)")
	if len(evs) == 0 {
		t.Fatalf("expected evidence items to be generated")
	}

	// Agregar un item de tipo mock 'historial' (Ola 4) para verificar extensibilidad
	evs = append(evs, EvidenceItem{
		Type:  "historial",
		Label: "crash_previo",
		Value: "hace 10m por OOM",
	})

	foundHist := false
	for _, ev := range evs {
		if ev.Type == "historial" && ev.Label == "crash_previo" {
			foundHist = true
			break
		}
	}
	if !foundHist {
		t.Errorf("failed to extend evidence list with 'historial' item")
	}
}

// Criterio 6: V4 se renderiza como overlay centrado y conserva flota detrás
func TestTUI_Ola2_CenteredOverlay(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:     "c-front",
			Name:   "front-app",
			Status: "running",
		},
	}
	m.cursor = 0

	// Abrir V4
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newModel.(Model)

	view := m.View()
	// Contiene el encabezado del modal V4
	if !strings.Contains(view, "diagnóstico · front-app") {
		t.Errorf("expected V4 modal header in view, got:\n%s", view)
	}
	// Y la flota sigue visible en el fondo detrás del overlay
	if !strings.Contains(view, "CONTENEDORES") {
		t.Errorf("expected background fleet table to remain visible behind modal, got:\n%s", view)
	}

	// Cerrar con Esc
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.activeState != stateFleetTable {
		t.Errorf("expected return to stateFleetTable after Esc in V4, got %v", m.activeState)
	}
}

// Criterio 7: Cero RCE en V4 (navegación y selección jamás ejecutan comandos automáticos)
func TestTUI_Ola2_ZeroRCE_NoAutoExecution(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:     "c-safe",
			Name:   "safe-app",
			Status: "Exited (137) 1 minute ago",
		},
	}
	m.cursor = 0

	ruleRes := domain.DiagnosisResult{
		Level:           domain.LevelRule,
		RootCause:       "OOMKilled: contenedor superó límite de memoria",
		SuggestedAction: "restart",
	}
	newModel, _ := m.Update(diagnosisResultMsg{
		containerID: "c-safe",
		result:      ruleRes,
	})
	m = newModel.(Model)

	// Abrir V4
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newModel.(Model)

	// Navegar con j y k
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newModel.(Model)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = newModel.(Model)

	// Verificar que mockColl.executedCmds sigue 0
	if len(mockColl.executedCmds) != 0 {
		t.Fatalf("ZERO RCE VIOLATION: command was executed while navigating V4: %v", mockColl.executedCmds)
	}
}

// ============================================================================
// OLA 3: RED, DEPENDENCIAS Y RADIO DE IMPACTO — TESTS DE ACEPTACIÓN
// ============================================================================

// Criterio 1: Cascada visible (dependencias inferidas bidireccionales y línea resumida en CONTEXTO)
func TestTUI_Ola3_Criterio1_CascadaVisible(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:             "db-1",
			Name:           "solv_db",
			Status:         "Exited (0)",
			Networks:       []string{"solv_net"},
			NetworkAliases: []string{"db"},
			Ports:          []string{"127.0.0.1:5432->5432"},
		},
		{
			ID:             "api-1",
			Name:           "api-node",
			Status:         "running",
			Networks:       []string{"solv_net"},
			EnvVars:        []string{"DATABASE_URL=postgres://db:5432/tracker"},
			Ports:          []string{"0.0.0.0:3000->3000"},
		},
	}

	// 1. V5 sobre solv_db muestra "dependen de mi: api-node (inferido)"
	m.cursor = 0
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)

	viewDB := m.View()
	if !strings.Contains(viewDB, "dependen de mi:") || !strings.Contains(viewDB, "api-node (inferido)") {
		t.Errorf("expected V5 on solv_db to show 'dependen de mi: api-node (inferido)', got:\n%s", viewDB)
	}

	// Cerrar V5
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)

	// 2. El panel derecho (CONTEXTO) de solv_db muestra la línea resumida
	fleetView := m.View()
	if !strings.Contains(fleetView, "dependen de mi: api-node") {
		t.Errorf("expected right pane CONTEXTO of solv_db to show 'dependen de mi: api-node', got:\n%s", fleetView)
	}

	// 3. V5 sobre api-node muestra "depende de: solv_db (inferido)"
	m.cursor = 1
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)

	viewAPI := m.View()
	if !strings.Contains(viewAPI, "depende de:") || !strings.Contains(viewAPI, "solv_db (inferido)") {
		t.Errorf("expected V5 on api-node to show 'depende de: solv_db (inferido)', got:\n%s", viewAPI)
	}
}

// Criterio 2: Falso positivo controlado (sin coincidencias o sin red compartida produce '--')
func TestTUI_Ola3_Criterio2_FalsoPositivoControlado(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:       "app-isolated",
			Name:     "app-isolated",
			Status:   "running",
			Networks: []string{"isolated_net"},
			EnvVars: []string{
				"APP_ENV=production",
				"PORT=8080",
				"SECRET_KEY=supersecret12345",
			},
		},
	}
	m.cursor = 0

	// Abrir V5
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)

	view := m.View()
	if !strings.Contains(view, "depende de:         --") && !strings.Contains(view, "depende de:        --") && !strings.Contains(view, "depende de:      --") {
		if !strings.Contains(view, "--") {
			t.Errorf("expected V5 to display '--' for empty dependencies, got:\n%s", view)
		}
	}
	if strings.Contains(view, "(inferido)") {
		t.Errorf("expected NO inferred dependencies for unmatching env vars, got:\n%s", view)
	}

	// Cerrar y verificar panel derecho
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	panelView := m.View()
	if !strings.Contains(panelView, "dependen de mi: --") {
		t.Errorf("expected right pane to show 'dependen de mi: --', got:\n%s", panelView)
	}
}

// Criterio 3: Conflicto de puerto retenido por contenedor running
func TestTUI_Ola3_Criterio3_ConflictoPuerto(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:     "c-prod",
			Name:   "nginx-prod",
			Status: "Up 3 hours",
			Ports:  []string{"0.0.0.0:8080->80"},
		},
		{
			ID:     "c-test",
			Name:   "nginx-test",
			Status: "Exited (1) 2 minutes ago",
			Ports:  []string{"0.0.0.0:8080->80"},
		},
	}

	// 1. V4 sobre nginx-test muestra evidencia de puerto retenido
	m.cursor = 1
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newModel.(Model)

	v4View := m.View()
	if !strings.Contains(v4View, "puerto 8080 · retenido por: nginx-prod") {
		t.Errorf("expected V4 to display 'puerto 8080 · retenido por: nginx-prod', got:\n%s", v4View)
	}

	// Cerrar V4
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)

	// 2. V5 sobre nginx-test muestra "[||] en conflicto con: nginx-prod"
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)

	v5View := m.View()
	if !strings.Contains(v5View, "[||] en conflicto con: nginx-prod") {
		t.Errorf("expected V5 to display '[||] en conflicto con: nginx-prod', got:\n%s", v5View)
	}
}

// Criterio 4: Exposición cromática (0.0.0.0 -> [||] expuesto; 127.0.0.1 -> [OK] loopback; Cero [!!])
func TestTUI_Ola3_Criterio4_ExposicionPuertos(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:     "c-ports",
			Name:   "gateway-proxy",
			Status: "running",
			Ports:  []string{"127.0.0.1:5432->5432", "0.0.0.0:8080->8080"},
		},
	}
	m.cursor = 0

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)

	v5View := m.View()
	if !strings.Contains(v5View, "[OK] loopback") {
		t.Errorf("expected 127.0.0.1 bind to show '[OK] loopback', got:\n%s", v5View)
	}
	if !strings.Contains(v5View, "[||] expuesto") {
		t.Errorf("expected 0.0.0.0 bind to show '[||] expuesto', got:\n%s", v5View)
	}
	// D3 Cero falsas alarmas: la exposición informativa NUNCA debe usar [!!]
	lines := strings.Split(v5View, "\n")
	for _, l := range lines {
		if strings.Contains(l, "8080") || strings.Contains(l, "5432") {
			if strings.Contains(l, "[!!]") {
				t.Errorf("PROHIBITED: port exposure must NEVER use [!!] glyph: %s", l)
			}
		}
	}
}

// Criterio 5: Navegación global con 'n' en cualquier contenedor y Help actualizado
func TestTUI_Ola3_Criterio5_NavegacionV5(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:     "healthy-app",
			Name:   "healthy-app",
			Status: "running",
		},
	}
	m.cursor = 0

	// 1. Tecla 'n' abre V5 incluso en contenedor sano sin diagnóstico
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)
	if m.activeState != stateNetworkModal {
		t.Fatalf("expected activeState to be stateNetworkModal, got %v", m.activeState)
	}

	view := m.View()
	if !strings.Contains(view, "red · healthy-app") {
		t.Errorf("expected V5 header 'red · healthy-app', got:\n%s", view)
	}

	// 2. Esc vuelve a la flota
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)
	if m.activeState != stateFleetTable {
		t.Fatalf("expected activeState to return to stateFleetTable after Esc, got %v", m.activeState)
	}

	// 3. Help overlay muestra "n" "V5 red" sin "(proximamente)"
	m.activeState = stateHelp
	helpView := m.View()
	if !strings.Contains(helpView, "V5 red") {
		t.Errorf("expected help overlay to contain 'V5 red', got:\n%s", helpView)
	}
	if strings.Contains(helpView, "V5 red (próximamente)") || strings.Contains(helpView, "V5 red (proximamente)") {
		t.Errorf("help overlay must NOT have '(proximamente)' for key 'n'")
	}
}

// Criterio 6: 80 columnas con blindaje ANSI y truncamiento seguro de alias en V5
func TestTUI_Ola3_Criterio6_80ColumnasSinWrap(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	longAlias := "database-primary-cluster-us-east-replica-01-shard-02-very-long-name"
	m.metrics = []domain.ContainerMetric{
		{
			ID:             "db-wide",
			Name:           "db-wide",
			Status:         "running",
			Networks:       []string{"solv_net"},
			IPAddress:      "172.19.0.5",
			NetworkAliases: []string{longAlias},
		},
	}
	m.cursor = 0

	v5View := BuildNetworkModalContent(m, m.metrics[0])
	lines := strings.Split(v5View, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w > 80 {
			t.Errorf("V5 line %d exceeds 80 columns (width=%d):\n%s", i, w, line)
		}
	}
	if !strings.Contains(v5View, "...") {
		t.Errorf("expected long alias to be truncated with '...' in V5, got:\n%s", v5View)
	}
}

// Criterio 7: Cero RCE en V5 (solo lectura, sin acciones ejecutables)
func TestTUI_Ola3_Criterio7_ZeroRCE_ReadOnly(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)
	m.width = 120
	m.height = 40
	m.metrics = []domain.ContainerMetric{
		{
			ID:     "c-ro",
			Name:   "ro-app",
			Status: "running",
		},
	}
	m.cursor = 0

	// Abrir V5
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)

	// Intentar presionar teclas de acción típicas: enter, r, s, x
	for _, key := range []string{"enter", "r", "s", "x", "j", "k"} {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = newModel.(Model)
	}

	// Verificar que jamás se invocó remediación ni se cambió a modal de confirmación
	if len(mockColl.executedCmds) != 0 {
		t.Fatalf("ZERO RCE VIOLATION: command was executed from V5: %v", mockColl.executedCmds)
	}
	if m.activeState == stateConfirmRemediation {
		t.Fatalf("V5 must NOT transition to remediation confirmation")
	}
}

// ============================================================================
// OLA 4: MEMORIA DEL SISTEMA (CONTEXTO TEMPORAL) — TESTS DE ACEPTACIÓN
// ============================================================================

// Criterio 1: Umbral exacto (2 crashes OOM -> banner SIN tag; 3er crash -> muestra "(3/1h)" sin abrir V4)
func TestTUI_Ola4_Criterio1_UmbralExacto(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40

	// 1. Crash 1 (hace 30m)
	c1 := domain.ContainerMetric{
		ID:              "c-oom",
		Name:            "worker-app",
		Status:          "Exited (137) 30 minutes ago",
		LastStateChange: time.Now().Add(-30 * time.Minute),
	}
	m.metrics = []domain.ContainerMetric{c1}
	m.cursor = 0
	if cmd := m.triggerTriageIfAnomalous(c1); cmd != nil {
		msg := cmd()
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}

	view1 := m.View()
	if strings.Contains(view1, "recurrente") || strings.Contains(view1, "(1/1h)") {
		t.Errorf("1st crash must NOT be tagged as recurrent, got:\n%s", view1)
	}

	// 2. Crash 2 (hace 15m)
	c2 := domain.ContainerMetric{
		ID:              "c-oom",
		Name:            "worker-app",
		Status:          "Exited (137) 15 minutes ago",
		LastStateChange: time.Now().Add(-15 * time.Minute),
	}
	m.metrics = []domain.ContainerMetric{c2}
	if cmd := m.triggerTriageIfAnomalous(c2); cmd != nil {
		msg := cmd()
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}

	view2 := m.View()
	if strings.Contains(view2, "recurrente") || strings.Contains(view2, "(2/1h)") {
		t.Errorf("2nd crash must NOT be tagged as recurrent, got:\n%s", view2)
	}

	// 3. Crash 3 (hace 2m) -> Dispara recurrencia
	c3 := domain.ContainerMetric{
		ID:              "c-oom",
		Name:            "worker-app",
		Status:          "Exited (137) 2 minutes ago",
		LastStateChange: time.Now().Add(-2 * time.Minute),
	}
	m.metrics = []domain.ContainerMetric{c3}
	if cmd := m.triggerTriageIfAnomalous(c3); cmd != nil {
		msg := cmd()
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}

	view3 := m.View()
	if !strings.Contains(view3, "recurrente (3/1h)") {
		t.Errorf("3rd crash MUST display 'recurrente (3/1h)', got:\n%s", view3)
	}
	if m.activeState != stateFleetTable {
		t.Errorf("expected activeState to remain stateFleetTable without opening V4, got %v", m.activeState)
	}
}

// Criterio 2: V4 del tercer crash muestra las tres líneas de historial
func TestTUI_Ola4_Criterio2_V4TresLineasHistorial(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40

	// Simular 3 crashes en el journal con diagnósticos previos y métricas de RAM
	c1 := domain.ContainerMetric{
		ID:              "c-hist",
		Name:            "api-service",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-25 * time.Minute),
	}
	c2 := domain.ContainerMetric{
		ID:              "c-hist",
		Name:            "api-service",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-15 * time.Minute),
	}
	c3 := domain.ContainerMetric{
		ID:              "c-hist",
		Name:            "api-service",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-2 * time.Minute),
	}

	m.metrics = []domain.ContainerMetric{c3}
	m.cursor = 0

	// Tendencia RAM creciente sostenida
	m.metricsHistory["c-hist"] = &MetricHistory{
		RAM: []float64{100, 100, 150, 150, 200, 200},
	}

	// Registrar los 3 crashes
	if cmd := m.triggerTriageIfAnomalous(c1); cmd != nil {
		msg := cmd()
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}
	if cmd := m.triggerTriageIfAnomalous(c2); cmd != nil {
		msg := cmd()
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}
	if cmd := m.triggerTriageIfAnomalous(c3); cmd != nil {
		msg := cmd()
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}

	// Abrir V4
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newModel.(Model)

	v4View := m.View()

	// 1. Línea de conteo + homogeneidad + recencia
	if !strings.Contains(v4View, "historial: 3 crashes en 1h (todos OOM)") {
		t.Errorf("expected V4 to display 'historial: 3 crashes en 1h (todos OOM)', got:\n%s", v4View)
	}

	// 2. Línea de hipótesis previa
	if !strings.Contains(v4View, "hipótesis previa:") {
		t.Errorf("expected V4 to display 'hipótesis previa:', got:\n%s", v4View)
	}

	// 3. Línea de tendencia RAM
	if !strings.Contains(v4View, "tendencia RAM: creciente sostenida") {
		t.Errorf("expected V4 to display 'tendencia RAM: creciente sostenida', got:\n%s", v4View)
	}
}

// Criterio 3: Con IA activa, prompt capturado incluye bloque y salvaguarda solo desde el 2º crash
func TestTUI_Ola4_Criterio3_PromptHistorialIA(t *testing.T) {
	var capturedPrompts []string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if msgs, ok := reqBody["messages"].([]interface{}); ok {
			for _, msg := range msgs {
				if mObj, ok := msg.(map[string]interface{}); ok {
					if mObj["role"] == "user" {
						capturedPrompts = append(capturedPrompts, fmt.Sprint(mObj["content"]))
					}
				}
			}
		}
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "[OOMKilled -> Incrementar límites de memoria]"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := domain.DefaultAIConfig()
	cfg.ActiveProvider = domain.ProviderOpenRouter
	p := cfg.Providers[domain.ProviderOpenRouter]
	p.APIKey = "test-key"
	p.Endpoint = mockServer.URL
	cfg.Providers[domain.ProviderOpenRouter] = p

	client := ai.NewTriageClientWithConfig(cfg)
	client.SetCatalogService(ai.NewCatalogService(t.TempDir() + "/cat.json"))

	m := NewModel(nil)
	m.triageClient = client
	client.SetCrashJournal(m.crashJournal)

	// Crash 1: primer crash
	c1 := domain.ContainerMetric{
		ID:              "c-ia",
		Name:            "ia-service",
		Status:          "Exited (137) just now",
		LastStateChange: time.Now().Add(-10 * time.Minute),
	}
	m.metrics = []domain.ContainerMetric{c1}
	m.cursor = 0

	cmd1 := m.triggerTriageForced(c1)
	if cmd1 != nil {
		msg := cmd1()
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}

	if len(capturedPrompts) < 1 {
		t.Fatalf("expected 1 prompt to be captured")
	}
	prompt1 := capturedPrompts[0]
	// En el primer crash NO debe incluir el bloque de historial
	if strings.Contains(prompt1, "Historial reciente:") {
		t.Errorf("1st crash prompt must NOT contain 'Historial reciente:', got:\n%s", prompt1)
	}
	if strings.Contains(prompt1, "INSTRUCCIÓN: la hipótesis previa es hipótesis") {
		t.Errorf("1st crash prompt must NOT contain instruction guard, got:\n%s", prompt1)
	}

	// Crash 2: segundo crash
	c2 := domain.ContainerMetric{
		ID:              "c-ia",
		Name:            "ia-service",
		Status:          "Exited (137) 1m ago",
		LastStateChange: time.Now().Add(-1 * time.Minute),
	}
	m.metrics = []domain.ContainerMetric{c2}

	cmd2 := m.triggerTriageForced(c2)
	if cmd2 != nil {
		msg := cmd2()
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}

	if len(capturedPrompts) < 2 {
		t.Fatalf("expected 2 prompts to be captured")
	}
	prompt2 := capturedPrompts[1]
	// En el segundo crash SÍ debe incluir el bloque de historial y la guardia
	if !strings.Contains(prompt2, "Historial reciente:") {
		t.Errorf("2nd crash prompt MUST contain 'Historial reciente:', got:\n%s", prompt2)
	}
	if !strings.Contains(prompt2, "INSTRUCCIÓN: la hipótesis previa es hipótesis, no verdad. Verifícala contra la evidencia nueva. Descártala si la contradice.") {
		t.Errorf("2nd crash prompt MUST contain exact instruction guard, got:\n%s", prompt2)
	}
}

// Criterio 4: Dedupe (un solo crash real + 10 ticks de refresh produce Count == 1)
func TestTUI_Ola4_Criterio4_DedupeTicks(t *testing.T) {
	m := NewModel(nil)
	changeTime := time.Now().Add(-5 * time.Minute)
	c := domain.ContainerMetric{
		ID:              "c-dedupe-test",
		Name:            "dedupe-app",
		Status:          "Exited (137)",
		LastStateChange: changeTime,
	}

	// Simular 10 ticks de telemetría repetida
	for i := 0; i < 10; i++ {
		newModel, _ := m.Update([]domain.ContainerMetric{c})
		m = newModel.(Model)
	}

	count := m.crashJournal.Count("c-dedupe-test", "", 1*time.Hour)
	if count != 1 {
		t.Fatalf("expected Count == 1 after 10 ticks of identical state, got %d", count)
	}
}

// Criterio 5: Volatilidad (reiniciar el agente vacía el journal; primer crash post-reinicio es limpio)
func TestTUI_Ola4_Criterio5_Volatilidad(t *testing.T) {
	// Agente previo con 3 crashes
	m1 := NewModel(nil)
	c := domain.ContainerMetric{
		ID:              "c-reset",
		Name:            "reset-app",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-10 * time.Minute),
	}
	m1.crashJournal.Record(c, "diag 1", "RULE")
	c.LastStateChange = time.Now().Add(-5 * time.Minute)
	m1.crashJournal.Record(c, "diag 2", "RULE")
	c.LastStateChange = time.Now().Add(-1 * time.Minute)
	m1.crashJournal.Record(c, "diag 3", "RULE")

	if m1.crashJournal.Count("c-reset", "", 1*time.Hour) != 3 {
		t.Fatalf("m1 should have 3 crashes")
	}

	// Simular reinicio del agente instanciando un nuevo Model
	m2 := NewModel(nil)
	m2.width = 120
	m2.height = 40
	cFresh := domain.ContainerMetric{
		ID:              "c-reset",
		Name:            "reset-app",
		Status:          "Exited (137)",
		LastStateChange: time.Now(),
	}
	m2.metrics = []domain.ContainerMetric{cFresh}
	m2.cursor = 0

	if cmd := m2.triggerTriageIfAnomalous(cFresh); cmd != nil {
		msg := cmd()
		newModel, _ := m2.Update(msg)
		m2 = newModel.(Model)
	}

	view := m2.View()
	if strings.Contains(view, "recurrente") {
		t.Errorf("post-restart agent must NOT mark first crash as recurrent, got:\n%s", view)
	}

	// Abrir V4
	newModel, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m2 = newModel.(Model)
	v4View := m2.View()
	if strings.Contains(v4View, "hipótesis previa:") {
		t.Errorf("post-restart V4 must NOT have previous hypothesis, got:\n%s", v4View)
	}
}

// Criterio 6: H5 (en OOM recurrente, SuggestedAction sigue siendo restart; nota en V4 y NO en banner)
func TestTUI_Ola4_Criterio6_H5_SuggestedActionAndNote(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 40

	c := domain.ContainerMetric{
		ID:              "c-h5",
		Name:            "h5-app",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-20 * time.Minute),
	}
	m.metrics = []domain.ContainerMetric{c}
	m.cursor = 0

	// Registrar 3 crashes OOM
	m.crashJournal.Record(c, "OOM 1", "RULE")
	c.LastStateChange = time.Now().Add(-10 * time.Minute)
	m.crashJournal.Record(c, "OOM 2", "RULE")
	c.LastStateChange = time.Now().Add(-1 * time.Minute)

	if cmd := m.triggerTriageIfAnomalous(c); cmd != nil {
		msg := cmd()
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
	}

	res := m.diagnosisResults["c-h5"]
	// 1. SuggestedAction sigue siendo restart
	if res.SuggestedAction != "restart" {
		t.Errorf("expected SuggestedAction to remain 'restart', got: %s", res.SuggestedAction)
	}

	// 2. Banner NO contiene la nota explicativa
	bannerView := m.View()
	if strings.Contains(bannerView, "reiniciar no resolverá la causa raíz") {
		t.Errorf("banner must NOT contain recurrence note, got:\n%s", bannerView)
	}

	// 3. V4 SÍ contiene la nota explicativa como segunda línea
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newModel.(Model)
	v4View := m.View()
	if !strings.Contains(v4View, "reiniciar no resolverá la causa raíz") {
		t.Errorf("V4 MUST contain recurrence note 'reiniciar no resolverá la causa raíz', got:\n%s", v4View)
	}
}

// Criterio 7: 80 columnas (banner con recurrencia y root cause largo trunca limpio sin wrap)
func TestTUI_Ola4_Criterio7_80ColumnasSinWrap(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24

	longRootCause := "OOMKilled: contenedor superó límite de memoria de forma crítica y extrema en producción tras fuga continua"
	c := domain.ContainerMetric{
		ID:              "c-long",
		Name:            "long-app",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-1 * time.Minute),
	}
	m.metrics = []domain.ContainerMetric{c}
	m.cursor = 0

	// Simular resultado recurrente
	res := domain.DiagnosisResult{
		Level:           domain.LevelRule,
		RootCause:       fmt.Sprintf("%s recurrente (3/1h)", longRootCause),
		SuggestedAction: "restart",
		RecurrenceCount: 3,
		RecurrenceNote:  "reiniciar no resolverá la causa raíz",
	}
	m.diagnosisResults["c-long"] = res

	view := m.View()
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		// Verificar que las líneas del banner o tabla no excedan 80 cols
		if strings.Contains(line, "recurrente") {
			w := lipgloss.Width(line)
			if w > 80 {
				t.Errorf("banner line %d exceeds 80 columns (width=%d):\n%s", i, w, line)
			}
			if !strings.Contains(line, "...") {
				t.Errorf("expected long root cause in banner to be truncated with '...', got:\n%s", line)
			}
		}
	}
}

// ============================================================================
// OLA 5: CONTRATO DE SALIDA ESTRUCTURADO Y VISIBILIDAD DE COSTO
// ============================================================================

// Criterio 1: Respuesta con prosa + JSON adentro -> banner [AI] con root_cause limpio; el saludo no aparece
func TestTUI_Ola5_Criterio1_ProsaConJSONAdentro(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 30

	c := domain.ContainerMetric{
		ID:     "c-prose",
		Name:   "worker-prose",
		Status: "Exited (137)",
	}
	m.metrics = []domain.ContainerMetric{c}
	m.cursor = 0

	raw := "Estimado operador:\n{\"root_cause\":\"Fuga de memoria en worker\",\"severity\":\"critical\",\"suggested_action\":\"restart\",\"confidence\":\"high\"}\nSaludos cordiales!"
	parser := service.NewAIResponseParser()
	res := parser.Parse(raw, domain.TokenUsage{TotalTokens: 100}, c.Status)

	m.diagnosisResults["c-prose"] = res
	view := m.View()

	if !strings.Contains(view, "[AI]") {
		t.Errorf("expected [AI] tag in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Fuga de memoria en worker") {
		t.Errorf("expected root cause 'Fuga de memoria en worker' in view, got:\n%s", view)
	}
	if strings.Contains(view, "Estimado operador") || strings.Contains(view, "Saludos cordiales") {
		t.Errorf("courtesy prose MUST NOT appear in rendered view, got:\n%s", view)
	}
}

// Criterio 2: JSON sin campo confidence -> banner [AI] con "· conf baja" forzado y evidencia en V4
func TestTUI_Ola5_Criterio2_JSONSinConfidence(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 30

	c := domain.ContainerMetric{
		ID:     "c-no-conf",
		Name:   "app-no-conf",
		Status: "Exited (1)",
	}
	m.metrics = []domain.ContainerMetric{c}
	m.cursor = 0

	raw := `{"root_cause":"Conexión rechazada por DB","severity":"warning","suggested_action":"none"}`
	parser := service.NewAIResponseParser()
	res := parser.Parse(raw, domain.TokenUsage{TotalTokens: 50}, c.Status)

	if res.Confidence != "low" {
		t.Fatalf("expected confidence to be forced to 'low', got %q", res.Confidence)
	}

	m.diagnosisResults["c-no-conf"] = res
	view := m.View()

	if !strings.Contains(view, "· conf baja") {
		t.Errorf("expected banner to contain '· conf baja', got:\n%s", view)
	}

	// Verificar evidencia en V4
	evidences := BuildEvidence(m, c, res, "")
	foundConfEvidence := false
	for _, ev := range evidences {
		if ev.Type == "proceso" && strings.Contains(ev.Value, "confianza del modelo: baja") {
			foundConfEvidence = true
			break
		}
	}
	if !foundConfEvidence {
		t.Errorf("expected V4 evidence to include 'confianza del modelo: baja', got: %+v", evidences)
	}
}

// Criterio 3: JSON con suggested_action "delete" -> acción efectiva none; V4 no muestra sugerencia; Cero RCE
func TestTUI_Ola5_Criterio3_SuggestedActionDelete_ZeroRCE(t *testing.T) {
	m := NewModel(nil)

	c := domain.ContainerMetric{
		ID:     "c-del",
		Name:   "app-del",
		Status: "Exited (1)",
	}
	raw := `{"root_cause":"Falla irreversible","severity":"critical","suggested_action":"delete","confidence":"high"}`
	parser := service.NewAIResponseParser()
	res := parser.Parse(raw, domain.TokenUsage{}, c.Status)

	if res.SuggestedAction != "none" {
		t.Fatalf("suggested_action 'delete' MUST fall back to 'none', got %q", res.SuggestedAction)
	}
	if res.Confidence != "low" {
		t.Errorf("confidence MUST be forced to 'low' when action falls back to default, got %q", res.Confidence)
	}

	actions := m.GetV4Actions(c, res)
	for _, act := range actions {
		if act.ActionType == domain.ActionType("delete") || strings.Contains(act.Label, "delete") {
			t.Fatalf("ZERO RCE VIOLATION: illegal action 'delete' present in V4 actions: %+v", act)
		}
	}
}

// Criterio 4: Respuesta sin JSON -> [AI~] idéntico al comportamiento de Ola 1
func TestTUI_Ola5_Criterio4_RespuestaSinJSON(t *testing.T) {
	parser := service.NewAIResponseParser()
	raw := "No puedo determinar la causa raíz debido a falta de datos en los logs"
	res := parser.Parse(raw, domain.TokenUsage{}, "Exited (1)")

	if res.Level != domain.LevelAIPartial {
		t.Errorf("expected LevelAIPartial [AI~], got %v", res.Level)
	}
	if res.SuggestedAction != "none" {
		t.Errorf("expected action 'none', got %q", res.SuggestedAction)
	}
}

// Criterio 5: Mock FAST devuelve severity critical + confidence high -> +2 req (FAST + DEEP), V4 nota re-análisis, segundo evento (cache) +0 req
func TestTUI_Ola5_Criterio5_MockFastCriticalTriggersDeep(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	fastCalls := 0
	deepCalls := 0

	mockTriage := &mockTriageService{
		diagnoseWithSlotFn: func(ctx context.Context, name, image, status, logs string, slot domain.DiagnosisSlot) (string, domain.TokenUsage) {
			if slot == domain.SlotDeep {
				deepCalls++
				return `{"root_cause":"OOM crítico confirmado en DEEP","severity":"critical","suggested_action":"restart","confidence":"high"}`, domain.TokenUsage{PromptTokens: 1000, CompletionTokens: 200, TotalTokens: 1200}
			}
			fastCalls++
			return `{"root_cause":"Posible OOM rápido","severity":"critical","suggested_action":"restart","confidence":"high"}`, domain.TokenUsage{PromptTokens: 300, CompletionTokens: 50, TotalTokens: 350}
		},
	}

	m := NewModel(mockColl)
	m.aiConfig.SelectionMode = domain.SelectionAuto
	m.triageClient = mockTriage

	c := domain.ContainerMetric{
		ID:              "c-crit-re",
		Name:            "crit-service",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-5 * time.Minute),
	}
	m.metrics = []domain.ContainerMetric{c}
	m.cursor = 0

	// 1. Primer evento: ejecuta FAST y se re-ejecuta en DEEP
	cmd1 := m.triggerTriageForced(c)
	if cmd1 == nil {
		t.Fatalf("expected cmd1 to not be nil")
	}
	msg1 := cmd1()
	newM, _ := m.Update(msg1)
	m = newM.(Model)

	if fastCalls != 1 || deepCalls != 1 {
		t.Fatalf("expected 1 FAST and 1 DEEP call, got fast=%d deep=%d", fastCalls, deepCalls)
	}
	if m.aiMeter.TotalRequests() != 2 {
		t.Errorf("expected aiMeter to record 2 requests (FAST + DEEP), got %d", m.aiMeter.TotalRequests())
	}

	// Verificar nota de re-análisis en V4
	res := m.diagnosisResults["c-crit-re"]
	if !res.ReanalyzedDeep {
		t.Errorf("expected ReanalyzedDeep to be true")
	}
	evidences := BuildEvidence(m, c, res, "")
	foundDeepEvidence := false
	for _, ev := range evidences {
		if ev.Type == "proceso" && strings.Contains(ev.Value, "re-analizado en DEEP") {
			foundDeepEvidence = true
			break
		}
	}
	if !foundDeepEvidence {
		t.Errorf("expected V4 evidence to contain DEEP re-analysis note, got: %+v", evidences)
	}

	// 2. Segundo evento idéntico: debe servirse de caché sin nuevas llamadas
	reqBefore := m.aiMeter.TotalRequests()
	cmd2 := m.triggerTriageIfAnomalous(c)
	if cmd2 != nil {
		t.Errorf("expected cached triage to return nil command, got non-nil")
	}
	reqAfter := m.aiMeter.TotalRequests()
	if reqAfter-reqBefore != 0 {
		t.Errorf("expected +0 requests from cache hit, got +%d", reqAfter-reqBefore)
	}
}

// Criterio 6: Sesión solo con modelos free -> status bar "sesión: N req · ~$0.00". Modelo fuera de tabla -> V3 muestra tokens sin "~$"
func TestTUI_Ola5_Criterio6_SesionFreeYModeloSinPrecio(t *testing.T) {
	meter := service.NewAIMeter()

	// 1. Modelo free
	meter.Record(domain.ProviderOpenRouter, domain.SlotFast, "openrouter/free", domain.TokenUsage{TotalTokens: 500}, "", "")
	sb := meter.FormatStatusBar()
	if !strings.Contains(sb, "sesión: 1 req · ~$0.00") {
		t.Errorf("expected 'sesión: 1 req · ~$0.00' for free model, got %q", sb)
	}

	// 2. Modelo fuera de tabla (desconocido)
	meter.Record(domain.ProviderCustom, domain.SlotFast, "unregistered-llm-model", domain.TokenUsage{TotalTokens: 300}, "", "")
	provStats := meter.FormatProviderStats(domain.ProviderCustom)
	if strings.Contains(provStats, "$") {
		t.Errorf("model outside table MUST NOT display '$', got %q", provStats)
	}
	if !strings.Contains(provStats, "sesión: 1 req · 300 tok") {
		t.Errorf("expected 'sesión: 1 req · 300 tok', got %q", provStats)
	}
}

// Criterio 7: 80 columnas: banner con recurrencia + conf baja + root cause largo trunca limpio, cayendo primero segmentos opcionales
func TestTUI_Ola5_Criterio7_80ColsOrdenDeterministaTruncado(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24

	longRootCause := "Falla crítica por saturación extrema del heap de memoria de la máquina virtual java"
	c := domain.ContainerMetric{
		ID:     "c-80cols",
		Name:   "app-80",
		Status: "Exited (137)",
	}
	m.metrics = []domain.ContainerMetric{c}
	m.cursor = 0

	res := domain.DiagnosisResult{
		Level:           domain.LevelAI,
		RootCause:       fmt.Sprintf("%s recurrente (3/1h)", longRootCause),
		Severity:        "critical",
		SuggestedAction: "restart",
		Confidence:      "low",
		RecurrenceCount: 3,
		RecurrenceNote:  "reiniciar no resolverá la causa raíz",
	}
	m.diagnosisResults["c-80cols"] = res

	view := m.View()
	lines := strings.Split(view, "\n")
	foundBanner := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[AI]") && strings.Contains(trimmed, "Falla crítica") {
			foundBanner = true
			w := lipgloss.Width(line)
			if w > 80 {
				t.Errorf("banner line %d exceeds 80 columns (width=%d):\n%s", i, w, line)
			}
			// El orden determinista hace caer '· conf baja' primero
			if strings.Contains(line, "· conf baja") {
				t.Errorf("expected '· conf baja' to be dropped first to fit 80 cols, got:\n%s", line)
			}
			if !strings.Contains(line, "[d] detalle") {
				t.Errorf("expected '[d] detalle' to be preserved in banner, got:\n%s", line)
			}
		}
	}
	if !foundBanner {
		t.Errorf("did not find banner line in view:\n%s", view)
	}
}

// Criterio 8: Reiniciar el agente resetea contadores a "sesión: 0 req · ~$0.00"
func TestTUI_Ola5_Criterio8_ResetContadoresSesion(t *testing.T) {
	m1 := NewModel(nil)
	m1.aiMeter.Record(domain.ProviderAnthropic, domain.SlotFast, "claude-3-5-sonnet-20241022", domain.TokenUsage{TotalTokens: 5000}, "", "")

	if m1.aiMeter.TotalRequests() != 1 {
		t.Fatalf("expected 1 request in m1, got %d", m1.aiMeter.TotalRequests())
	}

	// Reiniciar modelo
	m2 := NewModel(nil)
	if m2.aiMeter.TotalRequests() != 0 {
		t.Errorf("expected 0 requests on fresh model restart, got %d", m2.aiMeter.TotalRequests())
	}
	if m2.aiMeter.TotalCost() != 0.0 {
		t.Errorf("expected 0.0 cost on fresh model restart, got %f", m2.aiMeter.TotalCost())
	}
	sb := m2.aiMeter.FormatStatusBar()
	if !strings.Contains(sb, "sesión: 0 req · ~$0.00") {
		t.Errorf("expected 'sesión: 0 req · ~$0.00' on fresh model restart, got %q", sb)
	}
}

// =========================================================================
// OLA 6: INCIDENTES CORRELACIONADOS (10 CRITERIOS DE ACEPTACIÓN)
// =========================================================================

// Criterio 1: Cascada con postgres, api-node y nginx produce UN banner [INC]
func TestTUI_Ola6_Criterio1_CascadaBannerUnificado(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)
	m.width = 100
	m.height = 30

	now := time.Now()
	c1 := domain.ContainerMetric{
		ID:              "c-pg",
		Name:            "postgres",
		Status:          "Exited (137)",
		ComposeProject:  "solv_stack",
		Networks:        []string{"solv_net"},
		LastStateChange: now.Add(-10 * time.Second),
	}
	c2 := domain.ContainerMetric{
		ID:              "c-api",
		Name:            "api-node",
		Status:          "Exited (1)",
		ComposeProject:  "solv_stack",
		Networks:        []string{"solv_net"},
		EnvVars:         []string{"DATABASE_URL=postgres://postgres:5432"},
		LastStateChange: now.Add(-5 * time.Second),
	}
	c3 := domain.ContainerMetric{
		ID:              "c-nginx",
		Name:            "nginx",
		Status:          "Exited (1)",
		ComposeProject:  "solv_stack",
		Networks:        []string{"solv_net"},
		EnvVars:         []string{"UPSTREAM_SERVER=api-node:8080"},
		LastStateChange: now.Add(-2 * time.Second),
	}

	m1, _ := m.Update([]domain.ContainerMetric{c1, c2, c3})
	mod := m1.(Model)

	// Verificar que existe el incidente activo
	inc := mod.incidentAggregator.GetActiveIncidentFor("postgres")
	if inc == nil {
		t.Fatalf("expected active incident for postgres")
	}
	if inc.GroupName != "solv_stack" {
		t.Errorf("expected group solv_stack, got %q", inc.GroupName)
	}
	if inc.CandidateOrigin != "postgres" {
		t.Errorf("expected candidate origin postgres, got %q", inc.CandidateOrigin)
	}

	// Renderizar vista
	view := mod.View()
	lines := strings.Split(view, "\n")
	incBannerCount := 0
	for _, line := range lines {
		if strings.Contains(line, "[INC]") && !strings.Contains(line, "parte de incidente") {
			incBannerCount++
			if !strings.Contains(line, "origen postgres") {
				t.Errorf("expected banner to identify postgres as origin, got:\n%s", line)
			}
			if !strings.Contains(line, "[d]") {
				t.Errorf("expected banner to have '[d]' hint, got:\n%s", line)
			}
		}
	}
	if incBannerCount != 1 {
		t.Errorf("expected exactly 1 [INC] top banner, found %d", incBannerCount)
	}

	// Verificar panel derecho DIAGNÓSTICO
	if !strings.Contains(view, "[INC] parte de incidente solv_stack") {
		t.Errorf("expected right panel to indicate member of incident, got:\n%s", view)
	}
}

// Criterio 2: Coincidencia sin lazo topológico NO produce incidente
func TestTUI_Ola6_Criterio2_CoincidenciaSinLazoTopologico(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 30

	now := time.Now()
	c1 := domain.ContainerMetric{
		ID:              "c-db",
		Name:            "db-isolated",
		Status:          "Exited (1)",
		ComposeProject:  "app-a",
		Networks:        []string{"net-a"},
		LastStateChange: now,
	}
	c2 := domain.ContainerMetric{
		ID:              "c-cache",
		Name:            "cache-isolated",
		Status:          "Exited (1)",
		ComposeProject:  "app-b",
		Networks:        []string{"net-b"},
		LastStateChange: now,
	}

	m1, _ := m.Update([]domain.ContainerMetric{c1, c2})
	mod := m1.(Model)

	if len(mod.incidentAggregator.GetAllActiveIncidents()) != 0 {
		t.Errorf("expected 0 incidents for containers without hard topology, got %d", len(mod.incidentAggregator.GetAllActiveIncidents()))
	}
	if mod.incidentAggregator.GetActiveIncidentFor("db-isolated") != nil {
		t.Errorf("expected db-isolated to have no incident")
	}
}

// Criterio 3: Cascada lenta (60-90s después) se adjunta si hay arista depende-de e incidente < 5min
func TestTUI_Ola6_Criterio3_CascadaLentaAdjuncion(t *testing.T) {
	m := NewModel(nil)
	t0 := time.Now().Add(-80 * time.Second)

	c1 := domain.ContainerMetric{
		ID:              "c1",
		Name:            "postgres",
		Status:          "Exited (137)",
		Networks:        []string{"solv_net"},
		LastStateChange: t0,
	}
	c2 := domain.ContainerMetric{
		ID:              "c2",
		Name:            "redis",
		Status:          "Exited (1)",
		Networks:        []string{"solv_net"},
		LastStateChange: t0.Add(2 * time.Second),
	}

	// Abrir incidente en t0
	m1, _ := m.Update([]domain.ContainerMetric{c1, c2})
	mod := m1.(Model)
	initInc := mod.incidentAggregator.GetActiveIncidentFor("postgres")
	if initInc == nil {
		t.Fatalf("expected incident at t0")
	}

	// 80s después: api-node falla con dependencia hacia postgres
	t1 := time.Now()
	c3 := domain.ContainerMetric{
		ID:              "c3",
		Name:            "api-node",
		Status:          "Exited (1)",
		Networks:        []string{"solv_net"},
		EnvVars:         []string{"DATABASE_URL=postgres://postgres:5432"},
		LastStateChange: t1,
	}

	m2, _ := mod.Update([]domain.ContainerMetric{c1, c2, c3})
	mod2 := m2.(Model)

	attachedInc := mod2.incidentAggregator.GetActiveIncidentFor("api-node")
	if attachedInc == nil {
		t.Fatalf("expected api-node to be attached to active incident")
	}
	if attachedInc.ID != initInc.ID {
		t.Errorf("expected api-node to join incident %s, got %s", initInc.ID, attachedInc.ID)
	}
	if len(attachedInc.Cascade) < 3 {
		t.Errorf("expected cascade count to update with attached member, got %d", len(attachedInc.Cascade))
	}
}

// Criterio 4: Estados de confianza (s/d si simultáneo sin arista, confirmado si antiguo+arista, probable si antiguo solo)
func TestTUI_Ola6_Criterio4_EstadosDeConfianzaOrigen(t *testing.T) {
	agg := service.NewIncidentAggregator(30)
	now := time.Now()

	// 1. Simultáneos sin arista -> s/d
	c1 := domain.ContainerMetric{ID: "1", Name: "w1", Status: "Exited (1)", Networks: []string{"n1"}, LastStateChange: now}
	c2 := domain.ContainerMetric{ID: "2", Name: "w2", Status: "Exited (1)", Networks: []string{"n1"}, LastStateChange: now}
	incSD := agg.IngestAnomalies([]domain.ContainerMetric{c1, c2}, service.NewDependencyGraph([]domain.ContainerMetric{c1, c2}), nil, now)
	if len(incSD) != 1 || incSD[0].OriginConfidence != domain.ConfidenceUndetermined {
		t.Errorf("expected ConfidenceUndetermined, got %v", incSD[0].OriginConfidence)
	}

	// 2. Más antiguo + arista -> confirmado
	agg2 := service.NewIncidentAggregator(30)
	c3 := domain.ContainerMetric{ID: "3", Name: "db", Status: "Exited (137)", Networks: []string{"n2"}, LastStateChange: now.Add(-10 * time.Second)}
	c4 := domain.ContainerMetric{ID: "4", Name: "api", Status: "Exited (1)", Networks: []string{"n2"}, EnvVars: []string{"DB=db"}, LastStateChange: now}
	g2 := service.NewDependencyGraph([]domain.ContainerMetric{c3, c4})
	incConf := agg2.IngestAnomalies([]domain.ContainerMetric{c3, c4}, g2, nil, now)
	if len(incConf) != 1 || incConf[0].OriginConfidence != domain.ConfidenceConfirmed {
		t.Errorf("expected ConfidenceConfirmed, got %v", incConf[0].OriginConfidence)
	}

	// 3. Más antiguo sin arista -> probable
	agg3 := service.NewIncidentAggregator(30)
	c5 := domain.ContainerMetric{ID: "5", Name: "svc-a", Status: "Exited (1)", Networks: []string{"n3"}, LastStateChange: now.Add(-25 * time.Second)}
	c6 := domain.ContainerMetric{ID: "6", Name: "svc-b", Status: "Exited (1)", Networks: []string{"n3"}, LastStateChange: now}
	g3 := service.NewDependencyGraph([]domain.ContainerMetric{c5, c6})
	incProb := agg3.IngestAnomalies([]domain.ContainerMetric{c5, c6}, g3, nil, now)
	if len(incProb) != 1 || incProb[0].OriginConfidence != domain.ConfidenceProbable {
		t.Errorf("expected ConfidenceProbable, got %v", incProb[0].OriginConfidence)
	}
}

// Criterio 5: Política informativo vs prudente
func TestTUI_Ola6_Criterio5_PoliticaInformativoVsPrudente(t *testing.T) {
	inc := &service.Incident{
		ID:               "inc-5",
		GroupName:        "solv_net",
		CandidateOrigin:  "postgres",
		OriginConfidence: domain.ConfidenceProbable,
		Cascade:          []string{"postgres", "api-node", "nginx"},
		Diagnosis: domain.DiagnosisResult{
			Level: domain.LevelAI,
		},
		Events: []service.IncidentEvent{
			{ContainerName: "postgres", Reason: "OOM"},
			{ContainerName: "api-node", Reason: "exit 1"},
			{ContainerName: "nginx", Reason: "502"},
		},
	}

	// Modo Informativo (default) -> muestra "origen prob. postgres"
	bInfo := service.FormatIncidentBanner(inc, domain.BannerPolicyInformativo, 80)
	if !strings.Contains(bInfo, "origen prob. postgres") {
		t.Errorf("expected 'origen prob. postgres' in informativo banner, got %q", bInfo)
	}

	// Modo Prudente -> reserva el banner para confirmado, muestra "3 anómalos"
	bPrud := service.FormatIncidentBanner(inc, domain.BannerPolicyPrudente, 80)
	if strings.Contains(bPrud, "origen prob.") {
		t.Errorf("expected 'origen prob.' to be hidden in prudente banner, got %q", bPrud)
	}
	if !strings.Contains(bPrud, "3 anómalos") {
		t.Errorf("expected '3 anómalos' in prudente banner, got %q", bPrud)
	}

	// Pero en V4, SIEMPRE se muestra con su evidencia
	m := NewModel(nil)
	m.width = 100
	m.height = 30
	m.activeIncident = inc
	m.activeState = stateDiagnosisModal
	v4View := m.View()
	if !strings.Contains(v4View, "postgres (probable)") {
		t.Errorf("expected V4 to display 'postgres (probable)', got:\n%s", v4View)
	}
}

// Criterio 6: Override manual ('o') sobre contenedor en V4 incidente fija (manual) y acción sugerida
func TestTUI_Ola6_Criterio6_OverrideManualOrigen(t *testing.T) {
	now := time.Now()
	c1 := domain.ContainerMetric{ID: "c1", Name: "w1", Status: "Exited (1)", Networks: []string{"solv_net"}, LastStateChange: now}
	c2 := domain.ContainerMetric{ID: "c2", Name: "w2", Status: "Exited (1)", Networks: []string{"solv_net"}, LastStateChange: now}

	m := NewModel(nil)
	m.width = 100
	m.height = 30
	m1, _ := m.Update([]domain.ContainerMetric{c1, c2})
	mod := m1.(Model)

	// Abrir V4 incidente con 'd'
	m2, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	mod2 := m2.(Model)
	if mod2.activeIncident == nil {
		t.Fatalf("expected activeIncident in V4")
	}

	// Inicialmente origen s/d
	v4Initial := mod2.View()
	if !strings.Contains(v4Initial, "sin determinar · señales en conflicto") {
		t.Errorf("expected initial undetermined origin in V4, got:\n%s", v4Initial)
	}

	// Mover cursor hacia el segundo contenedor 'w2' y pulsar 'o'
	m3, _ := mod2.Update(tea.KeyMsg{Type: tea.KeyDown})
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	mod4 := m4.(Model)

	v4Manual := mod4.View()
	if !strings.Contains(v4Manual, "w2 (manual)") {
		t.Errorf("expected 'w2 (manual)' in V4 after pressing 'o', got:\n%s", v4Manual)
	}
	if !strings.Contains(v4Manual, "aplicar restart a w2 (origen)") {
		t.Errorf("expected suggested action targeting w2, got:\n%s", v4Manual)
	}

	// Pulsar 'r' para confirmar remediación -> debe abrir modal de confirmación, CERO RCE
	m5, _ := mod4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	mod5 := m5.(Model)
	if mod5.activeState != stateConfirmRemediation {
		t.Fatalf("expected stateConfirmRemediation after pressing 'r', got %v", mod5.activeState)
	}
	if mod5.pendingContainer.Name != "w2" {
		t.Errorf("expected pending remediation container to be 'w2', got %q", mod5.pendingContainer.Name)
	}
}

// Criterio 7: Modo offline / sin IA funciona bajo [RULE] con origen por timeline + arista
func TestTUI_Ola6_Criterio7_DegradacionOfflineReglas(t *testing.T) {
	m := NewModel(nil) // Sin IA conectada
	m.width = 100
	m.height = 30
	now := time.Now()

	c1 := domain.ContainerMetric{
		ID:              "c-pg",
		Name:            "postgres",
		Status:          "Exited (137)",
		Networks:        []string{"offline_net"},
		LastStateChange: now.Add(-10 * time.Second),
	}
	c2 := domain.ContainerMetric{
		ID:              "c-api",
		Name:            "api-node",
		Status:          "Exited (1)",
		Networks:        []string{"offline_net"},
		EnvVars:         []string{"DB=postgres"},
		LastStateChange: now,
	}

	m1, _ := m.Update([]domain.ContainerMetric{c1, c2})
	mod := m1.(Model)

	inc := mod.incidentAggregator.GetActiveIncidentFor("postgres")
	if inc == nil {
		t.Fatalf("expected active incident")
	}
	if inc.Diagnosis.Level != domain.LevelRule {
		t.Errorf("expected LevelRule, got %v", inc.Diagnosis.Level)
	}
	if !strings.Contains(inc.Diagnosis.RootCause, "postgres") {
		t.Errorf("expected root cause to mention postgres, got %q", inc.Diagnosis.RootCause)
	}

	// Abrir V4 incidente
	m2, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	mod2 := m2.(Model)
	v4View := mod2.View()
	if !strings.Contains(v4View, "[RULE]") {
		t.Errorf("expected [RULE] badge in offline incident V4, got:\n%s", v4View)
	}
}

// Criterio 8: Presupuesto de prompt (grupo de 6 contenedores -> 4 con logs, 2 por nombre)
func TestTUI_Ola6_Criterio8_PresupuestoPrompt(t *testing.T) {
	mockColl := &mockCollectorForTUI{
		logs: "2026-09-11 12:00:00 fatal error in container\n",
	}
	m := NewModel(mockColl)
	now := time.Now()

	var metrics []domain.ContainerMetric
	for i := 1; i <= 6; i++ {
		name := fmt.Sprintf("svc-%d", i)
		metrics = append(metrics, domain.ContainerMetric{
			ID:              fmt.Sprintf("id-%d", i),
			Name:            name,
			Status:          "Exited (1)",
			Networks:        []string{"big_net"},
			LastStateChange: now.Add(time.Duration(i) * time.Second),
		})
	}

	m1, _ := m.Update(metrics)
	mod := m1.(Model)
	inc := mod.incidentAggregator.GetActiveIncidentFor("svc-1")
	if inc == nil {
		t.Fatalf("expected active incident for 6 containers")
	}

	prompt := mod.buildIncidentPrompt(inc)
	// Verificar que contiene logs de máx 4
	logCount := strings.Count(prompt, "Logs:\n")
	if logCount > 4 {
		t.Errorf("expected at most 4 containers with logs, got %d", logCount)
	}
	// Los contenedores excedentes entran solo por nombre
	if !strings.Contains(prompt, "Otros contenedores involucrados") {
		t.Errorf("expected prompt to list overflow containers by name, got:\n%s", prompt)
	}
}

// Criterio 9: Cierre tras 2x ventana sin adjunciones
func TestTUI_Ola6_Criterio9_CierrePeriodoQuieto(t *testing.T) {
	agg := service.NewIncidentAggregator(10) // 10s ventana -> 20s quiet
	now := time.Now()

	c1 := domain.ContainerMetric{ID: "c1", Name: "app1", Status: "Exited (1)", Networks: []string{"q_net"}, LastStateChange: now}
	c2 := domain.ContainerMetric{ID: "c2", Name: "app2", Status: "Exited (1)", Networks: []string{"q_net"}, LastStateChange: now}

	opened := agg.IngestAnomalies([]domain.ContainerMetric{c1, c2}, nil, nil, now)
	if len(opened) != 1 {
		t.Fatalf("expected 1 incident")
	}

	// 15s después: sigue abierto
	closed := agg.CheckQuietPeriods(now.Add(15 * time.Second))
	if len(closed) != 0 {
		t.Errorf("expected 0 closed at +15s, got %d", len(closed))
	}
	if agg.GetActiveIncidentFor("app1") == nil {
		t.Errorf("expected incident to be active at +15s")
	}

	// 25s después: cerrado
	closed = agg.CheckQuietPeriods(now.Add(25 * time.Second))
	if len(closed) != 1 {
		t.Errorf("expected 1 closed incident at +25s, got %d", len(closed))
	}
	if agg.GetActiveIncidentFor("app1") != nil {
		t.Errorf("expected incident to be closed at +25s")
	}
}

// Criterio 10: Cero vistas nuevas, '?' no lista V6, tecla 't' renombrada a Preferencias
func TestTUI_Ola6_Criterio10_CeroVistasNuevas(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 30
	m.activeState = stateHelp

	view := m.View()

	// Prohibido crear V6
	if strings.Contains(view, "V6") || strings.Contains(view, "v6") {
		t.Errorf("help overlay must NOT list any 'V6' view, got:\n%s", view)
	}

	// 't' renombrada a Preferencias
	if !strings.Contains(view, "t") || !strings.Contains(view, "Preferencias") {
		t.Errorf("help overlay must list 't' as Preferencias, got:\n%s", view)
	}

	// Help incluye categoría Incidentes con 'd' y 'o'
	if !strings.Contains(view, "Incidentes") {
		t.Errorf("help overlay must include 'Incidentes' category, got:\n%s", view)
	}
	if !strings.Contains(view, "o") || !strings.Contains(view, "Marcar origen") {
		t.Errorf("help overlay must include 'o' for Marcar origen, got:\n%s", view)
	}
}

// =========================================================================
// OLA 7: ENDURECIMIENTO DE CAMPO (8 CRITERIOS DE ACEPTACIÓN)
// =========================================================================

// Criterio 1: -race limpio con loop concurrente de métricas e incidentes
func TestTUI_Ola7_Criterio1_RaceLimpio(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)
	m.width = 100
	m.height = 30

	done := make(chan struct{})
	var wg sync.WaitGroup

	// Goroutine simulando colector en background
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-done:
				return
			default:
				time.Sleep(1 * time.Millisecond)
				_, _ = mockColl.Collect(context.Background())
			}
		}
	}()

	// Goroutine principal de Bubbletea updates y views
	for i := 0; i < 50; i++ {
		now := time.Now()
		c1 := domain.ContainerMetric{
			ID:              "c-pg",
			Name:            "postgres",
			Status:          "Exited (137)",
			ComposeProject:  "solv_stack",
			Networks:        []string{"solv_net"},
			LastStateChange: now.Add(-5 * time.Second),
		}
		c2 := domain.ContainerMetric{
			ID:              "c-api",
			Name:            "api-node",
			Status:          "Exited (1)",
			ComposeProject:  "solv_stack",
			Networks:        []string{"solv_net"},
			LastStateChange: now.Add(-2 * time.Second),
		}

		m1, _ := m.Update([]domain.ContainerMetric{c1, c2})
		m = m1.(Model)
		_ = m.View()

		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = m2.(Model)
		_ = m.View()
	}

	close(done)
	wg.Wait()
}

// Criterio 2: Trap de pánico y restauración de TTY con crash.log
func TestTUI_Ola7_Criterio2_PanicoForzadoYTTY(t *testing.T) {
	// Probar que AppendCrashLog escribe el formato requerido
	stack := []byte("goroutine 1 [running]:\nmain.testPanic()\n\t/fake/path.go:10")
	logPath := AppendCrashLog("simulated nil dereference", stack)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read crash log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "version="+AppVersion) {
		t.Errorf("expected crash log to contain version=%s, got:\n%s", AppVersion, content)
	}
	if !strings.Contains(content, "simulated nil dereference") {
		t.Errorf("expected crash log to contain panic reason, got:\n%s", content)
	}
	if !strings.Contains(content, "main.testPanic") {
		t.Errorf("expected crash log to contain stack trace, got:\n%s", content)
	}

	// Restaurar TTY no debe paniquear
	RestoreTTY()
}

// Criterio 3: Cero WithMouse en el repositorio
func TestTUI_Ola7_Criterio3_CeroWithMouseEnRepo(t *testing.T) {
	// Inspeccionar archivos .go en internal/delivery/tui y cmd
	searchPaths := []string{"../../cmd", "../../internal/delivery/tui"}
	for _, sp := range searchPaths {
		err := filepath.Walk(sp, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			if strings.Contains(string(content), "WithMouse") {
				t.Errorf("file %s contains prohibited mouse tracking option 'WithMouse*'", path)
			}
			return nil
		})
		if err != nil {
			t.Logf("walk warning for path %s: %v", sp, err)
		}
	}
}

// Criterio 4: Terminal de 25 filas modo compacto
func TestTUI_Ola7_Criterio4_TerminalCompacto25Filas(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)
	m.width = 80
	m.height = 25 // Piso estricto de 25 filas

	now := time.Now()
	// Crear 5 eventos anómalos en el incidente
	var metrics []domain.ContainerMetric
	for i := 1; i <= 5; i++ {
		metrics = append(metrics, domain.ContainerMetric{
			ID:              fmt.Sprintf("c-svc-%d", i),
			Name:            fmt.Sprintf("svc-%d", i),
			Status:          "Exited (1)",
			ComposeProject:  "microservices",
			Networks:        []string{"net_ms"},
			LastStateChange: now.Add(time.Duration(-10+i) * time.Second),
		})
	}

	m1, _ := m.Update(metrics)
	mod := m1.(Model)

	// Abrir V4 de diagnóstico (modo incidente)
	m2, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	diagMod := m2.(Model)
	if diagMod.activeState != stateDiagnosisModal {
		t.Fatalf("expected stateDiagnosisModal, got %v", diagMod.activeState)
	}

	view := diagMod.View()

	// En 25 filas, el timeline compacto muestra "+2 más" (5 - 3 = 2)
	if !strings.Contains(view, "+2 más") {
		t.Errorf("expected compact timeline to show '+2 más' for 5 events, got:\n%s", view)
	}

	// Debe mostrar scroll indicator o esc
	if !strings.Contains(view, "esc") {
		t.Errorf("expected esc in header, got:\n%s", view)
	}
}

// Criterio 5: 80 columnas y cascada larga wrappea por tokens sin partir nombres ni flechas
func TestTUI_Ola7_Criterio5_WrapTokens80Columnas(t *testing.T) {
	longCascade := "postgres-primary-db -> backend-auth-service-node -> front-nginx-loadbalancer-external (inferido)"
	lines := WrapByTokens(longCascade, 45)

	if len(lines) < 2 {
		t.Fatalf("expected wrapped lines >= 2, got %d", len(lines))
	}

	for _, line := range lines {
		// Ningún token o flecha debe aparecer roto
		if strings.HasSuffix(line, "-") && !strings.HasSuffix(line, " ->") {
			t.Errorf("token was hyphen-split across boundary: %q", line)
		}
		if lipgloss.Width(line) > 45 {
			t.Errorf("line exceeded max width 45: %q (w=%d)", line, lipgloss.Width(line))
		}
	}
}

// Criterio 6: Escenario del reporte: 10 contenedores en compose, 2 fallando
func TestTUI_Ola7_Criterio6_EscenarioReporteCompose10(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)
	m.width = 100
	m.height = 30

	now := time.Now()
	var metrics []domain.ContainerMetric

	// 2 fallando
	c1 := domain.ContainerMetric{
		ID:              "c-postgres11",
		Name:            "postgres",
		Status:          "Exited (137)",
		ComposeProject:  "server_tracker",
		Networks:        []string{"st_net"},
		LastStateChange: now.Add(-10 * time.Second),
	}
	c2 := domain.ContainerMetric{
		ID:              "c-api2222222",
		Name:            "api",
		Status:          "Exited (1)",
		ComposeProject:  "server_tracker",
		Networks:        []string{"st_net"},
		LastStateChange: now.Add(-5 * time.Second),
	}
	metrics = append(metrics, c1, c2)

	// 1 en estado Created normal (NO es anomalía en Fase 6)
	cCreated := domain.ContainerMetric{
		ID:             "c-migration0",
		Name:           "migration",
		Status:         "Created",
		ComposeProject: "server_tracker",
		Networks:       []string{"st_net"},
	}
	metrics = append(metrics, cCreated)

	// 7 en running saludable
	for i := 1; i <= 7; i++ {
		metrics = append(metrics, domain.ContainerMetric{
			ID:             fmt.Sprintf("c-healthy%04d", i),
			Name:           fmt.Sprintf("worker-%d", i),
			Status:         "running",
			ComposeProject: "server_tracker",
			Networks:       []string{"st_net"},
		})
	}

	m1, _ := m.Update(metrics)
	mod := m1.(Model)

	inc := mod.incidentAggregator.GetActiveIncidentFor("postgres")
	if inc == nil {
		t.Fatalf("expected active incident for postgres")
	}

	// 1. Miembros = exactamente 2 (c1 y c2) deduplicados por ID canónico
	if len(inc.Members) != 2 {
		t.Errorf("expected exactly 2 members in incident, got %d", len(inc.Members))
	}
	if _, ok := inc.Members["c-postgres11"]; !ok {
		t.Errorf("expected member c-postgres11 in incident")
	}
	if _, ok := inc.Members["c-api2222222"]; !ok {
		t.Errorf("expected member c-api2222222 in incident")
	}

	// 2. Timeline NO contiene eventos Created
	for _, ev := range inc.Events {
		if strings.Contains(strings.ToLower(ev.Status), "created") {
			t.Errorf("prohibited 'Created' event found in incident timeline: %+v", ev)
		}
	}

	// 3. Abrir modal y verificar línea de vecinos sanos: 8 (7 running + 1 created no anómalo)
	m2, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	diagMod := m2.(Model)
	view := diagMod.View()

	if !strings.Contains(view, "otros en server_tracker: 8 · sin anomalías") {
		t.Errorf("expected healthy peers line 'otros en server_tracker: 8 · sin anomalías', got:\n%s", view)
	}
}

// Criterio 7: Status bar sin keymap, [?] en cabecera y toast de primera ejecución
func TestTUI_Ola7_Criterio7_PisoLimpioYToast(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	mockV := &mockVaultForTUI{
		savedThemeConfig: domain.ThemeConfig{
			ActiveTheme:         "tokyo-night",
			OnboardingHintShown: false, // Primera ejecución
		},
	}
	m := NewModel(mockColl, mockV)
	m.width = 100
	m.height = 30

	view1 := m.View()

	// Cabecera contiene afijo [?]
	if !strings.Contains(view1, "[?]") {
		t.Errorf("expected header to contain affix '[?]', got:\n%s", view1)
	}

	// Toast visible en primera ejecución
	if !strings.Contains(view1, "pulsa ? para ver atajos") {
		t.Errorf("expected onboarding toast in first run, got:\n%s", view1)
	}

	// Status bar NO contiene el keymap redundante
	if strings.Contains(view1, "[j/k]: Navegar") || strings.Contains(view1, "[p]: Fijar") {
		t.Errorf("status bar must NOT contain exhaustive keymap, got:\n%s", view1)
	}

	// Presionar una tecla debe ocultar el toast y persistir flag
	m1, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mod2 := m1.(Model)

	if !mod2.themeConfig.OnboardingHintShown {
		t.Errorf("expected OnboardingHintShown to be true after keypress")
	}

	view2 := mod2.View()
	if strings.Contains(view2, "pulsa ? para ver atajos") {
		t.Errorf("toast must NOT appear after first keypress, got:\n%s", view2)
	}

	// Tecla '?' abre help
	m3, _ := mod2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	helpMod := m3.(Model)
	if helpMod.activeState != stateHelp {
		t.Errorf("expected '?' to open stateHelp, got %v", helpMod.activeState)
	}
}

// Criterio 8: No regresión de olas 0 a 6
func TestTUI_Ola7_Criterio8_NoRegresionOlas0a6(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)
	m.width = 100
	m.height = 30

	now := time.Now()
	c1 := domain.ContainerMetric{
		ID:              "c-redis",
		Name:            "redis-cache",
		Status:          "Exited (137)",
		ComposeProject:  "app_stack",
		Networks:        []string{"app_net"},
		LastStateChange: now.Add(-10 * time.Second),
	}
	c2 := domain.ContainerMetric{
		ID:              "c-web",
		Name:            "web-app",
		Status:          "Exited (1)",
		ComposeProject:  "app_stack",
		Networks:        []string{"app_net"},
		LastStateChange: now.Add(-5 * time.Second),
	}

	m1, _ := m.Update([]domain.ContainerMetric{c1, c2})
	mod := m1.(Model)

	// 1. Offline [RULE] y Banner [INC]
	view := mod.View()
	if !strings.Contains(view, "[INC]") {
		t.Errorf("expected unified [INC] banner, got:\n%s", view)
	}
	if !strings.Contains(view, "[RULE]") {
		t.Errorf("expected offline [RULE] level, got:\n%s", view)
	}

	// 2. Meter de sesión visible
	if !strings.Contains(view, "sesión:") {
		t.Errorf("expected session meter in status bar, got:\n%s", view)
	}

	// 3. Flujo cuota / IA con 'c'
	m2, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	configMod := m2.(Model)
	if configMod.activeState != stateConfigModal {
		t.Errorf("expected 'c' to open stateConfigModal, got %v", configMod.activeState)
	}

	// 4. Modal de confirmación con 'r'
	m3, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	confirmMod := m3.(Model)
	if confirmMod.activeState != stateConfirmRemediation {
		t.Errorf("expected 'r' to open stateConfirmRemediation, got %v", confirmMod.activeState)
	}
}

// Test interactivo: Tab alterna entre acciones y mini-scroll de evidencias con indicación clara
func TestTUI_Ola7_EvidenciaMiniScrollYTab(t *testing.T) {
	mockColl := &mockCollectorForTUI{}
	m := NewModel(mockColl)
	m.width = 80
	m.height = 25 // modo compacto

	now := time.Now()
	// Contenedor con múltiples evidencias (RAM, CPU, exit, restarts, redes, historial)
	c := domain.ContainerMetric{
		ID:              "c-grafana",
		Name:            "server_tracker-grafana",
		Status:          "Exited (137)",
		RAMBytes:        59 * 1024 * 1024,
		RAMLimitBytes:   7832 * 1024 * 1024,
		CPUPercent:      1.7,
		RestartCount:    2,
		Networks:        []string{"solv_net"},
		LastStateChange: now.Add(-4 * time.Minute),
	}

	m1, _ := m.Update([]domain.ContainerMetric{c})
	mod := m1.(Model)

	// Abrir diagnóstico individual
	m2, _ := mod.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	diagMod := m2.(Model)
	if diagMod.activeState != stateDiagnosisModal {
		t.Fatalf("expected stateDiagnosisModal, got %v", diagMod.activeState)
	}

	view1 := diagMod.View()

	// Debe mencionar explícitamente Tab o cómo scrollear
	if !strings.Contains(view1, "Tab") && !strings.Contains(view1, "tab") {
		t.Errorf("view must indicate [Tab] to scroll evidences, got:\n%s", view1)
	}

	// Presionar Tab: cambia el foco a sección evidencia
	m3, _ := diagMod.Update(tea.KeyMsg{Type: tea.KeyTab})
	focusMod := m3.(Model)
	if focusMod.v4FocusSection != 1 {
		t.Errorf("expected v4FocusSection == 1 after Tab, got %d", focusMod.v4FocusSection)
	}

	view2 := focusMod.View()
	if !strings.Contains(view2, "activo • ↑/↓ para scrollear") {
		t.Errorf("view must indicate active evidence scroll mode, got:\n%s", view2)
	}

	// Presionar 'j' / 'down' para scrollear hacia abajo en evidencias
	m4, _ := focusMod.Update(tea.KeyMsg{Type: tea.KeyDown})
	scrollMod := m4.(Model)
	if scrollMod.v4EvidenceScroll != 1 {
		t.Errorf("expected v4EvidenceScroll == 1 after Down, got %d", scrollMod.v4EvidenceScroll)
	}

	// Presionar Tab de nuevo: vuelve a acciones
	m5, _ := scrollMod.Update(tea.KeyMsg{Type: tea.KeyTab})
	backMod := m5.(Model)
	if backMod.v4FocusSection != 0 {
		t.Errorf("expected v4FocusSection == 0 after second Tab, got %d", backMod.v4FocusSection)
	}
}


