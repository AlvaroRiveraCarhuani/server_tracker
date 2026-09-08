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
}

type shellFinishedMsg struct {
	err error
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// TriageService define el contrato para el análisis pasivo con IA.
type TriageService interface {
	DiagnoseContainer(ctx context.Context, name, image, status, logs string) string
}

// Model representa el estado global de la TUI interactiva.
type Model struct {
	collector        ports.CollectorPort
	vaultService     ports.VaultPort
	triageClient     TriageService
	diagnosisCache   map[string]string
	triagePending    map[string]bool
	metricsHistory   map[string]*MetricHistory
	metrics          []domain.ContainerMetric
	cursor           int
	activeState      sessionState
	filterInput      textinput.Model
	apiKeyInput      textinput.Model
	filterValue      string
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

	ki := textinput.New()
	ki.Placeholder = "sk-or-v1-..."
	ki.Prompt = "Clave API: "
	ki.PromptStyle = StyleFilterPrompt
	ki.EchoMode = textinput.EchoPassword
	ki.EchoCharacter = '•'

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

	var triageClient TriageService
	if vaultSvc != nil {
		if key, err := vaultSvc.GetOpenRouterKey(); err == nil && key != "" {
			triageClient = ai.NewTriageClientWithKey(key)
		}
	}
	if triageClient == nil {
		triageClient = ai.NewTriageClient()
	}

	return Model{
		collector:      collector,
		vaultService:   vaultSvc,
		triageClient:   triageClient,
		diagnosisCache: make(map[string]string),
		triagePending:  make(map[string]bool),
		metricsHistory: make(map[string]*MetricHistory),
		cursor:         0,
		activeState:    stateFleetTable,
		filterInput:    ti,
		apiKeyInput:    ki,
		viewport:       vp,
		lastSync:       time.Now(),
		width:          100,
		height:         24,
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
		diag := m.triageClient.DiagnoseContainer(ctx, c.Name, c.Image, c.Status, logs)
		return diagnosisResultMsg{
			containerID: c.ID,
			diagnosis:   diag,
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
			switch msg.String() {
			case "esc":
				m.apiKeyInput.Blur()
				m.activeState = stateFleetTable
				return m, nil
			case "enter":
				val := strings.TrimSpace(m.apiKeyInput.Value())
				if val != "" {
					if m.vaultService != nil {
						if err := m.vaultService.SaveOpenRouterKey(val); err != nil {
							m.statusMessage = fmt.Sprintf("[!!] Error guardando clave en bóveda: %v", err)
						} else {
							m.statusMessage = "[OK] Clave de OpenRouter cifrada en bóveda local (AES-256-GCM)"
						}
					} else {
						m.statusMessage = "[OK] Clave de OpenRouter configurada en memoria"
					}
					m.triageClient = ai.NewTriageClientWithKey(val)
					m.diagnosisCache = make(map[string]string)
					m.statusExpiry = time.Now().Add(5 * time.Second)
				}
				m.apiKeyInput.Blur()
				m.activeState = stateFleetTable
				return m, nil
			default:
				var cmd tea.Cmd
				m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
				return m, cmd
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
				m.apiKeyInput.Reset()
				m.apiKeyInput.Focus()
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
	if m.activeState == stateConfirmRemediation {
		return m.viewConfirmModal()
	}
	if m.activeState == stateConfigModal {
		return m.viewConfigModal()
	}

	switch m.activeState {
	case stateLogViewer:
		return m.viewLogs()
	case stateHelp:
		return m.viewHelp()
	default:
		return m.viewTable()
	}
}

func (m Model) viewConfirmModal() string {
	actionStr := strings.ToUpper(string(m.pendingAction))
	title := StyleModalTitle.Render(fmt.Sprintf("[!!] CONFIRMAR REMEDIACION: %s", actionStr))

	question := fmt.Sprintf("¿Deseas ejecutar %s en el contenedor?\n\n  Contenedor : %s\n  ID          : %s",
		actionStr,
		lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(m.pendingContainer.Name),
		lipgloss.NewStyle().Foreground(ColorSubtext0).Render(m.pendingContainer.ID),
	)

	var btnConfirm, btnCancel string
	if m.confirmModalBtn == 0 {
		btnConfirm = StyleBtnFocusedConfirm.Render("[ Confirmar (y / Enter) ]")
		btnCancel = StyleBtnBlurred.Render("  Cancelar (n / Esc)  ")
	} else {
		btnConfirm = StyleBtnBlurred.Render("  Confirmar (y)  ")
		btnCancel = StyleBtnFocusedCancel.Render("[ Cancelar (n / Esc / Enter) ]")
	}

	keys := fmt.Sprintf("%s      %s", btnConfirm, btnCancel)

	body := fmt.Sprintf("%s\n\n%s\n\n%s", title, question, keys)
	modalWidth := 56
	if m.width > 20 && m.width-10 < modalWidth {
		modalWidth = m.width - 10
	}
	modal := StyleModal.Width(modalWidth).Render(body)

	return lipgloss.Place(
		max(60, m.width-4),
		max(10, m.height-2),
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)
}

func (m Model) viewConfigModal() string {
	title := StyleModalTitle.Render("  CONFIGURACION SEGURA: MOTOR AIOps  ")

	desc := lipgloss.NewStyle().Foreground(ColorText).Render(
		"Ingrese su API Key de OpenRouter para activar el\ndiagnóstico Zero-Prompt en tiempo real.\n\n" +
			lipgloss.NewStyle().Foreground(ColorSubtext0).Render("Blindaje D2: La clave se almacena cifrada en disco con\nAES-256-GCM y derivación de clave Argon2id (~/.solv/vault.enc)."),
	)

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMauve).
		Padding(0, 1).
		Width(52).
		Render(m.apiKeyInput.View())

	help := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(
		"[Enter] Guardar Cifrado    [Esc] Cancelar",
	)

	body := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", title, desc, inputBox, help)
	modalWidth := 58
	if m.width > 20 && m.width-10 < modalWidth {
		modalWidth = m.width - 10
	}
	modal := StyleModal.Width(modalWidth).Render(body)

	return lipgloss.Place(
		max(60, m.width-4),
		max(10, m.height-2),
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)
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
			auxLines += 3
		}
		panelHeight := max(8, m.height-auxLines)

		// Panel Izquierdo: Lista de Flota (Master)
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
			leftHeader = fmt.Sprintf("FLOTA (%d) • %d-%d", total, start+1, end)
		} else {
			leftHeader = fmt.Sprintf("FLOTA (%d)", total)
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
			rightContent.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("       (Mín: %s | Máx: %s)", cpuMinStr, cpuMaxStr)) + "\n\n")

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
				rightContent.WriteString(fmt.Sprintf("  RAM: %6.1f MB %s (Pico: %.1f MB, Sin límite Docker)\n", ramMB, ramBar, peakMB))
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
					diag = "Analizando contenedor con OpenRouter..."
				} else {
					diag = "Pendiente de diagnóstico analítico..."
				}
			}
			bannerContent := fmt.Sprintf("%s %s", StyleAIOpsTag.Render("[AIOps]"), diag)
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
			shortcuts := fmt.Sprintf("[j/k, Scroll]: Navegar  |  [l/Enter]: Logs  |  [e]: Shell  |  [r]: Restart  |  [s]: Stop  |  [x]: Aislar  |  [c]: Clave IA  |  [/]: Filtro%s", filterTag)
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
	var b strings.Builder

	b.WriteString(StyleModalTitle.Render("  SOLV SERVER TRACKER :: GUIA DE COMANDOS  ") + "\n\n")

	b.WriteString("  NAVEGACION:\n")
	b.WriteString("    j / Down / ScrollDown : Mover cursor hacia abajo (actualiza detalle vivo)\n")
	b.WriteString("    k / Up / ScrollUp     : Mover cursor hacia arriba (actualiza detalle vivo)\n")
	b.WriteString("    /                     : Abrir filtro interactivo en tiempo real\n")
	b.WriteString("    l / Enter             : Abrir visor de logs en vivo a pantalla completa\n")
	b.WriteString("    e                     : Abrir shell interactiva en contenedor (/bin/sh)\n\n")
	b.WriteString("  REMEDIACION EN CALIENTE (Cero RCE):\n")
	b.WriteString("    r                     : Reiniciar contenedor seleccionado (restart)\n")
	b.WriteString("    s                     : Detener contenedor seleccionado (stop)\n")
	b.WriteString("    x                     : Aislar contenedor de la red (isolate_network)\n\n")
	b.WriteString("  CONFIGURACION Y SEGURIDAD:\n")
	b.WriteString("    c                     : Configurar API Key de OpenRouter cifrada en reposo (AES-256-GCM)\n\n")
	b.WriteString("  SISTEMA:\n")
	b.WriteString("    Esc / h               : Cerrar visor / cancelar filtro o remediación\n")
	b.WriteString("    ?                     : Mostrar / ocultar esta ayuda\n")
	b.WriteString("    q / Ctrl+C            : Salir de la aplicación\n\n")
	b.WriteString(StyleStatusBar.Render("Presiona Esc para volver a la tabla de contenedores."))

	return b.String()
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
