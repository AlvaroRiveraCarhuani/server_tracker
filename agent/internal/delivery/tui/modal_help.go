package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// KeyBinding representa la asociación entre una tecla, su descripción y categoría funcional.
type KeyBinding struct {
	Key         string
	Description string
	Category    string // "Navegación", "Acciones", "Modelos de IA", "General"
}

// DefaultKeyBindings es la fuente única de verdad para los atajos de teclado de la TUI.
var DefaultKeyBindings = []KeyBinding{
	// Navegación
	{Key: "↑ / k", Description: "Subir", Category: "Navegación"},
	{Key: "↓ / j", Description: "Bajar", Category: "Navegación"},
	{Key: "p", Description: "Fijar / Desanclar", Category: "Navegación"},
	{Key: "P", Description: "Limpiar fijados", Category: "Navegación"},
	{Key: "Enter / l", Description: "Ver logs", Category: "Navegación"},
	{Key: "/", Description: "Buscar / Filtrar", Category: "Navegación"},

	// Modelos de IA
	{Key: "Tab", Description: "Ciclar modo IA", Category: "Modelos de IA"},
	{Key: "c", Description: "Elegir modelo", Category: "Modelos de IA"},
	{Key: "d", Description: "V4 diagnóstico", Category: "Modelos de IA"},
	{Key: "i", Description: "Solicitar IA", Category: "Modelos de IA"},
	{Key: "n", Description: "V5 red", Category: "Modelos de IA"},

	// Acciones
	{Key: "r", Description: "Reiniciar", Category: "Acciones"},
	{Key: "s", Description: "Detener", Category: "Acciones"},
	{Key: "x", Description: "Aislar de red", Category: "Acciones"},
	{Key: "e", Description: "Abrir terminal", Category: "Acciones"},

	// General
	{Key: "t", Description: "Temas y estilos", Category: "General"},
	{Key: "?", Description: "Ver esta ayuda", Category: "General"},
	{Key: "q / Esc", Description: "Cerrar / Salir", Category: "General"},
}

func (m Model) viewHelp() string {
	modalWidth := 74
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	bgStyle := lipgloss.NewStyle().Background(ColorSurface0)
	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Background(ColorSurface0).Render("Atajos de teclado")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-3)
	header := headerLeft + bgStyle.Render(strings.Repeat(" ", spLen)) + escBadge

	colW := (innerW - 4) / 2
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender).Background(ColorSurface0)
	keyStyle := lipgloss.NewStyle().Foreground(ColorPeach).Background(ColorSurface0).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorText).Background(ColorSurface0)
	dimStyle := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0)

	renderHelpRow := func(kb KeyBinding) string {
		kStr := keyStyle.Render(fmt.Sprintf("  %-11s", kb.Key))
		descW := max(10, colW-13)
		activeDescStyle := descStyle
		if strings.Contains(kb.Description, "(próximamente)") {
			activeDescStyle = dimStyle
		}
		dStr := activeDescStyle.Render(fmt.Sprintf("%-*s", descW, kb.Description))
		return kStr + dStr
	}

	renderSectionHeader := func(title string) string {
		return sectionTitle.Render(fmt.Sprintf("%-*s", colW, title))
	}

	// Agrupar atajos por categoría
	catBindings := make(map[string][]KeyBinding)
	for _, kb := range DefaultKeyBindings {
		catBindings[kb.Category] = append(catBindings[kb.Category], kb)
	}

	// Columna Izquierda: Navegación y Modelos de IA
	var leftLines []string
	leftLines = append(leftLines, renderSectionHeader("Navegación"))
	for _, kb := range catBindings["Navegación"] {
		leftLines = append(leftLines, renderHelpRow(kb))
	}
	leftLines = append(leftLines, bgStyle.Render(strings.Repeat(" ", colW)))
	leftLines = append(leftLines, renderSectionHeader("Modelos de IA"))
	for _, kb := range catBindings["Modelos de IA"] {
		leftLines = append(leftLines, renderHelpRow(kb))
	}

	// Columna Derecha: Acciones y General
	var rightLines []string
	rightLines = append(rightLines, renderSectionHeader("Acciones"))
	for _, kb := range catBindings["Acciones"] {
		rightLines = append(rightLines, renderHelpRow(kb))
	}
	rightLines = append(rightLines, bgStyle.Render(strings.Repeat(" ", colW)))
	rightLines = append(rightLines, renderSectionHeader("General"))
	for _, kb := range catBindings["General"] {
		rightLines = append(rightLines, renderHelpRow(kb))
	}

	sep := bgStyle.Render("    ")
	maxRows := max(len(leftLines), len(rightLines))
	var combinedRows []string
	for i := 0; i < maxRows; i++ {
		l := bgStyle.Render(strings.Repeat(" ", colW))
		if i < len(leftLines) {
			l = leftLines[i]
		}
		r := bgStyle.Render(strings.Repeat(" ", colW))
		if i < len(rightLines) {
			r = rightLines[i]
		}
		combinedRows = append(combinedRows, l+sep+r)
	}
	centeredColumns := strings.Join(combinedRows, "\n")

	body := fmt.Sprintf("%s\n\n%s", header, centeredColumns)
	return StyleModal.Width(modalWidth).Render(body)
}
