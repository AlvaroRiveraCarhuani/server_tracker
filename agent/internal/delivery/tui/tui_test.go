package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
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
	calls int
	diag  string
}

func (m *mockTriageService) DiagnoseContainer(ctx context.Context, name, image, status, logs string) string {
	m.calls++
	return m.diag
}

func (m *mockTriageService) DiagnoseContainerWithUsage(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage) {
	m.calls++
	return m.diag, domain.TokenUsage{PromptTokens: 120, CompletionTokens: 25, TotalTokens: 145, EstimatedCostUSD: 0.0003}
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
		diag: "Database connection timeout -> Verificar conectividad y credenciales de BD",
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

	// 1. Abrir modal con 't'
	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	mod := m1.(Model)
	if mod.activeState != stateThemeModal {
		t.Fatalf("expected stateThemeModal after pressing 't', got %v", mod.activeState)
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



