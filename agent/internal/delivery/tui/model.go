package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/service"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/usecases"
	"github.com/alvaroriverac/server_tracker_agent/internal/i18n"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/ai"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/vault"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model representa el estado global de la TUI interactiva.
type Model struct {
	collector          ports.CollectorPort
	vaultService       ports.VaultPort
	triageClient       TriageService
	ruleEngine         *service.RuleEngine
	crashJournal       *service.CrashJournal
	reExecutedEvents   map[string]bool
	diagnosisResults   map[string]domain.DiagnosisResult
	diagnosisCache     map[string]string
	lastDiagnosisUsage map[string]domain.TokenUsage
	triagePending      map[string]bool
	metricsHistory     map[string]*MetricHistory
	metrics            []domain.ContainerMetric
	cursor             int
	activeState        sessionState
	filterInput        textinput.Model
	filterValue        string

	// Estado del Subsistema de IA: Grafo de 3 vistas y Discovery
	catalogService       *ai.CatalogService
	aiState              aiViewState
	aiPolicyCursor       int // 0: FAST, 1: DEEP, 2: AUTO
	aiTargetSlot         domain.DiagnosisSlot
	aiBrowserCursor      int
	aiFilterFree         bool
	aiFilterLocal        bool
	aiSearchActive       bool
	aiSearchInput        textinput.Model
	aiProvidersCursor    int
	connectProvider      domain.AIProvider
	apiKeyInput          textinput.Model
	endpointInput        textinput.Model
	connectFocusField    int // 0: URL, 1: Key
	aiConfig             domain.AIConfig
	discoveredConnectors map[domain.AIProvider]ai.DiscoveryResult
	discoveredModels     []domain.ModelRef

	// Métricas de consumo AIOps
	sessionTokensUsed int
	sessionCostUSD    float64
	aiMeter           *service.AIMeter

	// Estado del Selector de Temas y Tipografía
	themeConfig     domain.ThemeConfig
	themeListCursor int

	// Contenedores fijados (Pinning prioritario)
	pinnedContainers map[string]bool

	// Análisis de Incidentes Correlacionados (Ola 6)
	incidentAggregator *service.IncidentAggregator
	activeIncident     *service.Incident
	v4IncidentCursor   int // Índice de contenedor enfocado en lista de miembros de incidente
	preferencesCursor  int // 0: Temas y estilos, 1: Origen en banner

	viewport         viewport.Model
	selectedName     string
	selectedID       string
	selectedState    string
	pendingAction    domain.ActionType
	pendingContainer domain.ContainerMetric
	confirmModalBtn  int // 0 = Confirmar, 1 = Cancelar
	v4ActionCursor   int // Índice de acción seleccionada en modal V4 diagnóstico
	statusMessage    string
	statusExpiry     time.Time
	lastError        string
	lastSync         time.Time
	width            int
	height           int

	// Responsividad y Toast de Primera Ejecución (Ola 7)
	overlayScrollOffset int
	toastVisible        bool
	toastExpiry         time.Time
	v4FocusSection      int 
	v4EvidenceScroll    int 
	language            i18n.Language
	langOverlayCursor   int // 0: español, 1: english
}

