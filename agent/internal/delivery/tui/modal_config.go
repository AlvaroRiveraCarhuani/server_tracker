package tui

import (
	"fmt"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/charmbracelet/lipgloss"
)

type configViewMode int

const (
	configViewSelectModel configViewMode = iota
	configViewConnectKey
	configViewCustomModel
)

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

		escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Render("esc")
		headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender).Render(fmt.Sprintf("Configurar %s", meta.Name))
		spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
		header := headerLeft + strings.Repeat(" ", spLen) + escBadge

		keyBorderColor := ColorSurface2
		if m.connectFocusField == 0 {
			keyBorderColor = ColorMauve
		}
		keyBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(keyBorderColor).
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
			Padding(0, 1).
			Width(innerW - 4).
			Render(m.endpointInput.View())

		masked := pCfg.MaskedKey()
		statusInfo := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("Estado actual: %s", lipgloss.NewStyle().Foreground(ColorPeach).Render(masked)))

		saveBtn := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGreen).
			Background(ColorGreen).
			Foreground(ColorBase).
			Bold(true).
			Padding(0, 3).
			Render("✔  GUARDAR (Enter)")

		centeredSave := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Render(saveBtn)

		body := fmt.Sprintf("%s\n\n%s\n%s\n\n%s\n\n%s", header, keyBox, epBox, statusInfo, centeredSave)
		return StyleModal.Width(modalWidth).Render(body)

	case configViewCustomModel:
		escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Render("esc")
		headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorLavender).Render("Modelo personalizado")
		spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
		header := headerLeft + strings.Repeat(" ", spLen) + escBadge

		inputBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMauve).
			Padding(0, 1).
			Width(innerW - 4).
			Render(m.customModelInput.View())

		saveBtn := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGreen).
			Background(ColorGreen).
			Foreground(ColorBase).
			Bold(true).
			Padding(0, 3).
			Render("✔  ACTIVAR (Enter)")

		centeredSave := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Render(saveBtn)

		body := fmt.Sprintf("%s\n\n%s\n\n%s", header, inputBox, centeredSave)
		return StyleModal.Width(modalWidth).Render(body)

	default: // configViewSelectModel (Estilo OpenCode)
		escBadge := lipgloss.NewStyle().Foreground(ColorSubtext0).Render("esc")
		headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorText).Render("Select model")
		spLen := max(1, innerW-lipgloss.Width(headerLeft)-lipgloss.Width(escBadge))
		header := headerLeft + strings.Repeat(" ", spLen) + escBadge

		searchLine := m.modelSearchInput.View()

		items := m.getFilteredModelItems()
		maxVisible := max(6, m.height-12)
		start := 0
		if m.modelListCursor >= maxVisible {
			start = m.modelListCursor - maxVisible + 1
		}
		end := min(len(items), start+maxVisible)
		if end-start < maxVisible {
			start = max(0, end-maxVisible)
		}

		var lines []string

		for i := start; i < end; i++ {
			it := items[i]
			if it.isHeader {
				headerStyle := lipgloss.NewStyle().Foreground(ColorMauve).Bold(true)
				lines = append(lines, headerStyle.Render(it.headerTitle))
				continue
			}

			if it.isCustom {
				customLabel := "+ Custom Model ID..."
				if i == m.modelListCursor {
					customStyled := lipgloss.NewStyle().
						Foreground(ColorPeach).
						Bold(true).
						Width(innerW).
						Render("  " + customLabel)
					lines = append(lines, customStyled)
				} else {
					lines = append(lines, lipgloss.NewStyle().Foreground(ColorSubtext0).Render("  "+customLabel))
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
					Foreground(ColorPeach).
					Bold(true).
					Width(innerW).
					Render(rowText)
				lines = append(lines, rowStyled)
			} else {
				rowStyled := lipgloss.NewStyle().
					Foreground(ColorText).
					Width(innerW).
					Render(rowText)
				lines = append(lines, rowStyled)
			}
		}

		listBlock := strings.Join(lines, "\n")
		sep := lipgloss.NewStyle().Foreground(ColorSurface1).Render(strings.Repeat("─", innerW))

		footer := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(
			"Enter: Select   Ctrl+A: Connect provider   Esc: Close",
		)

		body := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n%s", header, searchLine, listBlock, sep, footer)
		return StyleModal.Width(modalWidth).Render(body)
	}
}
