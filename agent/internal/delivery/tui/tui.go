package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/ai"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/vault"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type sessionState int

const (
	stateFleetTable sessionState = iota
	stateLogViewer
	stateFiltering
	stateConfirmRemediation
	stateConfigModal
	stateHelp
)

type tickMsg time.Time
type logsMsg struct {
	containerName string
	content       string
	err           error
}

type remediationResultMsg struct {
	action        domain.ActionType
	containerName string
	elapsed       time.Duration
	err           error
}

type diagnosisResultMsg struct {
	containerID string
	diagnosis   string
	usage       domain.TokenUsage
}

type shellFinishedMsg struct {
	err error
}

type configViewMode int

const (
	configViewSelectModel configViewMode = iota
	configViewConnectKey
	configViewCustomModel
)

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// TriageService define el contrato para el análisis pasivo con IA y reporte de tokens.
type TriageService interface {
	DiagnoseContainer(ctx context.Context, name, image, status, logs string) string
	DiagnoseContainerWithUsage(ctx context.Context, name, image, status, logs string) (string, domain.TokenUsage)
}

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
	if vaultSvc != nil {
		if c, err := vaultSvc.GetAIConfig(); err == nil {
			aiCfg = c
		} else {
			aiCfg = domain.DefaultAIConfig()
		}
	} else {
		aiCfg = domain.DefaultAIConfig()
	}

	var triageClient TriageService
	if aiCfg.ActiveProvider != "" {
		triageClient = ai.NewTriageClientWithConfig(aiCfg)
	} else {
		triageClient = ai.NewTriageClient()
	}

	return Model{
		collector:          collector,
		vaultService:       vaultSvc,
		triageClient:       triageClient,
		diagnosisCache:     make(map[string]string),
		lastDiagnosisUsage: make(map[string]domain.TokenUsage),
		triagePending:      make(map[string]bool),
		metricsHistory:     make(map[string]*MetricHistory),
		cursor:             0,
		activeState:        stateFleetTable,
		filterInput:        ti,
		modelSearchInput:   ms,
		apiKeyInput:        ki,
		endpointInput:      ei,
		customModelInput:   ci,
		aiConfig:           aiCfg,
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

		metrics, err := m.collector.Collect(ctx)
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

func (m Model) filteredMetrics() []domain.ContainerMetric {
	if m.filterValue == "" {
		return m.metrics
	}
	var filtered []domain.ContainerMetric
	query := strings.ToLower(m.filterValue)
	for _, c := range m.metrics {
		if strings.Contains(strings.ToLower(c.Name), query) || strings.Contains(strings.ToLower(c.Image), query) {
			filtered = append(filtered, c)
		}
	}
	return filtered
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

		logs, _ := m.collector.GetContainerLogs(ctx, c.ID, 50)
		diag, usage := m.triageClient.DiagnoseContainerWithUsage(ctx, c.Name, c.Image, c.Status, logs)
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
		err := m.collector.ExecuteRemediation(ctx, cmd)
		return remediationResultMsg{
			action:        action,
			containerName: c.Name,
			elapsed:       time.Since(start),
			err:           err,
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = max(20, msg.Width-6)
		m.viewport.Height = max(5, msg.Height-8)

	case tea.MouseMsg:
		filtered := m.filteredMetrics()
		switch msg.Type {
		case tea.MouseLeft:
			if m.activeState == stateFleetTable || m.activeState == stateFiltering {
				headerOffset := 3
				if m.lastError != "" {
					headerOffset = 4
				}
				clickedRow := msg.Y - headerOffset
				if clickedRow >= 0 && clickedRow < len(filtered) {
					m.cursor = clickedRow
					if cmd := m.triggerTriageIfAnomalous(filtered[m.cursor]); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}
		case tea.MouseWheelDown:
			if m.activeState == stateFleetTable || m.activeState == stateFiltering {
				if len(filtered) > 0 && m.cursor < len(filtered)-1 {
					m.cursor++
					if cmd := m.triggerTriageIfAnomalous(filtered[m.cursor]); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			} else if m.activeState == stateLogViewer {
				m.viewport.LineDown(1)
			}
		case tea.MouseWheelUp:
			if m.activeState == stateFleetTable || m.activeState == stateFiltering {
				if len(filtered) > 0 && m.cursor > 0 {
					m.cursor--
					if cmd := m.triggerTriageIfAnomalous(filtered[m.cursor]); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			} else if m.activeState == stateLogViewer {
				m.viewport.LineUp(1)
			}
		}

	case tea.KeyMsg:
		switch m.activeState {
		case stateFiltering:
			switch msg.String() {
			case "enter", "esc":
				m.filterValue = m.filterInput.Value()
				m.filterInput.Blur()
				m.activeState = stateFleetTable
				m.cursor = 0
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.filterValue = m.filterInput.Value()
				m.cursor = 0
				return m, cmd
			}

		case stateLogViewer:
			switch msg.String() {
			case "esc", "h", "q":
				m.activeState = stateFleetTable
			default:
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}

		case stateHelp:
			if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" {
				m.activeState = stateFleetTable
			}

		case stateConfirmRemediation:
			switch msg.String() {
			case "left", "right", "tab", "shift+tab", "h", "l":
				m.confirmModalBtn = 1 - m.confirmModalBtn
			case "enter":
				if m.confirmModalBtn == 0 {
					cmd := m.executeRemediation(m.pendingContainer, m.pendingAction)
					m.statusMessage = fmt.Sprintf("[..] Ejecutando %s en '%s'...", m.pendingAction, m.pendingContainer.Name)
					m.statusExpiry = time.Now().Add(10 * time.Second)
					m.activeState = stateFleetTable
					return m, cmd
				} else {
					m.statusMessage = "[--] Acción cancelada por el usuario"
					m.statusExpiry = time.Now().Add(3 * time.Second)
					m.activeState = stateFleetTable
				}
			case "y", "Y":
				cmd := m.executeRemediation(m.pendingContainer, m.pendingAction)
				m.statusMessage = fmt.Sprintf("[..] Ejecutando %s en '%s'...", m.pendingAction, m.pendingContainer.Name)
				m.statusExpiry = time.Now().Add(10 * time.Second)
				m.activeState = stateFleetTable
				return m, cmd
			case "n", "N", "esc", "q":
				m.statusMessage = "[--] Acción cancelada por el usuario"
				m.statusExpiry = time.Now().Add(3 * time.Second)
				m.activeState = stateFleetTable
			}

		case stateConfigModal:
			items := m.getFilteredModelItems()
			switch m.configMode {
			case configViewSelectModel:
				switch msg.String() {
				case "esc", "ctrl+c":
					m.modelSearchInput.Blur()
					m.activeState = stateFleetTable
					return m, nil
				case "up", "ctrl+p", "ctrl+k":
					if len(items) > 0 {
						newCursor := m.modelListCursor - 1
						for newCursor >= 0 && items[newCursor].isHeader {
							newCursor--
						}
						if newCursor >= 0 {
							m.modelListCursor = newCursor
						}
					}
					return m, nil
				case "down", "ctrl+n", "ctrl+j":
					if len(items) > 0 {
						newCursor := m.modelListCursor + 1
						for newCursor < len(items) && items[newCursor].isHeader {
							newCursor++
						}
						if newCursor < len(items) {
							m.modelListCursor = newCursor
						}
					}
					return m, nil
				case "ctrl+a":
					// Conectar / editar API Key del ítem seleccionado o activo
					selectedProv := m.aiConfig.ActiveProvider
					if m.modelListCursor >= 0 && m.modelListCursor < len(items) {
						it := items[m.modelListCursor]
						if !it.isHeader && !it.isCustom {
							selectedProv = it.option.Provider
						}
					}
					m.connectProvider = selectedProv
					m.configMode = configViewConnectKey
					m.connectFocusField = 0
					pCfg := m.aiConfig.Providers[selectedProv]
					m.apiKeyInput.Reset()
					m.apiKeyInput.SetValue(pCfg.APIKey)
					m.endpointInput.Reset()
					m.endpointInput.SetValue(pCfg.Endpoint)
					m.apiKeyInput.Focus()
					m.endpointInput.Blur()
					return m, textinput.Blink
				case "enter":
					if m.modelListCursor >= 0 && m.modelListCursor < len(items) {
						it := items[m.modelListCursor]
						if it.isHeader {
							return m, nil
						}
						if it.isCustom {
							m.configMode = configViewCustomModel
							m.customModelInput.Reset()
							m.customModelInput.SetValue(m.aiConfig.ActiveModel)
							m.customModelInput.Focus()
							return m, textinput.Blink
						}
						// Modelo de catálogo
						prov := it.option.Provider
						pCfg := m.aiConfig.Providers[prov]
						meta := domain.GetProviderMeta(prov)

						// Si requiere key y no tiene, pasar directo a conectar key
						if meta.RequiresKey && strings.TrimSpace(pCfg.APIKey) == "" {
							m.connectProvider = prov
							m.configMode = configViewConnectKey
							m.connectFocusField = 0
							m.apiKeyInput.Reset()
							m.endpointInput.Reset()
							m.endpointInput.SetValue(pCfg.Endpoint)
							m.apiKeyInput.Focus()
							m.endpointInput.Blur()
							m.statusMessage = fmt.Sprintf("[INFO] %s requiere clave API para activarse", meta.Name)
							m.statusExpiry = time.Now().Add(3 * time.Second)
							return m, textinput.Blink
						}

						// Activar modelo y proveedor
						m.aiConfig.ActiveProvider = prov
						m.aiConfig.ActiveModel = it.option.ID
						pCfg.DefaultModel = it.option.ID
						m.aiConfig.Providers[prov] = pCfg
						if m.vaultService != nil {
							_ = m.vaultService.SaveAIConfig(m.aiConfig)
						}
						m.triageClient = ai.NewTriageClientWithConfig(m.aiConfig)
						m.diagnosisCache = make(map[string]string)
						m.statusMessage = fmt.Sprintf("[OK] Modelo activo: %s (%s)", it.option.DisplayName, meta.Name)
						m.statusExpiry = time.Now().Add(4 * time.Second)
						m.activeState = stateFleetTable
						return m, nil
					}
				default:
					var cmd tea.Cmd
					m.modelSearchInput, cmd = m.modelSearchInput.Update(msg)
					// Reajustar cursor al primer ítem seleccionable si la lista cambia
					newItems := m.getFilteredModelItems()
					if m.modelListCursor >= len(newItems) {
						m.modelListCursor = 0
					}
					for m.modelListCursor < len(newItems) && newItems[m.modelListCursor].isHeader {
						m.modelListCursor++
					}
					return m, cmd
				}

			case configViewConnectKey:
				switch msg.String() {
				case "esc":
					m.apiKeyInput.Blur()
					m.endpointInput.Blur()
					m.configMode = configViewSelectModel
					m.modelSearchInput.Focus()
					return m, textinput.Blink
				case "tab", "down":
					m.connectFocusField = (m.connectFocusField + 1) % 2
					if m.connectFocusField == 0 {
						m.apiKeyInput.Focus()
						m.endpointInput.Blur()
					} else {
						m.apiKeyInput.Blur()
						m.endpointInput.Focus()
					}
					return m, textinput.Blink
				case "shift+tab", "up":
					m.connectFocusField = (m.connectFocusField + 1) % 2
					if m.connectFocusField == 0 {
						m.apiKeyInput.Focus()
						m.endpointInput.Blur()
					} else {
						m.apiKeyInput.Blur()
						m.endpointInput.Focus()
					}
					return m, textinput.Blink
				case "enter":
					valKey := strings.TrimSpace(m.apiKeyInput.Value())
					valEP := strings.TrimSpace(m.endpointInput.Value())
					prov := m.connectProvider
					meta := domain.GetProviderMeta(prov)
					pCfg := m.aiConfig.Providers[prov]
					pCfg.APIKey = valKey
					pCfg.Endpoint = valEP
					if pCfg.DefaultModel == "" {
						pCfg.DefaultModel = meta.DefaultModel
					}
					m.aiConfig.Providers[prov] = pCfg
					m.aiConfig.ActiveProvider = prov
					m.aiConfig.ActiveModel = pCfg.DefaultModel

					if m.vaultService != nil {
						if err := m.vaultService.SaveAIConfig(m.aiConfig); err != nil {
							m.statusMessage = fmt.Sprintf("[!!] Error guardando en bóveda: %v", err)
						} else {
							m.statusMessage = fmt.Sprintf("[OK] Clave de %s cifrada bajo Blindaje D2 (AES-256-GCM)", meta.Name)
						}
					}
					m.triageClient = ai.NewTriageClientWithConfig(m.aiConfig)
					m.diagnosisCache = make(map[string]string)
					m.statusExpiry = time.Now().Add(4 * time.Second)
					m.apiKeyInput.Blur()
					m.endpointInput.Blur()
					m.configMode = configViewSelectModel
					m.modelSearchInput.Focus()
					return m, textinput.Blink
				default:
					var cmd tea.Cmd
					if m.connectFocusField == 0 {
						m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
					} else {
						m.endpointInput, cmd = m.endpointInput.Update(msg)
					}
					return m, cmd
				}

			case configViewCustomModel:
				switch msg.String() {
				case "esc":
					m.customModelInput.Blur()
					m.configMode = configViewSelectModel
					m.modelSearchInput.Focus()
					return m, textinput.Blink
				case "enter":
					customVal := strings.TrimSpace(m.customModelInput.Value())
					if customVal != "" {
						m.aiConfig.ActiveModel = customVal
						prov := m.aiConfig.ActiveProvider
						pCfg := m.aiConfig.Providers[prov]
						pCfg.DefaultModel = customVal
						m.aiConfig.Providers[prov] = pCfg
						if m.vaultService != nil {
							_ = m.vaultService.SaveAIConfig(m.aiConfig)
						}
						m.triageClient = ai.NewTriageClientWithConfig(m.aiConfig)
						m.diagnosisCache = make(map[string]string)
						m.statusMessage = fmt.Sprintf("[OK] Modelo personalizado activo: %s", customVal)
						m.statusExpiry = time.Now().Add(4 * time.Second)
						m.activeState = stateFleetTable
						return m, nil
					}
					m.configMode = configViewSelectModel
					m.modelSearchInput.Focus()
					return m, textinput.Blink
				default:
					var cmd tea.Cmd
					m.customModelInput, cmd = m.customModelInput.Update(msg)
					return m, cmd
				}
			}

		case stateFleetTable:
			filtered := m.filteredMetrics()
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "j", "down":
				if len(filtered) > 0 {
					m.cursor = (m.cursor + 1) % len(filtered)
					if cmd := m.triggerTriageIfAnomalous(filtered[m.cursor]); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			case "k", "up":
				if len(filtered) > 0 {
					m.cursor = (m.cursor - 1 + len(filtered)) % len(filtered)
					if cmd := m.triggerTriageIfAnomalous(filtered[m.cursor]); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			case "/":
				m.activeState = stateFiltering
				m.filterInput.Focus()
				return m, textinput.Blink
			case "l", "enter":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					c := filtered[m.cursor]
					m.selectedID = c.ID
					m.selectedName = c.Name
					m.selectedState = c.Status
					m.activeState = stateLogViewer
					m.viewport.SetContent("Cargando logs de Docker...")
					return m, m.fetchLogs(c.ID, c.Name)
				}
			case "e":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					c := filtered[m.cursor]
					if strings.ToLower(c.Status) != "running" {
						m.statusMessage = fmt.Sprintf("[!!] No se puede abrir shell: '%s' no está activo (%s)", c.Name, c.Status)
						m.statusExpiry = time.Now().Add(4 * time.Second)
						return m, nil
					}
					shellCmd := exec.Command("docker", "exec", "-it", c.ID, "/bin/sh")
					return m, tea.ExecProcess(shellCmd, func(err error) tea.Msg {
						return shellFinishedMsg{err: err}
					})
				}
			case "c":
				m.activeState = stateConfigModal
				m.configMode = configViewSelectModel
				m.modelSearchInput.Reset()
				m.modelSearchInput.Focus()
				m.modelListCursor = 0
				items := m.getFilteredModelItems()
				for idx, it := range items {
					if it.isActive {
						m.modelListCursor = idx
						break
					}
				}
				return m, textinput.Blink

			case "r":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					m.pendingContainer = filtered[m.cursor]
					m.pendingAction = domain.ActionRestart
					m.confirmModalBtn = 0
					m.activeState = stateConfirmRemediation
				}
			case "s":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					m.pendingContainer = filtered[m.cursor]
					m.pendingAction = domain.ActionStop
					m.confirmModalBtn = 0
					m.activeState = stateConfirmRemediation
				}
			case "x":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					m.pendingContainer = filtered[m.cursor]
					m.pendingAction = domain.ActionIsolateNetwork
					m.confirmModalBtn = 0
					m.activeState = stateConfirmRemediation
				}
			case "?":
				m.activeState = stateHelp
			}
		}

	case shellFinishedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("[!!] Shell finalizada con error: %v", msg.err)
		} else {
			m.statusMessage = "[OK] Sesión de shell interactiva finalizada"
		}
		m.statusExpiry = time.Now().Add(4 * time.Second)
		return m, tea.ClearScreen

	case tickMsg:
		if !m.statusExpiry.IsZero() && time.Now().After(m.statusExpiry) {
			m.statusMessage = ""
			m.statusExpiry = time.Time{}
		}
		if m.activeState == stateFleetTable || m.activeState == stateFiltering {
			cmds = append(cmds, m.fetchMetrics())
		}
		cmds = append(cmds, tickCmd())

	case diagnosisResultMsg:
		m.diagnosisCache[msg.containerID] = msg.diagnosis
		m.lastDiagnosisUsage[msg.containerID] = msg.usage
		if msg.usage.TotalTokens > 0 {
			m.sessionTokensUsed += msg.usage.TotalTokens
			m.sessionCostUSD += msg.usage.EstimatedCostUSD
		}
		delete(m.triagePending, msg.containerID)

	case remediationResultMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("[!!] Error ejecutando %s en '%s': %v", msg.action, msg.containerName, msg.err)
		} else {
			m.statusMessage = fmt.Sprintf("[OK] %s completado en '%s' (%v)", strings.ToUpper(string(msg.action)), msg.containerName, msg.elapsed.Round(time.Millisecond))
		}
		m.statusExpiry = time.Now().Add(6 * time.Second)
		cmds = append(cmds, m.fetchMetrics())

	case logsMsg:
		if msg.err != nil {
			m.viewport.SetContent(fmt.Sprintf("[ERROR] Fallo al leer logs: %v", msg.err))
		} else if msg.content == "" {
			m.viewport.SetContent("No hay registros disponibles para este contenedor.")
		} else {
			m.viewport.SetContent(msg.content)
			m.viewport.GotoBottom()
		}

	case []domain.ContainerMetric:
		m.metrics = msg
		m.lastSync = time.Now()
		m.lastError = ""

		// Actualizar historial y podar contenedores eliminados
		activeIDs := make(map[string]bool, len(msg))
		for _, c := range msg {
			activeIDs[c.ID] = true
			hist, exists := m.metricsHistory[c.ID]
			if !exists {
				hist = &MetricHistory{}
				m.metricsHistory[c.ID] = hist
			}
			ramMB := float64(c.RAMBytes) / (1024 * 1024)
			hist.AddSample(c.CPUPercent, ramMB)
		}
		for id := range m.metricsHistory {
			if !activeIDs[id] {
				delete(m.metricsHistory, id)
			}
		}

		filtered := m.filteredMetrics()
		if m.cursor >= len(filtered) && len(filtered) > 0 {
			m.cursor = len(filtered) - 1
		}
		if len(filtered) > 0 && m.cursor < len(filtered) {
			if cmd := m.triggerTriageIfAnomalous(filtered[m.cursor]); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case error:
		m.lastError = msg.Error()
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	baseView := ""
	switch m.activeState {
	case stateLogViewer:
		baseView = m.viewLogs()
	default:
		baseView = m.viewTable()
	}

	if m.activeState == stateHelp {
		return overlayModal(baseView, m.viewHelp(), m.width, m.height)
	}

	if m.activeState == stateConfirmRemediation {
		return overlayModal(baseView, m.viewConfirmModal(), m.width, m.height)
	}

	if m.activeState == stateConfigModal {
		return overlayModal(baseView, m.viewConfigModal(), m.width, m.height)
	}

	return baseView
}