// NewModel inicializa el modelo de la TUI con soporte de bóveda para AIOps.
func NewModel(collector ports.CollectorPort, v ...ports.VaultPort) Model {
	ti := textinput.New()
	ti.Placeholder = "filtrar por nombre o imagen..."
	ti.Prompt = "/ "
	ti.PromptStyle = StyleFilterPrompt

	// Input para búsqueda rápida en el navegador de modelos (V2)
	ms := textinput.New()
	ms.Placeholder = "buscar modelo..."
	ms.Prompt = "/ "
	ms.PromptStyle = lipgloss.NewStyle().Foreground(ColorPeach).Bold(true)
	ms.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ms.PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorSurface2)

	// Input para clave de API (V3 sub-paso)
	ki := textinput.New()
	ki.Placeholder = "pegar API key..."
	ki.Prompt = "Key: "
	ki.PromptStyle = lipgloss.NewStyle().Foreground(ColorLavender).Bold(true)
	ki.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ki.PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorSurface2)
	ki.EchoMode = textinput.EchoPassword
	ki.EchoCharacter = '•'

	// Input para Base URL / Endpoint (V3 sub-paso custom)
	ei := textinput.New()
	ei.Placeholder = "http://localhost:8080/v1"
	ei.Prompt = "URL: "
	ei.PromptStyle = lipgloss.NewStyle().Foreground(ColorLavender).Bold(true)
	ei.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ei.PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorSurface2)

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

	var vaultSvc ports.VaultPort
	if len(v) > 0 && v[0] != nil {
		vaultSvc = v[0]
	} else {
		homeDir, _ := os.UserHomeDir()
		vaultPath := filepath.Join(homeDir, ".solv", "vault.enc")
		passphrase := os.Getenv("SOLV_VAULT_PASSPHRASE")
		if passphrase == "" {
			passphrase = "solv_default_host_entropy"
		}
		vaultSvc = vault.NewCascadeVault(vaultPath, passphrase)
	}

	var aiCfg domain.AIConfig
	var themeCfg domain.ThemeConfig
	if vaultSvc != nil {
		if c, err := vaultSvc.GetAIConfig(); err == nil {
			aiCfg = c
		} else {
			aiCfg = domain.DefaultAIConfig()
		}
		if t, err := vaultSvc.GetThemeConfig(); err == nil {
			themeCfg = t
		} else {
			themeCfg = domain.DefaultThemeConfig()
		}
	} else {
		aiCfg = domain.DefaultAIConfig()
		themeCfg = domain.DefaultThemeConfig()
	}

	homeDir, _ := os.UserHomeDir()
	catCachePath := filepath.Join(homeDir, ".solv", "catalog_cache.json")
	catSvc := ai.NewCatalogService(catCachePath)

	var triageClient TriageService
	if aiCfg.ActiveProvider != "" {
		triageClient = ai.NewTriageClientWithConfig(aiCfg)
	} else {
		triageClient = ai.NewTriageClient()
	}
	journal := service.NewCrashJournal()
	if tc, ok := triageClient.(*ai.TriageClient); ok {
		tc.SetCatalogService(catSvc)
		tc.SetCrashJournal(journal)
	}

	if themeCfg.IncidentBannerPolicy == "" {
		themeCfg.IncidentBannerPolicy = "informativo"
	}
	if aiCfg.IncidentWindowSeconds <= 0 {
		aiCfg.IncidentWindowSeconds = 30
	}

	themeCursor := 0
	for idx, th := range domain.AvailableThemes {
		if th.ID == themeCfg.ActiveTheme {
			themeCursor = idx
			break
		}
	}

	pinnedMap := make(map[string]bool)
	if vaultSvc != nil {
		if pinned, err := vaultSvc.GetPinnedContainers(); err == nil && pinned != nil {
			for _, name := range pinned {
				if name != "" {
					pinnedMap[name] = true
				}
			}
		}
	}

	ruleEng := service.NewRuleEngine()

	activeLang := i18n.LangES
	firstRunLang := false
	suggestedCursor := 0 // 0: español, 1: english

	envLang := strings.ToLower(os.Getenv("LANG"))
	if strings.Contains(envLang, "en") {
		suggestedCursor = 1
	}

	if vaultSvc != nil {
		if storedLang, err := vaultSvc.GetLanguage(); err == nil && storedLang != "" {
			activeLang = i18n.NormalizeLanguage(storedLang)
		} else {
			firstRunLang = true
		}
	}

	initState := stateFleetTable
	if firstRunLang {
		initState = stateLanguageOverlay
	}

	if triageClient != nil {
		triageClient.SetLanguage(string(activeLang))
	}

	return Model{
		collector:            collector,
		vaultService:         vaultSvc,
		triageClient:         triageClient,
		ruleEngine:           ruleEng,
		crashJournal:         journal,
		reExecutedEvents:     make(map[string]bool),
		catalogService:       catSvc,
		diagnosisResults:     make(map[string]domain.DiagnosisResult),
		diagnosisCache:       make(map[string]string),
		lastDiagnosisUsage:   make(map[string]domain.TokenUsage),
		triagePending:        make(map[string]bool),
		metricsHistory:       make(map[string]*MetricHistory),
		pinnedContainers:     pinnedMap,
		cursor:               0,
		activeState:          initState,
		filterInput:          ti,
		aiState:              aiViewPolicy,
		aiPolicyCursor:       0,
		aiTargetSlot:         domain.SlotFast,
		aiBrowserCursor:      0,
		aiSearchInput:        ms,
		apiKeyInput:          ki,
		endpointInput:        ei,
		aiConfig:             aiCfg,
		themeConfig:          themeCfg,
		themeListCursor:      themeCursor,
		connectProvider:      aiCfg.ActiveProvider,
		discoveredConnectors: make(map[domain.AIProvider]ai.DiscoveryResult),
		discoveredModels:     make([]domain.ModelRef, 0),
		viewport:             vp,
		aiMeter:              service.NewAIMeter(),
		incidentAggregator:   service.NewIncidentAggregator(aiCfg.IncidentWindowSeconds),
		lastSync:             time.Now(),
		width:                100,
		height:               24,
		overlayScrollOffset:  0,
		toastVisible:         !firstRunLang && !themeCfg.OnboardingHintShown,
		toastExpiry:          time.Now().Add(5 * time.Second),
		language:             activeLang,
		langOverlayCursor:    suggestedCursor,
	}
}

