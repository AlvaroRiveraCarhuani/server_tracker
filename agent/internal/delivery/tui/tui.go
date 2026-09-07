package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/ports"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Catppuccin Mocha Color Palette
var (
	colorBase     = lipgloss.Color("#1e1e2e")
	colorSurface  = lipgloss.Color("#313244")
	colorOverlay  = lipgloss.Color("#45475a")
	colorText     = lipgloss.Color("#cdd6f4")
	colorSubtext  = lipgloss.Color("#a6adc8")
	colorLavender = lipgloss.Color("#b4befe")
	colorMauve    = lipgloss.Color("#cba6f7")
	colorGreen    = lipgloss.Color("#a6e3a1")
	colorPeach    = lipgloss.Color("#fab387")
	colorRed      = lipgloss.Color("#f38ba8")
	colorTeal     = lipgloss.Color("#94e2d5")
)

// Estilos Lip Gloss
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBase).
			Background(colorLavender).
			Padding(0, 1).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMauve).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorSurface)

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorLavender).
				Background(colorSurface)

	runningStyle = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	stoppedStyle = lipgloss.NewStyle().Foreground(colorRed)
	pausedStyle  = lipgloss.NewStyle().Foreground(colorPeach)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			MarginTop(1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSurface).
			Padding(0, 1)
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Model representa el estado de la vista Bubbletea.
type Model struct {
	collector ports.CollectorPort
	metrics   []domain.ContainerMetric
	cursor    int
	lastError string
	lastSync  time.Time
	width     int
	height    int
}

// NewModel inicializa el modelo de la TUI.
func NewModel(collector ports.CollectorPort) Model {
	return Model{
		collector: collector,
		cursor:    0,
		lastSync:  time.Now(),
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "j", "down":
			if len(m.metrics) > 0 {
				m.cursor = (m.cursor + 1) % len(m.metrics)
			}
		case "k", "up":
			if len(m.metrics) > 0 {
				m.cursor = (m.cursor - 1 + len(m.metrics)) % len(m.metrics)
			}
		case "r":
			return m, m.fetchMetrics()
		}

	case tickMsg:
		// Límite estricto de 2 FPS para telemetría
		return m, tea.Batch(m.fetchMetrics(), tickCmd())

	case []domain.ContainerMetric:
		m.metrics = msg
		m.lastSync = time.Now()
		m.lastError = ""
		if m.cursor >= len(m.metrics) && len(m.metrics) > 0 {
			m.cursor = len(m.metrics) - 1
		}

	case error:
		m.lastError = msg.Error()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("🛰️  SOLV SERVER TRACKER  [TUI Deep Work]"))
	b.WriteString("\n")

	if m.lastError != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("⚠️ Error: %s\n", m.lastError)))
	}

	// Tabla de Contenedores
	headers := fmt.Sprintf("%-14s %-22s %-10s %-9s %-12s %-12s %-6s",
		"ID", "CONTENEDOR", "ESTADO", "CPU %", "RAM (MB)", "EGRESS (KB/s)", "PIDS")
	b.WriteString(headerStyle.Render(headers))
	b.WriteString("\n")

	if len(m.metrics) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(colorSubtext).Render("  Buscando contenedores en /var/run/docker.sock...\n"))
	} else {
		for i, c := range m.metrics {
			statusStr := c.Status
			switch c.Status {
			case "running":
				statusStr = runningStyle.Render("● running")
			case "exited":
				statusStr = stoppedStyle.Render("○ stopped")
			case "paused":
				statusStr = pausedStyle.Render("⏸ paused")
			}

			ramMB := float64(c.RAMBytes) / (1024 * 1024)
			egressKB := c.EgressBytesSec / 1024.0

			row := fmt.Sprintf("%-14s %-22s %-19s %-8.1f%% %-12.1f %-12.2f %-6d",
				c.ID,
				truncate(c.Name, 20),
				statusStr,
				c.CPUPercent,
				ramMB,
				egressKB,
				c.PIDs,
			)

			if i == m.cursor {
				b.WriteString(selectedRowStyle.Render("▶ " + row))
			} else {
				b.WriteString("  " + row)
			}
			b.WriteString("\n")
		}
	}

	// Footer / Atajos
	shortcuts := "[j/k, ↑/↓]: Navegar  •  [r]: Refrescar  •  [q]: Salir  •  Última sincro: " + m.lastSync.Format("15:04:05")
	b.WriteString(statusBarStyle.Render(shortcuts))
	b.WriteString("\n")

	return cardStyle.Render(b.String())
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

// RunTUI inicia el programa interactivo Bubbletea.
func RunTUI(collector ports.CollectorPort) error {
	p := tea.NewProgram(NewModel(collector), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