func (m Model) viewConfirmModal() string {
	actionStr := strings.ToUpper(string(m.pendingAction))
	title := StyleModalTitle.Render(fmt.Sprintf("Confirmar acción: %s", actionStr))

	question := fmt.Sprintf("¿Deseas ejecutar %s en el contenedor?", actionStr)

	modalWidth := 70
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}

	innerW := modalWidth - 6
	maxNameW := max(10, innerW-14)
	cName := truncate(m.pendingContainer.Name, maxNameW)

	containerLine := fmt.Sprintf("Contenedor : %s", lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0).Bold(true).Render(cName))
	idLine := fmt.Sprintf("ID         : %s", lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render(m.pendingContainer.ID))

	btnBase := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderBackground(ColorSurface0).
		Width(24).
		Align(lipgloss.Center)

	var btnConfirmStyle, btnCancelStyle lipgloss.Style

	if m.confirmModalBtn == 0 {
		btnConfirmStyle = btnBase.
			BorderForeground(ColorGreen).
			Background(ColorGreen).
			Foreground(ColorBase).
			Bold(true)

		btnCancelStyle = btnBase.
			BorderForeground(ColorSurface2).
			Background(ColorSurface0).
			Foreground(ColorSubtext0)
	} else {
		btnConfirmStyle = btnBase.
			BorderForeground(ColorSurface2).
			Background(ColorSurface0).
			Foreground(ColorSubtext0)

		btnCancelStyle = btnBase.
			BorderForeground(ColorPeach).
			Background(ColorPeach).
			Foreground(ColorBase).
			Bold(true)
	}

	btnConfirm := btnConfirmStyle.Render("✔  CONFIRMAR (y)")
	btnCancel := btnCancelStyle.Render("✖  CANCELAR (n/Esc)")
	sep := lipgloss.NewStyle().Background(ColorSurface0).Render("      \n      \n      ")

	buttonsRow := lipgloss.JoinHorizontal(lipgloss.Top, btnConfirm, sep, btnCancel)
	centeredButtons := lipgloss.NewStyle().
		Width(innerW).
		Align(lipgloss.Center).
		Background(ColorSurface0).
		Render(buttonsRow)

	body := fmt.Sprintf("%s\n\n%s\n\n  %s\n  %s\n\n%s",
		title,
		question,
		containerLine,
		idLine,
		centeredButtons,
	)

	return StyleModal.Width(modalWidth).Render(body)
}