type catalogSyncedMsg struct{}
type discoveryMsg struct {
	results map[domain.AIProvider]ai.DiscoveryResult
}

func (m Model) probeConnectorsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		connectors := ai.DefaultConnectors()
		results := ai.ProbeConnectors(ctx, connectors, m.aiConfig.Providers, nil)
		return discoveryMsg{results: results}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchMetrics(), m.syncCatalog(), m.probeConnectorsCmd(), tickCmd())
}

func (m Model) syncCatalog() tea.Cmd {
	return func() tea.Msg {
		if m.catalogService == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.catalogService.SyncIfExpired(ctx, 1*time.Hour, m.aiConfig.Providers)
		return catalogSyncedMsg{}
	}
}

func (m Model) fetchMetrics() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		uc := usecases.NewCollectTelemetryUseCase(m.collector)
		metrics, err := uc.Execute(ctx)
		if err != nil {
			return err
		}
		return metrics
	}
}

func (m Model) fetchLogs(containerID, containerName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		logs, err := m.collector.GetContainerLogs(ctx, containerID, 150)
		return logsMsg{
			containerName: containerName,
			content:       logs,
			err:           err,
		}
	}
}

func (m Model) isPinned(name string) bool {
	if m.pinnedContainers == nil {
		return false
	}
	return m.pinnedContainers[name]
}

func (m *Model) togglePin(name string) {
	if m.pinnedContainers == nil {
		m.pinnedContainers = make(map[string]bool)
	}
	if m.pinnedContainers[name] {
		delete(m.pinnedContainers, name)
	} else {
		m.pinnedContainers[name] = true
	}
	m.savePinnedToVault()
}

func (m *Model) clearAllPins() {
	m.pinnedContainers = make(map[string]bool)
	m.savePinnedToVault()
}

func (m *Model) savePinnedToVault() {
	if m.vaultService == nil {
		return
	}
	var names []string
	for n := range m.pinnedContainers {
		names = append(names, n)
	}
	_ = m.vaultService.SavePinnedContainers(names)
}

