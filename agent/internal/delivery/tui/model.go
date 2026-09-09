package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/usecases"
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
	diagnosisCache     map[string]string
	lastDiagnosisUsage map[string]domain.TokenUsage
	triagePending      map[string]bool
	metricsHistory     map[string]*MetricHistory
	metrics            []domain.ContainerMetric
	cursor             int
	activeState        sessionState
	filterInput        textinput.Model
	filterValue        string

	// Estado del Modal de IA estilo OpenCode
	configMode         configViewMode
	modelSearchInput   textinput.Model
	apiKeyInput        textinput.Model
	endpointInput      textinput.Model
	customModelInput   textinput.Model
	connectFocusField  int // 0: Key, 1: Endpoint
	modelListCursor    int
	connectProvider    domain.AIProvider
	aiConfig           domain.AIConfig

	// Métricas de consumo AIOps
	sessionTokensUsed int
	sessionCostUSD    float64

	// Estado del Selector de Temas y Tipografía
	themeConfig     domain.ThemeConfig
	themeListCursor int

	// Contenedores fijados (Pinning prioritario)
	pinnedContainers map[string]bool

	viewport         viewport.Model
	selectedName     string
	selectedID       string
	selectedState    string
	pendingAction    domain.ActionType
	pendingContainer domain.ContainerMetric
	confirmModalBtn  int // 0 = Confirmar, 1 = Cancelar
	statusMessage    string
	statusExpiry     time.Time
	lastError        string
	lastSync         time.Time
	width            int
	height           int
}

// NewModel inicializa el modelo de la TUI con soporte de bóveda para AIOps.
func NewModel(collector ports.CollectorPort, v ...ports.VaultPort) Model {
	ti := textinput.New()
	ti.Placeholder = "filtrar por nombre o imagen..."
	ti.Prompt = "/ "
	ti.PromptStyle = StyleFilterPrompt

	// Input para búsqueda de modelos OpenCode style
	ms := textinput.New()
	ms.Placeholder = "Buscar modelo o proveedor..."
	ms.Prompt = "Search "
	ms.PromptStyle = lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Bold(true)
	ms.TextStyle = lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0)
	ms.PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorSurface2).Background(ColorSurface0)

	// Input para clave de API
	ki := textinput.New()
	ki.Placeholder = "sk-... o clave del proveedor"
	ki.Prompt = "API Key: "
	ki.PromptStyle = lipgloss.NewStyle().Foreground(ColorLavender).Background(ColorSurface0).Bold(true)
	ki.TextStyle = lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0)
	ki.PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorSurface2).Background(ColorSurface0)
	ki.EchoMode = textinput.EchoPassword
	ki.EchoCharacter = '•'

	// Input para Base URL / Endpoint
	ei := textinput.New()
	ei.Placeholder = "https://api... o http://localhost:11434"
	ei.Prompt = "Base URL: "
	ei.PromptStyle = lipgloss.NewStyle().Foreground(ColorLavender).Background(ColorSurface0).Bold(true)
	ei.TextStyle = lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0)
	ei.PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorSurface2).Background(ColorSurface0)

	// Input para modelo custom
	ci := textinput.New()
	ci.Placeholder = "ej. deepseek/deepseek-r1 o claude-3-7-sonnet"
	ci.Prompt = "Model ID: "
	ci.PromptStyle = lipgloss.NewStyle().Foreground(ColorLavender).Background(ColorSurface0).Bold(true)
	ci.TextStyle = lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0)
	ci.PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorSurface2).Background(ColorSurface0)

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

	ApplyTheme(themeCfg.ActiveTheme, themeCfg.BorderStyle, themeCfg.NerdFonts)

	var triageClient TriageService
	if aiCfg.ActiveProvider != "" {
		triageClient = ai.NewTriageClientWithConfig(aiCfg)
	} else {
		triageClient = ai.NewTriageClient()
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

	return Model{
		collector:          collector,
		vaultService:       vaultSvc,
		triageClient:       triageClient,
		diagnosisCache:     make(map[string]string),
		lastDiagnosisUsage: make(map[string]domain.TokenUsage),
		triagePending:      make(map[string]bool),
		metricsHistory:     make(map[string]*MetricHistory),
		pinnedContainers:   pinnedMap,
		cursor:             0,
		activeState:        stateFleetTable,
		filterInput:        ti,
		modelSearchInput:   ms,
		apiKeyInput:        ki,
		endpointInput:      ei,
		customModelInput:   ci,
		aiConfig:           aiCfg,
		themeConfig:        themeCfg,
		themeListCursor:    themeCursor,
		configMode:         configViewSelectModel,
		connectProvider:    aiCfg.ActiveProvider,
		viewport:           vp,
		lastSync:           time.Now(),
		width:              100,
		height:             24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchMetrics(), tickCmd())
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
	if m.triageClient == nil || !m.isAnomalous(c) {
		return nil
	}
	if _, cached := m.diagnosisCache[c.ID]; cached {
		return nil
	}
	if m.triagePending[c.ID] {
		return nil
	}
	m.triagePending[c.ID] = true

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()

		uc := usecases.NewDiagnoseContainerUseCase(m.collector, m.triageClient)
		diag, usage, _ := uc.Execute(ctx, c)

		return diagnosisResultMsg{
			containerID: c.ID,
			diagnosis:   diag,
			usage:       usage,
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

// RunTUI inicia el programa interactivo Bubbletea con soporte de mouse.
func RunTUI(collector ports.CollectorPort, v ...ports.VaultPort) error {
	p := tea.NewProgram(
		NewModel(collector, v...),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