type modelListItem struct {
	isHeader     bool
	headerTitle  string
	isCustom     bool
	option       domain.ModelOption
	isConfigured bool
	isActive     bool
}

func (m Model) getFilteredModelItems() []modelListItem {
	q := strings.ToLower(strings.TrimSpace(m.modelSearchInput.Value()))
	var items []modelListItem

	if q != "" {
		for _, mo := range domain.CatalogModels {
			pCfg := m.aiConfig.Providers[mo.Provider]
			meta := domain.GetProviderMeta(mo.Provider)
			isCfg := (!meta.RequiresKey) || (pCfg.APIKey != "")
			isAct := (mo.Provider == m.aiConfig.ActiveProvider && mo.ID == m.aiConfig.ActiveModel)

			if strings.Contains(strings.ToLower(mo.DisplayName), q) ||
				strings.Contains(strings.ToLower(mo.ID), q) ||
				strings.Contains(strings.ToLower(meta.Name), q) ||
				strings.Contains(strings.ToLower(string(mo.Provider)), q) {
				items = append(items, modelListItem{
					option:       mo,
					isConfigured: isCfg,
					isActive:     isAct,
				})
			}
		}
		items = append(items, modelListItem{isCustom: true})
		return items
	}

	var activeItems []modelListItem
	var configuredItems []modelListItem
	var otherItems []modelListItem

	for _, mo := range domain.CatalogModels {
		pCfg := m.aiConfig.Providers[mo.Provider]
		meta := domain.GetProviderMeta(mo.Provider)
		isCfg := (!meta.RequiresKey) || (pCfg.APIKey != "")
		isAct := (mo.Provider == m.aiConfig.ActiveProvider && mo.ID == m.aiConfig.ActiveModel)

		item := modelListItem{
			option:       mo,
			isConfigured: isCfg,
			isActive:     isAct,
		}

		if isAct {
			activeItems = append(activeItems, item)
		} else if isCfg {
			configuredItems = append(configuredItems, item)
		} else {
			otherItems = append(otherItems, item)
		}
	}

	if len(activeItems) > 0 {
		items = append(items, modelListItem{isHeader: true, headerTitle: "Active"})
		items = append(items, activeItems...)
	}

	if len(configuredItems) > 0 {
		items = append(items, modelListItem{isHeader: true, headerTitle: "Configured"})
		items = append(items, configuredItems...)
	}

	if len(otherItems) > 0 {
		items = append(items, modelListItem{isHeader: true, headerTitle: "Other Providers"})
		items = append(items, otherItems...)
	}

	items = append(items, modelListItem{isCustom: true})
	return items
}