func (m Model) filteredMetrics() []domain.ContainerMetric {
	var candidates []domain.ContainerMetric
	if m.filterValue == "" {
		candidates = m.metrics
	} else {
		query := strings.ToLower(m.filterValue)
		for _, c := range m.metrics {
			if strings.Contains(strings.ToLower(c.Name), query) || strings.Contains(strings.ToLower(c.Image), query) {
				candidates = append(candidates, c)
			}
		}
	}

	if len(m.pinnedContainers) == 0 {
		return candidates
	}

	var pinned []domain.ContainerMetric
	var unpinned []domain.ContainerMetric

	for _, c := range candidates {
		if m.pinnedContainers[c.Name] {
			pinned = append(pinned, c)
		} else {
			unpinned = append(unpinned, c)
		}
	}

	return append(pinned, unpinned...)
}

func (m Model) isAnomalous(c domain.ContainerMetric) bool {
	if strings.ToLower(c.Status) != "running" {
		return true
	}
	if c.RAMLimitBytes > 0 && float64(c.RAMBytes)/float64(c.RAMLimitBytes) >= 0.85 {
		return true
	}
	return false
}

func (m Model) triggerTriageIfAnomalous(c domain.ContainerMetric) tea.Cmd {
	return m.triggerTriage(c, false)
}

func (m Model) triggerTriageForced(c domain.ContainerMetric) tea.Cmd {
	return m.triggerTriage(c, true)
}

func (m Model) triggerTriage(c domain.ContainerMetric, forceAI bool) tea.Cmd {
	if !m.isAnomalous(c) {
		return nil
	}

	// En modo MANUAL sin forzar IA: evaluar reglas locales inmediatamente (coste 0, sincrónico)
	if m.aiConfig.SelectionMode == domain.SelectionManual && !forceAI {
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			uc := usecases.NewDiagnoseContainerUseCase(m.collector, m.triageClient, m.ruleEngine)
			uc.SetCrashJournal(m.crashJournal)
			uc.SetReExecutedMap(m.reExecutedEvents)
			res := uc.ExecuteWithCascade(ctx, c, false, domain.SelectionManual)

			return diagnosisResultMsg{
				containerID: c.ID,
				diagnosis:   res.RootCause,
				usage:       res.TokenUsage,
				result:      res,
			}
		}
	}

	// Si se consulta IA y ya está en caché, no repetir llamada a red
	if _, cached := m.diagnosisCache[c.ID]; cached && !forceAI {
		return nil
	}
	if m.triagePending[c.ID] {
		return nil
	}
	m.triagePending[c.ID] = true

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()

		uc := usecases.NewDiagnoseContainerUseCase(m.collector, m.triageClient, m.ruleEngine)
		uc.SetCrashJournal(m.crashJournal)
		uc.SetReExecutedMap(m.reExecutedEvents)
		res := uc.ExecuteWithCascade(ctx, c, forceAI, m.aiConfig.SelectionMode)

		return diagnosisResultMsg{
			containerID: c.ID,
			diagnosis:   res.RootCause,
			usage:       res.TokenUsage,
			result:      res,
		}
	}
}

func (m Model) executeRemediation(c domain.ContainerMetric, action domain.ActionType) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := domain.RemediationCommand{
			ContainerID: c.ID,
			Action:      action,
			Timestamp:   time.Now().Unix(),
		}
		uc := usecases.NewRemediateContainerUseCase(m.collector)
		err := uc.Execute(ctx, cmd)

		return remediationResultMsg{
			action:        action,
			containerName: c.Name,
			elapsed:       time.Since(start),
			err:           err,
		}
	}
}

// RunTUI inicia el programa interactivo Bubbletea con blindaje de TTY y trap de pánico (F2).
func RunTUI(collector ports.CollectorPort, v ...ports.VaultPort) (err error) {
	defer func() {
		if r := recover(); r != nil {
			HandlePanic(r)
		}
	}()

	p := tea.NewProgram(
		NewModel(collector, v...),
		tea.WithAltScreen(),
	)
	_, err = p.Run()
	RestoreTTY()
	return err
}
