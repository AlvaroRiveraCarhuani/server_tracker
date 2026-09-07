package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Paleta Oficial Catppuccin Mocha con Mayor Contraste y Brillo
var (
	ColorBase     = lipgloss.Color("#1e1e2e")
	ColorMantle   = lipgloss.Color("#181825")
	ColorCrust    = lipgloss.Color("#11111b")
	ColorSurface0 = lipgloss.Color("#313244")
	ColorSurface1 = lipgloss.Color("#45475a")
	ColorSurface2 = lipgloss.Color("#585b70")
	ColorOverlay0 = lipgloss.Color("#6c7086")
	ColorOverlay2 = lipgloss.Color("#9399b2")
	ColorSubtext0 = lipgloss.Color("#a6adc8")
	ColorSubtext1 = lipgloss.Color("#bac2de")
	ColorText     = lipgloss.Color("#cdd6f4")
	ColorLavender = lipgloss.Color("#b4befe")
	ColorMauve    = lipgloss.Color("#cba6f7")
	ColorGreen    = lipgloss.Color("#a6e3a1")
	ColorYellow   = lipgloss.Color("#f9e2af")
	ColorPeach    = lipgloss.Color("#fab387")
	ColorRed      = lipgloss.Color("#f38ba8")
	ColorBlue     = lipgloss.Color("#89b4fa")
	ColorTeal     = lipgloss.Color("#94e2d5")
)

// Estilos Lip Gloss Precomputados
var (
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBase).
			Background(ColorLavender).
			Padding(0, 1)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorMauve).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(ColorSurface2)

	StyleRowFocus = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Background(ColorSurface0)

	StyleStatusRunning = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	StyleStatusStopped = lipgloss.NewStyle().Foreground(ColorOverlay2)
	StyleStatusPaused  = lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)
	StyleStatusCritical = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)

	StyleEgressNormal = lipgloss.NewStyle().Foreground(ColorSubtext0)
	StyleEgressMedium = lipgloss.NewStyle().Foreground(ColorText)
	StyleEgressWarm   = lipgloss.NewStyle().Foreground(ColorPeach).Bold(true)
	StyleEgressDanger = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)

	StyleStatusBar = lipgloss.NewStyle().
			Foreground(ColorSubtext1).
			Background(ColorMantle).
			Padding(0, 1)

	StyleFilterPrompt = lipgloss.NewStyle().
				Foreground(ColorMauve).
				Bold(true)
)

// FormatStatus renderiza los glifos ASCII con semántica cromática estricta y alto contraste
func FormatStatus(status string, ramBytes, ramLimit uint64) (glyph, text string, style lipgloss.Style) {
	// Detección predictiva de riesgo OOM al 85%
	if ramLimit > 0 && status == "running" {
		usageRatio := float64(ramBytes) / float64(ramLimit)
		if usageRatio >= 0.85 {
			return "[!!]", "OOM_RISK", StyleStatusCritical
		}
	}

	switch strings.ToLower(status) {
	case "running":
		return "[OK]", "RUNNING", StyleStatusRunning
	case "exited", "stopped":
		return "[--]", "STOPPED", StyleStatusStopped
	case "paused":
		return "[||]", "PAUSED", StyleStatusPaused
	case "restarting", "dead":
		return "[!!]", strings.ToUpper(status), StyleStatusCritical
	default:
		return "[?]", strings.ToUpper(status), StyleStatusStopped
	}
}

// FormatEgress aplica el semáforo financiero y alineación de red
func FormatEgress(bytesSec float64) (formatted string, style lipgloss.Style) {
	kbSec := bytesSec / 1024.0
	mbSec := kbSec / 1024.0

	var numStr string
	if mbSec >= 1.0 {
		numStr = fmt.Sprintf("%.2f MB/s", mbSec)
	} else {
		numStr = fmt.Sprintf("%.1f KB/s", kbSec)
	}

	if bytesSec < 500*1024 { // < 500 KB/s
		return numStr, StyleEgressNormal
	} else if bytesSec < 5*1024*1024 { // 500 KB/s - 5 MB/s
		return numStr, StyleEgressMedium
	} else if bytesSec < 50*1024*1024 { // 5 MB/s - 50 MB/s
		return numStr, StyleEgressWarm
	}
	// > 50 MB/s (Riesgo Financiero)
	return numStr, StyleEgressDanger
}