func (m Model) viewConfigModal() string {
	modalWidth := 66
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	switch m.configMode {
	case configViewConnectKey:
		meta := domain.GetProviderMeta(m.connectProvider)
		pCfg := m.aiConfig.Providers[m.connectProvider]

		escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
		headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender).Background(ColorSurface0).Render(fmt.Sprintf("Configurar %s", meta.Name))
		spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
		header := lipgloss.NewStyle().Background(ColorSurface0).Render(headerLeft + strings.Repeat(" ", spLen) + escBadge)

		keyBorderColor := ColorSurface2
		if m.connectFocusField == 0 {
			keyBorderColor = ColorMauve
		}
		keyBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(keyBorderColor).
			BorderBackground(ColorSurface0).
			Background(ColorSurface0).
			Padding(0, 1).
			Width(innerW - 4).
			Render(m.apiKeyInput.View())

		epBorderColor := ColorSurface2
		if m.connectFocusField == 1 {
			epBorderColor = ColorMauve
		}
		epBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(epBorderColor).
			BorderBackground(ColorSurface0).
			Background(ColorSurface0).
			Padding(0, 1).
			Width(innerW - 4).
			Render(m.endpointInput.View())

		masked := pCfg.MaskedKey()
		statusInfo := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render(fmt.Sprintf("Estado actual: %s", lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Render(masked)))

		saveBtn := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGreen).
			BorderBackground(ColorSurface0).
			Background(ColorGreen).
			Foreground(ColorBase).
			Bold(true).
			Padding(0, 3).
			Render("✔  GUARDAR (Enter)")

		centeredSave := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Background(ColorSurface0).Render(saveBtn)

		body := fmt.Sprintf("%s\n\n%s\n%s\n\n%s\n\n%s", header, keyBox, epBox, statusInfo, centeredSave)
		return StyleModal.Width(modalWidth).Render(body)

	case configViewCustomModel:
		escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
		headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender).Background(ColorSurface0).Render("Modelo personalizado")
		spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
		header := lipgloss.NewStyle().Background(ColorSurface0).Render(headerLeft + strings.Repeat(" ", spLen) + escBadge)

		inputBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMauve).
			BorderBackground(ColorSurface0).
			Background(ColorSurface0).
			Padding(0, 1).
			Width(innerW - 4).
			Render(m.customModelInput.View())

		saveBtn := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGreen).
			BorderBackground(ColorSurface0).
			Background(ColorGreen).
			Foreground(ColorBase).
			Bold(true).
			Padding(0, 3).
			Render("✔  ACTIVAR (Enter)")

		centeredSave := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Background(ColorSurface0).Render(saveBtn)

		body := fmt.Sprintf("%s\n\n%s\n\n%s", header, inputBox, centeredSave)
		return StyleModal.Width(modalWidth).Render(body)

	default: // configViewSelectModel (Estilo OpenCode)
		escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
		headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorText).Background(ColorSurface0).Render("Select model")
		spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
		header := lipgloss.NewStyle().Background(ColorSurface0).Render(headerLeft + strings.Repeat(" ", spLen) + escBadge)

		searchLine := m.modelSearchInput.View()

		items := m.getFilteredModelItems()
		var lines []string

		for i, it := range items {
			if it.isHeader {
				headerStyle := lipgloss.NewStyle().Foreground(ColorMauve).Background(ColorSurface0).Bold(true)
				lines = append(lines, "\n"+headerStyle.Render(it.headerTitle))
				continue
			}

			if it.isCustom {
				customLabel := "+ Custom Model ID..."
				if i == m.modelListCursor {
					customStyled := lipgloss.NewStyle().
						Background(ColorSurface0).
						Foreground(ColorPeach).
						Bold(true).
						Width(innerW).
						Render("  " + customLabel)
					lines = append(lines, customStyled)
				} else {
					lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("  "+customLabel))
				}
				continue
			}

			meta := domain.GetProviderMeta(it.option.Provider)
			pCfg := m.aiConfig.Providers[it.option.Provider]
			keyMask := pCfg.MaskedKey()
			if !meta.RequiresKey {
				keyMask = "Local"
			}

			badge := "  "
			if it.isActive {
				badge = "● "
			}

			namePart := fmt.Sprintf("%s%s", badge, it.option.DisplayName)
			provPart := fmt.Sprintf("%s (%s)", meta.Name, keyMask)

			availW := innerW - 2
			nameW := max(12, availW-len(provPart)-2)
			nameTrunc := truncate(namePart, nameW)
			sp := max(1, availW-len(nameTrunc)-len(provPart))
			rowText := fmt.Sprintf("  %s%s%s", nameTrunc, strings.Repeat(" ", sp), provPart)

			if i == m.modelListCursor {
				rowStyled := lipgloss.NewStyle().
					Background(ColorSurface0).
					Foreground(ColorPeach).
					Bold(true).
					Width(innerW).
					Render(rowText)
				lines = append(lines, rowStyled)
			} else {
				rowStyled := lipgloss.NewStyle().
					Foreground(ColorText).
					Background(ColorSurface0).
					Width(innerW).
					Render(rowText)
				lines = append(lines, rowStyled)
			}
		}

		listBlock := strings.Join(lines, "\n")
		sep := lipgloss.NewStyle().Foreground(ColorSurface1).Background(ColorSurface0).Render(strings.Repeat("─", innerW))

		footer := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render(
			"Enter: Select   Ctrl+A: Connect provider   Esc: Close",
		)

		body := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n%s", header, searchLine, listBlock, sep, footer)
		return StyleModal.Width(modalWidth).Render(body)
	}
}


