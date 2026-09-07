package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
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
	stateHelp
)

type tickMsg time.Time
type logsMsg struct {
	containerName string
	content       string
	err           error
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Model representa el estado global de la TUI interactiva.
type Model struct {
	collector     ports.CollectorPort
	metrics       []domain.ContainerMetric
	cursor        int
	activeState   sessionState
	filterInput   textinput.Model
	filterValue   string
	viewport      viewport.Model
	selectedName  string
	selectedID    string
	selectedState string
	lastError     string
	lastSync      time.Time
	width         int
	height        int
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
		collector:   collector,
		cursor:      0,
		activeState: stateFleetTable,
		filterInput: ti,
		viewport:    vp,
		lastSync:    time.Now(),
		width:       100,
		height:      24,
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
		// Control preciso de scroll con la rueda del mouse: 1 fila exacta por paso
		switch msg.Type {
		case tea.MouseWheelDown:
			if m.activeState == stateFleetTable || m.activeState == stateFiltering {
				if len(filtered) > 0 && m.cursor < len(filtered)-1 {
					m.cursor++
				}
			} else if m.activeState == stateLogViewer {
				m.viewport.LineDown(1)
			}
		case tea.MouseWheelUp:
			if m.activeState == stateFleetTable || m.activeState == stateFiltering {
				if len(filtered) > 0 && m.cursor > 0 {
					m.cursor--
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

		case stateFleetTable:
			filtered := m.filteredMetrics()
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "j", "down":
				if len(filtered) > 0 {
					m.cursor = (m.cursor + 1) % len(filtered)
				}
			case "k", "up":
				if len(filtered) > 0 {
					m.cursor = (m.cursor - 1 + len(filtered)) % len(filtered)
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
				return m, m.fetchMetrics()
			case "?":
				m.activeState = stateHelp
			}
		}

	case tickMsg:
		if m.activeState == stateFleetTable || m.activeState == stateFiltering {
			cmds = append(cmds, m.fetchMetrics())
		}
		cmds = append(cmds, tickCmd())

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
		filtered := m.filteredMetrics()
		if m.cursor >= len(filtered) && len(filtered) > 0 {
			m.cursor = len(filtered) - 1
		}

	case error:
		m.lastError = msg.Error()
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
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

func (m Model) viewTable() string {
	var b strings.Builder

	// Header superior
	b.WriteString(StyleTitle.Render("SOLV SERVER TRACKER :: DATA PLANE"))
	b.WriteString("\n")

	if m.lastError != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorRed).Render(fmt.Sprintf("[ERROR] %s\n", m.lastError)))
	}

	// Ancho dinámico para la columna del nombre del contenedor
	fixedColsWidth := 14 + 18 + 10 + 12 + 16 + 6 // ID + STATUS + CPU + RAM + EGRESS + márgenes
	nameColWidth := max(24, m.width-fixedColsWidth-8)

	// Encabezado de columnas
	headers := fmt.Sprintf("  %-14s %-*s %-18s %-10s %-12s %16s",
		"ID", nameColWidth, "CONTAINER", "STATUS", "CPU %", "RAM (MB)", "EGRESS")
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
			colCPU := fmt.Sprintf("%-10.1f", c.CPUPercent)
			colRAM := fmt.Sprintf("%-12.1f", ramMB)
			colEgress := egressStyle.Render(fmt.Sprintf("%16s", egressStr))

			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}

			line := fmt.Sprintf("%s%s %s %s %s %s %s",
				prefix, colID, colName, statusStyled, colCPU, colRAM, colEgress)

			if i == m.cursor {
				b.WriteString(StyleRowFocus.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	// Barra inferior / Filtro
	if m.activeState == stateFiltering {
		b.WriteString("\n" + m.filterInput.View() + "\n")
	} else {
		filterTag := ""
		if m.filterValue != "" {
			filterTag = fmt.Sprintf(" [Filtro: '%s']", m.filterValue)
		}
		shortcuts := fmt.Sprintf("[j/k, Scroll]: Navegar  |  [/]: Filtrar  |  [l/Enter]: Logs  |  [r]: Refrescar  |  [?]: Ayuda  |  Sync: %s%s",
			m.lastSync.Format("15:04:05"), filterTag)
		b.WriteString("\n" + StyleStatusBar.Render(shortcuts))
	}

	return b.String()
}

func (m Model) viewLogs() string {
	var b strings.Builder

	glyph, statusText, statusStyle := FormatStatus(m.selectedState, 0, 0)
	statusBadge := statusStyle.Render(fmt.Sprintf("%s %s", glyph, statusText))

	breadcrumb := fmt.Sprintf("[<] Volver (Esc) | Logs: %s | Estado: %s", m.selectedName, statusBadge)
	b.WriteString(StyleTitle.Render(breadcrumb) + "\n\n")

	b.WriteString(m.viewport.View() + "\n\n")
	b.WriteString(StyleStatusBar.Render("[j/k, Up/Down, Scroll]: Scroll  |  [g/G]: Inicio/Fin  |  [Esc/h]: Volver a Flota"))

	return b.String()
}

func (m Model) viewHelp() string {
	var b strings.Builder

	b.WriteString(StyleTitle.Render("SOLV SERVER TRACKER :: ATAJOS DE TECLADO") + "\n\n")
	b.WriteString("  j / Down / ScrollDown : Mover cursor hacia abajo (1 en 1)\n")
	b.WriteString("  k / Up / ScrollUp     : Mover cursor hacia arriba (1 en 1)\n")
	b.WriteString("  /                     : Abrir filtro interactivo en tiempo real\n")
	b.WriteString("  l / Enter             : Abrir visor de logs en vivo a pantalla completa\n")
	b.WriteString("  r                     : Forzar recoleccion inmediata de metricas\n")
	b.WriteString("  Esc / h               : Cerrar visor de logs / cancelar filtro\n")
	b.WriteString("  ?                     : Mostrar / ocultar esta ayuda\n")
	b.WriteString("  q / Ctrl+C            : Salir de la aplicacion\n\n")
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
