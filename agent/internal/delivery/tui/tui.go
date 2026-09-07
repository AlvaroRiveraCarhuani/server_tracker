package tui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/ai"
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
	triageClient     TriageService
	diagnosisCache   map[string]string
	triagePending    map[string]bool
	metricsHistory   map[string]*MetricHistory
	metrics          []domain.ContainerMetric
	cursor           int
	activeState      sessionState
	filterInput      textinput.Model
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

// NewModel inicializa el modelo de la TUI.
func NewModel(collector ports.CollectorPort) Model {
	ti := textinput.New()
	ti.Placeholder = "filtrar por nombre o imagen..."
	ti.Prompt = "/ "
	ti.PromptStyle = StyleFilterPrompt

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

	return Model{
		collector:      collector,
		triageClient:   ai.NewTriageClient(),
		diagnosisCache: make(map[string]string),
		triagePending:  make(map[string]bool),
		metricsHistory: make(map[string]*MetricHistory),
		cursor:         0,
		activeState:    stateFleetTable,
		filterInput:    ti,
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

	content := ""
	switch m.activeState {
	case stateLogViewer:
		content = m.viewLogs()
	case stateHelp:
		content = m.viewHelp()
	default:
		content = m.viewTable()
	}

	// Expandir el contenedor a todo el ancho y alto disponible de la terminal
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSurface1).
		Padding(0, 1).
		Width(max(60, m.width-4)).
		Height(max(10, m.height-2))

	return cardStyle.Render(content)
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

func (m Model) viewTable() string {
	var b strings.Builder

	// Header superior
	b.WriteString(StyleTitle.Render("SOLV SERVER TRACKER :: DATA PLANE"))
	b.WriteString("\n")

	if m.lastError != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorRed).Render(fmt.Sprintf("[ERROR] %s\n", m.lastError)))
	}

	// Ancho dinámico para las columnas
	hasCPUMeter := m.width >= 95
	cpuColWidth := 10
	if hasCPUMeter {
		cpuColWidth = 14
	}
	fixedColsWidth := 14 + 18 + cpuColWidth + 12 + 16 + 6 // ID + STATUS + CPU + RAM + EGRESS + márgenes
	nameColWidth := max(22, m.width-fixedColsWidth-8)

	// Encabezado de columnas
	headers := fmt.Sprintf("  %-14s %-*s %-18s %-*s %-12s %16s",
		"ID", nameColWidth, "CONTAINER", "STATUS", cpuColWidth, "CPU %", "RAM (MB)", "EGRESS")
	b.WriteString(StyleHeader.Render(headers))
	b.WriteString("\n")

	filtered := m.filteredMetrics()
	if len(filtered) == 0 {
		if m.filterValue != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("  No se encontraron contenedores que coincidan con '%s'\n", m.filterValue)))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render("  Escaneando socket /var/run/docker.sock...\n"))
		}
	} else {
		for i, c := range filtered {
			glyph, statusText, statusStyle := FormatStatus(c.Status, c.RAMBytes, c.RAMLimitBytes)
			statusCell := fmt.Sprintf("%-4s %-10s", glyph, statusText)
			statusStyled := statusStyle.Render(statusCell)

			ramMB := float64(c.RAMBytes) / (1024 * 1024)
			egressStr, egressStyle := FormatEgress(c.EgressBytesSec)

			colID := fmt.Sprintf("%-14s", c.ID)
			colName := fmt.Sprintf("%-*s", nameColWidth, truncate(c.Name, nameColWidth-2))

			var colCPU string
			if hasCPUMeter {
				colCPU = fmt.Sprintf("%-5.1f %s", c.CPUPercent, RenderGradientBar(c.CPUPercent, 100.0, 4))
			} else {
				colCPU = fmt.Sprintf("%-10.1f", c.CPUPercent)
			}

			colRAM := fmt.Sprintf("%-12.1f", ramMB)
			colEgress := egressStyle.Render(fmt.Sprintf("%16s", egressStr))

			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}

			line := fmt.Sprintf("%s%s %s %s %-*s %s %s",
				prefix, colID, colName, statusStyled, cpuColWidth, colCPU, colRAM, colEgress)

			if i == m.cursor {
				b.WriteString(StyleRowFocus.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
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
			shortcuts := fmt.Sprintf("[j/k, Scroll]: Navegar  |  [l/Enter]: Logs  |  [r]: Restart  |  [s]: Stop  |  [x]: Aislar  |  [/]: Filtro%s", filterTag)
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
	cpuVal := 0.0
	if selectedMetric != nil {
		cpuVal = selectedMetric.CPUPercent
	}
	if hist != nil && len(hist.CPU) > 0 {
		spark := RenderSparkline(hist.CPU, 0, 100, 20)
		trend := hist.CalculateCPUTrend()
		minCPU, maxCPU := hist.CPUMinMax()
		trendStyled := trend.Style.Render(fmt.Sprintf("%s %s", trend.Symbol, trend.Label))
		trendLines = append(trendLines, fmt.Sprintf("  CPU: %5.1f%% [%s] %s  |  Mín: %.1f%%  Máx: %.1f%%", cpuVal, spark, trendStyled, minCPU, maxCPU))
	} else {
		trendLines = append(trendLines, fmt.Sprintf("  CPU: %5.1f%% %s", cpuVal, RenderGradientBar(cpuVal, 100.0, 14)))
	}

	// 2. Línea de RAM proporcional con gradiente térmico
	ramBytes := uint64(0)
	ramLimit := uint64(0)
	if selectedMetric != nil {
		ramBytes = selectedMetric.RAMBytes
		ramLimit = selectedMetric.RAMLimitBytes
	}
	ramMB := float64(ramBytes) / (1024 * 1024)

	if ramLimit > 0 {
		limitMB := float64(ramLimit) / (1024 * 1024)
		bar := RenderGradientBar(ramMB, limitMB, 14)
		pct := (ramMB / limitMB) * 100.0
		trendLines = append(trendLines, fmt.Sprintf("  RAM: %6.1f MB / %6.1f MB %s %5.1f%%", ramMB, limitMB, bar, pct))
	} else {
		peakMB := ramMB
		if hist != nil {
			peakMB = math.Max(ramMB, hist.PeakRAM())
		}
		bar := RenderGradientBar(ramMB, peakMB, 14)
		trendLines = append(trendLines, fmt.Sprintf("  RAM: %6.1f MB %s (Pico: %.1f MB, Sin límite Docker)", ramMB, bar, peakMB))
	}

	b.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render(strings.Join(trendLines, "\n")) + "\n\n")

	b.WriteString(m.viewport.View() + "\n\n")
	b.WriteString(StyleStatusBar.Render("[j/k, Up/Down, Scroll]: Scroll  |  [g/G]: Inicio/Fin  |  [Esc/h]: Volver a Flota"))

	return b.String()
}

func (m Model) viewHelp() string {
	var b strings.Builder

	b.WriteString(StyleTitle.Render("SOLV SERVER TRACKER :: ATAJOS DE TECLADO") + "\n\n")
	b.WriteString("  NAVEGACION:\n")
	b.WriteString("    j / Down / ScrollDown : Mover cursor hacia abajo (1 en 1)\n")
	b.WriteString("    k / Up / ScrollUp     : Mover cursor hacia arriba (1 en 1)\n")
	b.WriteString("    /                     : Abrir filtro interactivo en tiempo real\n")
	b.WriteString("    l / Enter             : Abrir visor de logs en vivo a pantalla completa\n\n")
	b.WriteString("  REMEDIACION EN CALIENTE (Cero RCE):\n")
	b.WriteString("    r                     : Reiniciar contenedor seleccionado (restart)\n")
	b.WriteString("    s                     : Detener contenedor seleccionado (stop)\n")
	b.WriteString("    x                     : Aislar contenedor de la red (isolate_network)\n\n")
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
func RunTUI(collector ports.CollectorPort) error {
	p := tea.NewProgram(
		NewModel(collector),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