// overlayModal superpone la caja modal centrada encima de la vista base sin descartar su contenido
func overlayModal(bg, modal string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	modalLines := strings.Split(modal, "\n")

	modalH := len(modalLines)
	modalW := lipgloss.Width(modal)

	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}

	startY := (height - modalH) / 2
	if startY < 0 {
		startY = 0
	}

	startX := (width - modalW) / 2
	if startX < 0 {
		startX = 0
	}

	for i := 0; i < modalH && (startY+i) < len(bgLines); i++ {
		lineIdx := startY + i
		origLine := bgLines[lineIdx]
		mLine := modalLines[i]

		leftPart := ansi.Cut(origLine, 0, startX)
		leftW := ansi.StringWidth(leftPart)
		if leftW < startX {
			leftPart += strings.Repeat(" ", startX-leftW)
		}

		rightPart := ansi.TruncateLeft(origLine, startX+modalW, "")

		bgLines[lineIdx] = leftPart + mLine + rightPart
	}

	return strings.Join(bgLines, "\n")
}


func (m Model) viewTable() string {
	var b strings.Builder

	// Header superior con Branding de SOLV
	branding := StyleSolvBranding.Render("SOLV") + lipgloss.NewStyle().Foreground(ColorSubtext0).Render(" :: OPERATOR WORKSPACE")
	b.WriteString(branding + "\n")

	if m.lastError != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorRed).Render(fmt.Sprintf("[ERROR] %s\n", m.lastError)))
	}

	filtered := m.filteredMetrics()

	// Si la terminal tiene ancho suficiente (>= 80), renderizamos el Split-Pane
	if m.width >= 80 {
		leftWidth := 38
		if m.width > 120 {
			leftWidth = min(42, int(float64(m.width)*0.32))
		}
		rightWidth := max(38, m.width-leftWidth-2)

		auxLines := 4
		if m.lastError != "" {
			auxLines++
		}
		if len(filtered) > 0 && m.cursor < len(filtered) && m.isAnomalous(filtered[m.cursor]) {
			auxLines += 4
		}
		panelHeight := max(6, m.height-auxLines)

		// Panel Izquierdo: Lista de Contenedores (Master)
		var leftContent strings.Builder
		innerLeftW := leftWidth - 4 // Ancho libre dentro de StyleCard (borde + padding)
		maxVisibleRows := max(3, panelHeight-3)

		total := len(filtered)
		start := 0
		if m.cursor >= maxVisibleRows {
			start = m.cursor - maxVisibleRows + 1
		}
		end := start + maxVisibleRows
		if end > total {
			end = total
			start = max(0, end-maxVisibleRows)
		}

		var leftHeader string
		if total > maxVisibleRows {
			leftHeader = fmt.Sprintf("CONTENEDORES (%d) • %d-%d", total, start+1, end)
		} else {
			leftHeader = fmt.Sprintf("CONTENEDORES (%d)", total)
		}
		leftContent.WriteString(StyleCardTitle.Render(leftHeader) + "\n\n")

		if total == 0 {
			if m.filterValue != "" {
				leftContent.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("Sin resultados para '%s'", m.filterValue)))
			} else {
				leftContent.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render("Escaneando socket..."))
			}
		} else {
			nameW := max(8, innerLeftW-14)
			for i := start; i < end; i++ {
				c := filtered[i]
				glyph, _, statusStyle := FormatStatus(c.Status, c.RAMBytes, c.RAMLimitBytes)
				tech := DetectTechnology(c.Image, c.Name)

				prefix := "  "
				if i == m.cursor {
					prefix = "> "
				}

				nameStr := truncate(c.Name, nameW)
				namePadded := fmt.Sprintf("%-*s", nameW, nameStr)
				cpuStr := fmt.Sprintf("%3.0f%%", c.CPUPercent)

				var cpuStyled string
				if c.CPUPercent >= 80 {
					cpuStyled = lipgloss.NewStyle().Foreground(ColorRed).Bold(true).Render(cpuStr)
				} else if c.CPUPercent >= 50 {
					cpuStyled = lipgloss.NewStyle().Foreground(ColorPeach).Render(cpuStr)
				} else {
					cpuStyled = lipgloss.NewStyle().Foreground(ColorSubtext0).Render(cpuStr)
				}

				techGlyphStyled := lipgloss.NewStyle().Foreground(tech.Color).Render(tech.NerdGlyph)
				rowLine := fmt.Sprintf("%s%s %s %s %s", prefix, statusStyle.Render(glyph), techGlyphStyled, namePadded, cpuStyled)

				if i == m.cursor {
					leftContent.WriteString(StyleRowFocus.Width(innerLeftW).Render(rowLine) + "\n")
				} else {
					leftContent.WriteString(lipgloss.NewStyle().Width(innerLeftW).Render(rowLine) + "\n")
				}
			}
		}

		leftPanel := StyleCard.Width(leftWidth).Height(panelHeight).Render(leftContent.String())

		// Panel Derecho: Ficha Técnica Viva (Detail)
		var rightContent strings.Builder
		if len(filtered) > 0 && m.cursor < len(filtered) {
			sel := filtered[m.cursor]
			tech := DetectTechnology(sel.Image, sel.Name)
			_, statusText, statusStyle := FormatStatus(sel.Status, sel.RAMBytes, sel.RAMLimitBytes)
			statusBadge := statusStyle.Render(fmt.Sprintf("[%s]", statusText))

			// Header del contenedor
			headerLine := fmt.Sprintf("%s  %s  %s", tech.Badge(), lipgloss.NewStyle().Bold(true).Foreground(ColorText).Render(sel.Name), statusBadge)
			rightContent.WriteString(headerLine + "\n")

			subInfo := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("  Imagen: %s  |  ID: %s  |  Categoría: %s", sel.Image, sel.ID, tech.Category))
			rightContent.WriteString(subInfo + "\n\n")

			// Tarjeta Térmica de Recursos
			rightContent.WriteString(StyleCardTitle.Render("METRICAS EN TIEMPO REAL:") + "\n")

			// CPU
			hist := m.metricsHistory[sel.ID]
			cpuSpark := ""
			cpuMinStr := "0.0%"
			cpuMaxStr := "0.0%"
			trendStyled := "≈ estable"
			if hist != nil && len(hist.CPU) > 0 {
				cpuSpark = RenderSparkline(hist.CPU, 0, 100, 16)
				trend := hist.CalculateCPUTrend()
				trendStyled = trend.Style.Render(fmt.Sprintf("%s %s", trend.Symbol, trend.Label))
				minC, maxC := hist.CPUMinMax()
				cpuMinStr = fmt.Sprintf("%.1f%%", minC)
				cpuMaxStr = fmt.Sprintf("%.1f%%", maxC)
			}
			cpuBar := RenderGradientBar(sel.CPUPercent, 100.0, 10)
			sparkBlock := ""
			if cpuSpark != "" {
				sparkBlock = fmt.Sprintf(" [%s]", cpuSpark)
			}
			rightContent.WriteString(fmt.Sprintf("  CPU: %5.1f%% %s%s %s\n", sel.CPUPercent, cpuBar, sparkBlock, trendStyled))
			if panelHeight >= 12 {
				rightContent.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("       (Mín: %s | Máx: %s)", cpuMinStr, cpuMaxStr)) + "\n")
			}

			// RAM
			ramMB := float64(sel.RAMBytes) / (1024 * 1024)
			if sel.RAMLimitBytes > 0 {
				limitMB := float64(sel.RAMLimitBytes) / (1024 * 1024)
				ramBar := RenderGradientBar(ramMB, limitMB, 12)
				pct := (ramMB / limitMB) * 100.0
				rightContent.WriteString(fmt.Sprintf("  RAM: %6.1f MB / %6.1f MB %s %5.1f%%\n", ramMB, limitMB, ramBar, pct))
			} else {
				peakMB := ramMB
				if hist != nil {
					peakMB = math.Max(ramMB, hist.PeakRAM())
				}
				ramBar := RenderGradientBar(ramMB, peakMB, 12)
				rightContent.WriteString(fmt.Sprintf("  RAM: %6.1f MB %s (Sin límite Docker)\n", ramMB, ramBar))
			}

			// Egress / Red
			egressStr, egressStyle := FormatEgress(sel.EgressBytesSec)
			rightContent.WriteString(fmt.Sprintf("  RED: Salida: %s\n\n", egressStyle.Render(egressStr)))

			// Sección de Acciones Rápidas
			rightContent.WriteString(StyleCardTitle.Render("ACCIONES DISPONIBLES:") + "\n")
			rightContent.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render("  [l/Enter] Logs en vivo    •  [e] Shell interactiva\n  [r] Restart  •  [s] Stop  •  [x] Aislar Red\n"))
		} else {
			rightContent.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render("Selecciona un contenedor de la lista izquierda."))
		}

		rightPanel := StyleCard.Width(rightWidth).Height(panelHeight).Render(rightContent.String())
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel) + "\n")
	} else {
		// Modo clásico colapsado para terminales angostas (< 80 cols)
		headers := fmt.Sprintf("  %-14s %-20s %-10s", "ID", "CONTAINER", "STATUS")
		b.WriteString(StyleHeader.Render(headers) + "\n")
		for i, c := range filtered {
			glyph, statusText, statusStyle := FormatStatus(c.Status, c.RAMBytes, c.RAMLimitBytes)
			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}
			line := fmt.Sprintf("%s%-14s %-20s %s", prefix, c.ID, truncate(c.Name, 18), statusStyle.Render(fmt.Sprintf("%s %s", glyph, statusText)))
			if i == m.cursor {
				b.WriteString(StyleRowFocus.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
	}

	// Banner AIOps contextual si el contenedor seleccionado tiene anomalía
	if len(filtered) > 0 && m.cursor < len(filtered) {
		selected := filtered[m.cursor]
		if m.isAnomalous(selected) {
			diag, cached := m.diagnosisCache[selected.ID]
			if !cached {
				if m.triagePending[selected.ID] {
					diag = "Analizando causa raíz con IA..."
				} else {
					diag = "Pendiente de diagnóstico analítico..."
				}
			}
			usage, hasUsage := m.lastDiagnosisUsage[selected.ID]
			usageBadge := ""
			if hasUsage && usage.TotalTokens > 0 {
				usageBadge = lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("  [%d tok • ~$%.4f]", usage.TotalTokens, usage.EstimatedCostUSD))
			}
			bannerContent := fmt.Sprintf("%s %s%s", StyleAIOpsTag.Render("[AIOps]"), diag, usageBadge)
			b.WriteString("\n" + StyleAIOpsBanner.Width(max(40, m.width-8)).Render(bannerContent) + "\n")
		}
	}

	// Barra inferior / Filtro
	if m.activeState == stateFiltering {
		b.WriteString("\n" + m.filterInput.View() + "\n")
	} else {
		if m.statusMessage != "" && time.Now().Before(m.statusExpiry) {
			b.WriteString("\n" + StyleStatusBar.Render(m.statusMessage))
		} else {
			filterTag := ""
			if m.filterValue != "" {
				filterTag = fmt.Sprintf(" [Filtro: '%s']", m.filterValue)
			}
			aiStatsTag := ""
			if m.sessionTokensUsed > 0 {
				aiStatsTag = fmt.Sprintf("  |  󰚩 %d tok (~$%.3f)", m.sessionTokensUsed, m.sessionCostUSD)
			}
			shortcuts := fmt.Sprintf("[j/k, Scroll]: Navegar  |  [l/Enter]: Logs  |  [e]: Shell  |  [r]: Restart  |  [s]: Stop  |  [x]: Aislar  |  [c]: Modelo IA  |  [/]: Filtro%s%s", filterTag, aiStatsTag)
			b.WriteString("\n" + StyleStatusBar.Render(shortcuts))
		}
	}

	return b.String()
}

