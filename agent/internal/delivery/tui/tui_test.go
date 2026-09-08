package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	tea "github.com/charmbracelet/bubbletea"
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
	model.triageClient = mockAI
	model.metrics = metrics
	model.cursor = 0

	// 1. Contenedor sano: no debe renderizar banner AIOps
	renderedHealthy := model.View()
	if strings.Contains(renderedHealthy, "[AIOps]") {
		t.Errorf("expected no AIOps banner on healthy container, got:\n%s", renderedHealthy)
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
		t.Fatalf("expected diagnosisResultMsg from triage cmd, got %T", msg)
	}
	if diagMsg.containerID != "c-crashed" {
		t.Errorf("expected diagnosis for c-crashed, got %s", diagMsg.containerID)
	}
	if mockAI.calls != 1 {
		t.Errorf("expected exactly 1 AI triage call, got %d", mockAI.calls)
	}

	// 4. Enviar diagnosisResultMsg al modelo y comprobar que se guarda en caché
	newModel, _ = m.Update(diagMsg)
	m = newModel.(Model)
	if cachedDiag, exists := m.diagnosisCache["c-crashed"]; !exists || cachedDiag != mockAI.diag {
		t.Errorf("expected cached diagnosis in model, got: %s (exists=%v)", cachedDiag, exists)
	}

	// 5. Renderizar vista con el contenedor anómalo en foco: debe mostrar el banner [AIOps]
	renderedCrashed := m.View()
	if !strings.Contains(renderedCrashed, "[AIOps]") {
		t.Errorf("expected rendered view to contain [AIOps] tag, got:\n%s", renderedCrashed)
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
	if !strings.Contains(rendered, "METRICAS EN TIEMPO REAL:") || !strings.Contains(rendered, "CPU:") || !strings.Contains(rendered, "RAM:") {
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
	savedURL      string
	savedToken    string
	savedKey      string
	savedAIConfig domain.AIConfig
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

func TestTUI_ConfigModalWorkflow(t *testing.T) {
	collector := &mockCollectorForTUI{metrics: sampleMetrics()}
	vaultMock := &mockVaultForTUI{}
	model := NewModel(collector, vaultMock)
	model.metrics = sampleMetrics()

	// 1. Abrir modal con 'c' -> configViewSelectModel (OpenCode style)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m := newModel.(Model)
	if m.activeState != stateConfigModal {
		t.Fatalf("expected activeState to be stateConfigModal, got %v", m.activeState)
	}
	if m.configMode != configViewSelectModel {
		t.Fatalf("expected configMode to be configViewSelectModel, got %v", m.configMode)
	}

	// 2. Renderizar vista estilo OpenCode
	renderedModal := m.View()
	if !strings.Contains(renderedModal, "Select model") || !strings.Contains(renderedModal, "Search") {
		t.Errorf("expected view to contain OpenCode header 'Select model' and 'Search', got:\n%s", renderedModal)
	}
	if !strings.Contains(renderedModal, "Ctrl+A: Connect provider") {
		t.Errorf("expected view to contain shortcut guide for Ctrl+A, got:\n%s", renderedModal)
	}

	// 3. Presionar Ctrl+A para conectar credencial de proveedor
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = newModel.(Model)
	if m.configMode != configViewConnectKey {
		t.Fatalf("expected configMode to be configViewConnectKey after Ctrl+A, got %v", m.configMode)
	}

	// 4. Escribir clave de Anthropic
	for _, r := range "sk-ant-testkey999" {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(Model)
	}

	// 5. Presionar Enter para guardar clave en la bóveda
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if m.configMode != configViewSelectModel {
		t.Errorf("expected to return to configViewSelectModel after saving key, got %v", m.configMode)
	}
	antCfg := vaultMock.savedAIConfig.Providers[m.connectProvider]
	if antCfg.APIKey != "sk-ant-testkey999" {
		t.Errorf("expected vault to store provider key, got %s", antCfg.APIKey)
	}

	// 6. Salir del modal con Esc
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



