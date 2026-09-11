package tui

import (
	"fmt"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/service"
	"github.com/charmbracelet/lipgloss"
)

// viewNetworkModal renderiza la ficha de red V5 centrada como overlay modal (solo lectura).
func (m Model) viewNetworkModal() string {
	c := m.pendingContainer
	if c.ID == "" && len(m.metrics) > 0 {
		c = m.metrics[0]
	}

	modalWidth := 74
	if m.width > 20 && m.width-4 < modalWidth {
		modalWidth = m.width - 4
	}
	innerW := modalWidth - 6

	bgStyle := lipgloss.NewStyle().Background(ColorSurface0)
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Background(ColorSurface0).Render(fmt.Sprintf("red · %s", c.Name))
	escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Background(ColorSurface0).Render("esc")
	spLen := max(1, innerW-lipgloss.Width(headerLeft)-3)
	header := headerLeft + bgStyle.Render(strings.Repeat(" ", spLen)) + escBadge

	var lines []string

	// 1. Redes, IP y Alias
	netStr := "--"
	if len(c.Networks) > 0 {
		netStr = strings.Join(c.Networks, ", ")
	}
	ipStr := "--"
	if c.IPAddress != "" {
		ipStr = c.IPAddress
	}
	aliasStr := "--"
	if len(c.NetworkAliases) > 0 {
		aliasStr = strings.Join(c.NetworkAliases, ", ")
	}

	metaPrefix := fmt.Sprintf("redes: %s · ip %s · alias: ", netStr, ipStr)
	var metaLine string
	if lipgloss.Width(metaPrefix)+lipgloss.Width(aliasStr) > innerW {
		avail := innerW - lipgloss.Width(metaPrefix)
		if avail > 4 {
			metaLine = metaPrefix + truncate(aliasStr, avail-3) + "..."
		} else {
			metaLine = truncate(metaPrefix+aliasStr, innerW-3) + "..."
		}
	} else {
		metaLine = metaPrefix + aliasStr
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(ColorText).Render(metaLine), "")

	// 2. Puertos Publicados
	graph := service.NewDependencyGraph(m.metrics)
	ports := graph.GetPublishedPorts(c)
	conflicts := graph.GetPortConflicts(c)

	lines = append(lines, lipgloss.NewStyle().Foreground(ColorLavender).Bold(true).Render("puertos publicados:"))
	if len(ports) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render("  --"))
	} else {
		for _, p := range ports {
			portMap := fmt.Sprintf("  %d -> %s:%d", p.ContainerPort, p.HostIP, p.HostPort)
			var badge string
			if owner, ok := conflicts[p.HostPort]; ok {
				badge = lipgloss.NewStyle().Foreground(ColorYellow).Render(fmt.Sprintf("[||] en conflicto con: %s", owner))
			} else if p.IsLoopback() {
				badge = lipgloss.NewStyle().Foreground(ColorGreen).Render("[OK] loopback")
			} else {
				badge = lipgloss.NewStyle().Foreground(ColorYellow).Render("[||] expuesto")
			}

			pad := innerW - lipgloss.Width(portMap) - lipgloss.Width(badge)
			var row string
			if pad > 1 {
				row = portMap + strings.Repeat(" ", pad) + badge
			} else {
				row = portMap + "  " + badge
			}
			lines = append(lines, row)
		}
	}
	lines = append(lines, "")

	// 3. Dependencias
	deps := graph.GetDependencies(c)
	depStr := "--"
	if len(deps) > 0 {
		depStr = strings.Join(deps, ", ") + " (inferido)"
	}
	depLabel := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("%-18s", "depende de:"))
	depVal := lipgloss.NewStyle().Foreground(ColorText).Render(depStr)
	lines = append(lines, fmt.Sprintf("%s %s", depLabel, depVal))

	dependents := graph.GetDependents(c)
	depMeStr := "--"
	if len(dependents) > 0 {
		depMeStr = strings.Join(dependents, ", ") + " (inferido)"
	}
	depMeLabel := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("%-18s", "dependen de mi:"))
	depMeVal := lipgloss.NewStyle().Foreground(ColorText).Render(depMeStr)
	lines = append(lines, fmt.Sprintf("%s %s", depMeLabel, depMeVal))

	// 4. Radio de Impacto
	_, impactText := graph.GetImpactRadius(c)
	impactLabel := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("%-18s", "radio de impacto:"))
	impactVal := lipgloss.NewStyle().Foreground(ColorText).Render(impactText)
	lines = append(lines, fmt.Sprintf("%s %s", impactLabel, impactVal))

	// 5. Egress con semáforo financiero
	egressStr, egressStyle := FormatEgress(c.EgressBytesSec)
	var egressGlyph string
	if c.EgressBytesSec >= 50*1024*1024 {
		egressGlyph = lipgloss.NewStyle().Foreground(ColorRed).Bold(true).Render("[!!]")
	} else if c.EgressBytesSec >= 5*1024*1024 {
		egressGlyph = lipgloss.NewStyle().Foreground(ColorYellow).Render("[||]")
	} else {
		egressGlyph = lipgloss.NewStyle().Foreground(ColorGreen).Render("[OK]")
	}
	egressLabel := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("%-18s", "egress:"))
	lines = append(lines, fmt.Sprintf("%s %-16s %s", egressLabel, egressStyle.Render(egressStr), egressGlyph))

	// 6. Pie de modal
	lines = append(lines, "", lipgloss.NewStyle().Foreground(ColorSubtext0).Render("esc: volver"))

	body := fmt.Sprintf("%s\n\n%s", header, strings.Join(lines, "\n"))
	return StyleModal.Width(modalWidth).Render(body)
}

// BuildNetworkModalContent expone el renderizado para tests independientes de UI.
func BuildNetworkModalContent(m Model, c domain.ContainerMetric) string {
	m.pendingContainer = c
	return m.viewNetworkModal()
}