func (m Model) viewLogs() string {
	var b strings.Builder

	glyph, statusText, statusStyle := FormatStatus(m.selectedState, 0, 0)
	statusBadge := statusStyle.Render(fmt.Sprintf("%s %s", glyph, statusText))

	breadcrumb := fmt.Sprintf("[<] Volver (Esc) | Logs: %s | Estado: %s", m.selectedName, statusBadge)
	b.WriteString(StyleTitle.Render(breadcrumb) + "\n")

	// Buscar métrica actual del contenedor seleccionado
	var selectedMetric *domain.ContainerMetric
	for _, c := range m.metrics {
		if c.ID == m.selectedID {
			selectedMetric = &c
			break
		}
	}

	hist := m.metricsHistory[m.selectedID]
	var trendLines []string

	// 1. Línea de CPU con sparkline cuantitativa y aceleración
	if hist != nil && len(hist.CPU) > 0 {
		cpuSpark := RenderSparkline(hist.CPU, 0, 100, 20)
		trend := hist.CalculateCPUTrend()
		trendStyled := trend.Style.Render(fmt.Sprintf("%s %s", trend.Symbol, trend.Label))
		minC, maxC := hist.CPUMinMax()
		curCPU := hist.CPU[len(hist.CPU)-1]
		cpuBar := RenderGradientBar(curCPU, 100.0, 12)
		trendLines = append(trendLines, fmt.Sprintf("  CPU: %5.1f%% %s [%s] %s  (Mín: %.1f%% | Máx: %.1f%%)", curCPU, cpuBar, cpuSpark, trendStyled, minC, maxC))
	} else if selectedMetric != nil {
		cpuBar := RenderGradientBar(selectedMetric.CPUPercent, 100.0, 12)
		trendLines = append(trendLines, fmt.Sprintf("  CPU: %5.1f%% %s", selectedMetric.CPUPercent, cpuBar))
	}

	// 2. Línea de RAM con medidor térmico btop y sparkline
	if hist != nil && len(hist.RAM) > 0 {
		ramSpark := RenderSparkline(hist.RAM, 0, hist.PeakRAM()*1.2, 20)
		curRAM := hist.RAM[len(hist.RAM)-1]
		if selectedMetric != nil && selectedMetric.RAMLimitBytes > 0 {
			limitMB := float64(selectedMetric.RAMLimitBytes) / (1024 * 1024)
			ramBar := RenderGradientBar(curRAM, limitMB, 12)
			pct := (curRAM / limitMB) * 100.0
			trendLines = append(trendLines, fmt.Sprintf("  RAM: %6.1f MB / %6.1f MB %s [%s] %5.1f%%", curRAM, limitMB, ramBar, ramSpark, pct))
		} else {
			peakMB := hist.PeakRAM()
			ramBar := RenderGradientBar(curRAM, peakMB, 12)
			trendLines = append(trendLines, fmt.Sprintf("  RAM: %6.1f MB %s [%s] (Pico: %.1f MB, Sin límite Docker)", curRAM, ramBar, ramSpark, peakMB))
		}
	} else if selectedMetric != nil {
		ramMB := float64(selectedMetric.RAMBytes) / (1024 * 1024)
		if selectedMetric.RAMLimitBytes > 0 {
			limitMB := float64(selectedMetric.RAMLimitBytes) / (1024 * 1024)
			ramBar := RenderGradientBar(ramMB, limitMB, 12)
			pct := (ramMB / limitMB) * 100.0
			trendLines = append(trendLines, fmt.Sprintf("  RAM: %6.1f MB / %6.1f MB %s %5.1f%%", ramMB, limitMB, ramBar, pct))
		} else {
			ramBar := RenderGradientBar(ramMB, ramMB, 12)
			trendLines = append(trendLines, fmt.Sprintf("  RAM: %6.1f MB %s", ramMB, ramBar))
		}
	}

	if len(trendLines) > 0 {
		trendBox := StyleTrendsBox.Render(strings.Join(trendLines, "\n"))
		b.WriteString(trendBox + "\n")
	}

	// Renderizar el visor de logs scrollable
	b.WriteString(m.viewport.View() + "\n")

	helpBar := lipgloss.NewStyle().Foreground(ColorSubtext0).Render("[Esc]: Volver a la flota  |  [Flechas / Scroll]: Desplazar registros")
	b.WriteString(helpBar)

	return b.String()
}

func (m Model) viewHelp() string {
	modalWidth := 72
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Background(ColorSurface0).Render("Atajos de teclado")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
	header := lipgloss.NewStyle().Background(ColorSurface0).Render(headerLeft + strings.Repeat(" ", spLen) + escBadge)

	colW := (innerW - 4) / 2
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender).Background(ColorSurface0)
	keyStyle := lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0)

	// Columna Izquierda: Navegación & Modelos
	var leftCol strings.Builder
	leftCol.WriteString(sectionTitle.Render("Navegación") + "\n")
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("↑ / k"), descStyle.Render("Subir")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("↓ / j"), descStyle.Render("Bajar")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("Enter / l"), descStyle.Render("Ver logs")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n\n", keyStyle.Render("/"), descStyle.Render("Buscar / Filtrar")))

	leftCol.WriteString(sectionTitle.Render("Modelos de IA") + "\n")
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("c"), descStyle.Render("Elegir modelo")))
	leftCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("Ctrl+A"), descStyle.Render("Configurar API Key")))

	// Columna Derecha: Acciones & General
	var rightCol strings.Builder
	rightCol.WriteString(sectionTitle.Render("Acciones") + "\n")
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("r"), descStyle.Render("Reiniciar")))
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("s"), descStyle.Render("Detener")))
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("x"), descStyle.Render("Aislar de red")))
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n\n", keyStyle.Render("e"), descStyle.Render("Abrir terminal")))

	rightCol.WriteString(sectionTitle.Render("General") + "\n")
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("?"), descStyle.Render("Ver esta ayuda")))
	rightCol.WriteString(fmt.Sprintf("  %-11s %s\n", keyStyle.Render("q / Esc"), descStyle.Render("Cerrar / Salir")))

	leftBlock := lipgloss.NewStyle().Width(colW).Background(ColorSurface0).Render(leftCol.String())
	rightBlock := lipgloss.NewStyle().Width(colW).Background(ColorSurface0).Render(rightCol.String())
	sep := lipgloss.NewStyle().Background(ColorSurface0).Render("    ")

	columnsRow := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, sep, rightBlock)
	centeredColumns := lipgloss.NewStyle().Width(innerW).Background(ColorSurface0).Render(columnsRow)

	body := fmt.Sprintf("%s\n\n%s", header, centeredColumns)
	return StyleModal.Width(modalWidth).Render(body)
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
