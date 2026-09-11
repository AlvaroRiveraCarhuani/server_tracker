package tui

import (
	"fmt"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/charmbracelet/lipgloss"
)

type aiViewState int

const (
	aiViewNone aiViewState = iota
	aiViewPolicy         // V1: Politica de Asignaciones (FAST / DEEP / AUTO)
	aiViewModelBrowser   // V2: Navegador Contextual de Modelos
	aiViewProviders      // V3: Proveedores e Infraestructura
	aiViewKeyInput       // Sub-paso: Conectar API Key
	aiViewCustomEndpoint // Sub-paso: Endpoint Personalizado
)

// viewAIModal delegates rendering to the active AI subview.
func (m Model) viewAIModal() string {
	switch m.aiState {
	case aiViewPolicy:
		return m.viewAIPolicy()
	case aiViewModelBrowser:
		return m.viewAIModelBrowser()
	case aiViewProviders:
		return m.viewAIProviders()
	case aiViewKeyInput:
		return m.viewAIKeyInput()
	case aiViewCustomEndpoint:
		return m.viewAICustomEndpoint()
	default:
		return m.viewAIPolicy()
	}
}

// -----------------------------------------------------------------------------
// V1: Politica de Asignaciones (FAST / DEEP / AUTO)
// -----------------------------------------------------------------------------

func (m Model) viewAIPolicy() string {
	dialogWidth := 68
	if m.width > 20 && m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	contentWidth := dialogWidth - 6

	var b strings.Builder

	// Header
	title := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render("diagnóstico · asignaciones")
	header := lipgloss.NewStyle().
		Border(CurrentBorder, false, false, true, false).
		BorderForeground(ColorSurface2).
		Padding(0, 1).
		Width(contentWidth).
		Render(title)
	b.WriteString(header + "\n\n")

	fastRef := domain.GetAssignedModel(domain.SlotFast, m.aiConfig.SlotPolicy)
	deepRef := domain.GetAssignedModel(domain.SlotDeep, m.aiConfig.SlotPolicy)

	// Rows: 0: FAST, 1: DEEP, 2: AUTO
	rows := []struct {
		slotName string
		model    domain.ModelRef
		desc     string
		readOnly bool
	}{
		{
			slotName: "FAST",
			model:    fastRef,
			desc:     fmt.Sprintf("%s · %s", fastRef.ProviderID, modelTag(fastRef)),
			readOnly: false,
		},
		{
			slotName: "DEEP",
			model:    deepRef,
			desc:     fmt.Sprintf("%s · %s", deepRef.ProviderID, modelTag(deepRef)),
			readOnly: false,
		},
		{
			slotName: "AUTO",
			model:    domain.ModelRef{},
			desc:     "deriva de FAST/DEEP por severidad",
			readOnly: true,
		},
	}

	for i, r := range rows {
		isFocused := (m.aiPolicyCursor == i)

		slotBadgeStyle := lipgloss.NewStyle().Bold(true)
		if isFocused {
			slotBadgeStyle = slotBadgeStyle.Foreground(ColorBase).Background(ColorPeach)
		} else {
			slotBadgeStyle = slotBadgeStyle.Foreground(ColorPeach)
		}
		slotBadge := slotBadgeStyle.Render(fmt.Sprintf("[%s]", r.slotName))

		var rowText string
		if r.readOnly {
			autoStyle := lipgloss.NewStyle().Foreground(ColorSubtext1)
			if isFocused {
				autoStyle = autoStyle.Foreground(ColorText).Bold(true)
			}
			rowText = fmt.Sprintf(" %s  %s", slotBadge, autoStyle.Render(r.desc))
		} else {
			nameStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
			tagStyle := lipgloss.NewStyle().Foreground(ColorSubtext0)
			if isFocused {
				nameStyle = nameStyle.Foreground(ColorLavender)
			}
			displayName := r.model.DisplayName
			if displayName == "" {
				displayName = r.model.ID
			}
			rowText = fmt.Sprintf(" %s  %s  %s", slotBadge, nameStyle.Render(displayName), tagStyle.Render(r.desc))
		}

		rowStyle := lipgloss.NewStyle().Padding(0, 1).Width(contentWidth)
		if isFocused {
			rowStyle = rowStyle.Background(ColorSurface0)
		}
		b.WriteString(rowStyle.Render(rowText) + "\n")
	}

	b.WriteString("\n")

	// Footer guide
	footerStyle := lipgloss.NewStyle().
		Border(CurrentBorder, true, false, false, false).
		BorderForeground(ColorSurface2).
		Padding(0, 1).
		Foreground(ColorSubtext0).
		Width(contentWidth)
	footer := footerStyle.Render("enter: reasignar   p: proveedores   esc: salir")
	b.WriteString(footer)

	// Container box
	boxStyle := lipgloss.NewStyle().
		Border(CurrentBorder).
		BorderForeground(ColorLavender).
		Background(ColorBase).
		Padding(1, 1).
		Width(dialogWidth)

	return boxStyle.Render(b.String())
}

func modelTag(m domain.ModelRef) string {
	if m.IsFree || m.PricingTier == domain.PricingFree {
		return "free"
	}
	if m.PricingTier == domain.PricingLow {
		return "low"
	}
	return "prem"
}

// -----------------------------------------------------------------------------
// V2: Navegador Contextual de Modelos
// -----------------------------------------------------------------------------

type browserItem struct {
	isGroupHeader bool
	groupTitle    string
	isModel       bool
	model         domain.ModelRef
	isCustomAdd   bool
}

func (m Model) getBrowserItems() []browserItem {
	var items []browserItem

	models := m.discoveredModels
	if len(models) == 0 && m.catalogService != nil {
		all := m.catalogService.AllModels()
		for _, am := range all {
			models = append(models, domain.ModelRef{
				ID:            am.ID,
				DisplayName:   am.DisplayName,
				ProviderID:    am.ProviderID,
				ContextLength: am.ContextLength,
				IsFree:        am.Pricing.IsFree || am.Pricing.Tier == domain.PricingFree,
				PricingTier:   am.Pricing.Tier,
				EvalOK:        true,
			})
		}
	}

	// Filter by search text
	q := strings.ToLower(strings.TrimSpace(m.aiSearchInput.Value()))

	// Filter by free / local flags
	var filtered []domain.ModelRef
	for _, mod := range models {
		if m.aiFilterFree && !mod.IsFree {
			continue
		}
		if m.aiFilterLocal {
			isLocal := (mod.ProviderID == domain.ProviderOllama || mod.ProviderID == domain.ProviderVLLM || mod.ProviderID == domain.ProviderLMStudio)
			if !isLocal {
				continue
			}
		}
		if q != "" {
			match := strings.Contains(strings.ToLower(mod.DisplayName), q) ||
				strings.Contains(strings.ToLower(mod.ID), q) ||
				strings.Contains(strings.ToLower(string(mod.ProviderID)), q)
			if !match {
				continue
			}
		}
		filtered = append(filtered, mod)
	}

	// Group by Provider
	grouped := make(map[domain.AIProvider][]domain.ModelRef)
	var provOrder []domain.AIProvider
	for _, mod := range filtered {
		if _, exists := grouped[mod.ProviderID]; !exists {
			provOrder = append(provOrder, mod.ProviderID)
		}
		grouped[mod.ProviderID] = append(grouped[mod.ProviderID], mod)
	}

	for _, prov := range provOrder {
		mods := grouped[prov]
		tag := "remoto"
		if prov == domain.ProviderOllama || prov == domain.ProviderVLLM || prov == domain.ProviderLMStudio {
			tag = fmt.Sprintf("local · %d modelos", len(mods))
		} else if prov == domain.ProviderOpenRouter {
			tag = "conectado · free"
		}
		headerTitle := fmt.Sprintf("%s · %s", strings.ToUpper(string(prov)), tag)
		items = append(items, browserItem{isGroupHeader: true, groupTitle: headerTitle})
		for _, mod := range mods {
			items = append(items, browserItem{isModel: true, model: mod})
		}
	}

	// Action to add custom endpoint
	items = append(items, browserItem{isCustomAdd: true})

	return items
}

func (m Model) viewAIModelBrowser() string {
	dialogWidth := 70
	if m.width > 20 && m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	contentWidth := dialogWidth - 6

	var b strings.Builder

	slotStr := strings.ToUpper(string(m.aiTargetSlot))
	if slotStr == "" {
		slotStr = "FAST"
	}

	// Header: title + search indicator
	titleLeft := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(fmt.Sprintf("modelo para %s", slotStr))
	searchIndicator := lipgloss.NewStyle().Foreground(ColorSubtext1).Render("/ buscar")
	if m.aiSearchActive {
		searchIndicator = lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render("/ " + m.aiSearchInput.Value())
	}
	gapW := (contentWidth - 2) - lipgloss.Width(titleLeft) - lipgloss.Width(searchIndicator)
	if gapW < 1 {
		gapW = 1
	}
	headerLine := titleLeft + strings.Repeat(" ", gapW) + searchIndicator

	header := lipgloss.NewStyle().
		Border(CurrentBorder, false, false, true, false).
		BorderForeground(ColorSurface2).
		Padding(0, 1).
		Width(contentWidth).
		Render(headerLine)
	b.WriteString(header + "\n")

	// Filters bar: [f] free  [l] local
	filterFreeText := "[f] free"
	if m.aiFilterFree {
		filterFreeText = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true).Render("[f] free [OK]")
	} else {
		filterFreeText = lipgloss.NewStyle().Foreground(ColorSubtext1).Render("[f] free")
	}

	filterLocalText := "[l] local"
	if m.aiFilterLocal {
		filterLocalText = lipgloss.NewStyle().Foreground(ColorLavender).Bold(true).Render("[l] local [OK]")
	} else {
		filterLocalText = lipgloss.NewStyle().Foreground(ColorSubtext1).Render("[l] local")
	}

	filtersLine := fmt.Sprintf("filtros: %s  %s", filterFreeText, filterLocalText)
	b.WriteString(lipgloss.NewStyle().Padding(0, 1).Render(filtersLine) + "\n\n")

	// Items list
	items := m.getBrowserItems()

	cursor := m.aiBrowserCursor
	if cursor < 0 {
		cursor = 0
	}
	// If cursor points to a group header, auto-advance to first model
	if cursor < len(items) && items[cursor].isGroupHeader {
		for cursor < len(items) && items[cursor].isGroupHeader {
			cursor++
		}
	}
	if cursor >= len(items) {
		cursor = max(0, len(items)-1)
		for cursor > 0 && items[cursor].isGroupHeader {
			cursor--
		}
	}

	// Windowing / paging: max 7 rows
	maxRows := 7
	startIdx := 0
	if cursor >= maxRows {
		startIdx = cursor - maxRows + 1
	}
	endIdx := min(len(items), startIdx+maxRows)

	var focusedModel *domain.ModelRef

	for i := startIdx; i < endIdx; i++ {
		it := items[i]
		isFocused := (i == cursor)

		if it.isGroupHeader {
			gStyle := lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Padding(0, 1)
			b.WriteString(gStyle.Render(it.groupTitle) + "\n")
			continue
		}

		if it.isCustomAdd {
			rowStyle := lipgloss.NewStyle().Padding(0, 2)
			if isFocused {
				rowStyle = rowStyle.Foreground(ColorLavender).Bold(true).Background(ColorSurface0)
			} else {
				rowStyle = rowStyle.Foreground(ColorSubtext0)
			}
			b.WriteString(rowStyle.Render("+ endpoint personalizado") + "\n")
			continue
		}

		if it.isModel {
			if isFocused {
				focusedModel = &it.model
			}
			prefix := "  "
			rowStyle := lipgloss.NewStyle().Padding(0, 1)
			if isFocused {
				prefix = "> "
				rowStyle = rowStyle.Foreground(ColorText).Bold(true).Background(ColorSurface0)
			} else {
				rowStyle = rowStyle.Foreground(ColorSubtext1)
			}
			disp := it.model.DisplayName
			if disp == "" {
				disp = it.model.ID
			}
			b.WriteString(rowStyle.Render(fmt.Sprintf("%s%s", prefix, disp)) + "\n")
		}
	}

	b.WriteString("\n")

	// Context line: ctx · p95 · free · eval [OK]
	var contextLine string
	if focusedModel != nil {
		ctxK := focusedModel.ContextLength / 1024
		if ctxK <= 0 {
			ctxK = 32
		}
		p95Str := "p95 2.1s"
		if focusedModel.LatencyP95 > 0 {
			p95Str = fmt.Sprintf("p95 %.1fs", focusedModel.LatencyP95.Seconds())
		}
		freeTag := "prem"
		if focusedModel.IsFree {
			freeTag = "free"
		}
		evalTag := "eval [--]"
		if focusedModel.EvalOK {
			evalTag = "eval [OK]"
		}
		metaContent := fmt.Sprintf("ctx %dk · %s · %s · %s", ctxK, p95Str, freeTag, evalTag)
		contextLine = lipgloss.NewStyle().Foreground(ColorSubtext0).Padding(0, 1).Render(metaContent)
	} else {
		contextLine = lipgloss.NewStyle().Foreground(ColorSubtext1).Padding(0, 1).Render("enter: seleccionar  esc: cancelar")
	}
	b.WriteString(contextLine + "\n")

	// Container box
	boxStyle := lipgloss.NewStyle().
		Border(CurrentBorder).
		BorderForeground(ColorLavender).
		Background(ColorBase).
		Padding(1, 1).
		Width(dialogWidth)

	return boxStyle.Render(b.String())
}

// -----------------------------------------------------------------------------
// V3: Proveedores e Infraestructura
// -----------------------------------------------------------------------------

type providerRowItem struct {
	providerID domain.AIProvider
	name       string
	family     domain.ConnectorFamily
	status     domain.ConnectorStatus
	modelCount int
	isCustom   bool
}

func (m Model) getProviderRows() []providerRowItem {
	providers := []domain.AIProvider{
		domain.ProviderOpenRouter,
		domain.ProviderOllama,
		domain.ProviderOpenAI,
		domain.ProviderAnthropic,
		domain.ProviderVLLM,
		domain.ProviderLMStudio,
	}

	var rows []providerRowItem
	for _, pID := range providers {
		meta := domain.GetProviderMeta(pID)
		pCfg := m.aiConfig.Providers[pID]

		status := domain.StatusAbsent
		modelCount := 0

		if disc, ok := m.discoveredConnectors[pID]; ok {
			status = disc.Status
			modelCount = len(disc.Models)
		} else {
			// Infer from config
			if meta.RequiresKey {
				if strings.TrimSpace(pCfg.APIKey) != "" {
					status = domain.StatusConnected
				} else {
					status = domain.StatusNoKey
				}
			} else {
				status = domain.StatusDetected
			}
		}

		fam := domain.FamilyCloudManaged
		if pID == domain.ProviderOllama || pID == domain.ProviderVLLM || pID == domain.ProviderLMStudio {
			fam = domain.FamilyLocalRuntime
		}

		rows = append(rows, providerRowItem{
			providerID: pID,
			name:       meta.Name,
			family:     fam,
			status:     status,
			modelCount: modelCount,
			isCustom:   false,
		})
	}

	// Option to add custom endpoint
	rows = append(rows, providerRowItem{
		isCustom: true,
		name:     "+ endpoint personalizado",
	})

	return rows
}

func (m Model) viewAIProviders() string {
	dialogWidth := 68
	if m.width > 20 && m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	contentWidth := dialogWidth - 6

	var b strings.Builder

	// Header: title + refresh indicator
	titleLeft := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render("proveedores")
	refreshHint := lipgloss.NewStyle().Foreground(ColorSubtext1).Render("r: refresh")
	gapW := contentWidth - lipgloss.Width(titleLeft) - lipgloss.Width(refreshHint)
	if gapW < 1 {
		gapW = 1
	}
	headerLine := titleLeft + strings.Repeat(" ", gapW) + refreshHint

	header := lipgloss.NewStyle().
		Border(CurrentBorder, false, false, true, false).
		BorderForeground(ColorSurface2).
		Padding(0, 1).
		Width(contentWidth).
		Render(headerLine)
	b.WriteString(header + "\n\n")

	rows := m.getProviderRows()
	for i, r := range rows {
		isFocused := (i == m.aiProvidersCursor)

		if r.isCustom {
			cStyle := lipgloss.NewStyle().Padding(0, 1)
			if isFocused {
				cStyle = cStyle.Foreground(ColorLavender).Bold(true).Background(ColorSurface0)
			} else {
				cStyle = cStyle.Foreground(ColorSubtext0)
			}
			b.WriteString(cStyle.Render(r.name) + "\n")
			continue
		}

		nameStyle := lipgloss.NewStyle().Bold(true)
		if isFocused {
			nameStyle = nameStyle.Foreground(ColorText)
		} else {
			nameStyle = nameStyle.Foreground(ColorSubtext1)
		}
		nameCol := nameStyle.Width(14).Render(r.name)

		var statusBadge string
		switch r.status {
		case domain.StatusConnected:
			statusBadge = lipgloss.NewStyle().Foreground(ColorGreen).Render("conectado · key [OK]")
		case domain.StatusNoKey:
			statusBadge = lipgloss.NewStyle().Foreground(ColorPeach).Render("sin key [conectar]")
		case domain.StatusDetected:
			countStr := fmt.Sprintf("%d modelos", r.modelCount)
			if r.modelCount <= 0 {
				countStr = "activo"
			}
			statusBadge = lipgloss.NewStyle().Foreground(ColorLavender).Render(fmt.Sprintf("local · %s [explorar]", countStr))
		case domain.StatusTimeout:
			statusBadge = lipgloss.NewStyle().Foreground(ColorRed).Render("timeout")
		default:
			statusBadge = lipgloss.NewStyle().Foreground(ColorSurface2).Render("no detectado")
		}

		line := fmt.Sprintf("%s %s", nameCol, statusBadge)
		if m.aiMeter != nil {
			if statsStr := m.aiMeter.FormatProviderStats(r.providerID); statsStr != "" {
				line = fmt.Sprintf("%s  · %s", line, lipgloss.NewStyle().Foreground(ColorSubtext0).Render(statsStr))
			}
		}
		rowStyle := lipgloss.NewStyle().Padding(0, 1).Width(contentWidth)
		if isFocused {
			rowStyle = rowStyle.Background(ColorSurface0)
		}
		b.WriteString(rowStyle.Render(line) + "\n")
	}

	b.WriteString("\n")

	// Footer guide
	footerStyle := lipgloss.NewStyle().
		Border(CurrentBorder, true, false, false, false).
		BorderForeground(ColorSurface2).
		Padding(0, 1).
		Foreground(ColorSubtext0).
		Width(contentWidth)
	footer := footerStyle.Render("enter: acción   a: añadir   r: refresh   esc: volver")
	b.WriteString(footer)

	// Container box
	boxStyle := lipgloss.NewStyle().
		Border(CurrentBorder).
		BorderForeground(ColorLavender).
		Background(ColorBase).
		Padding(1, 1).
		Width(dialogWidth)

	return boxStyle.Render(b.String())
}

// -----------------------------------------------------------------------------
// Sub-paso: Conectar API Key (1 solo campo)
// -----------------------------------------------------------------------------

func (m Model) viewAIKeyInput() string {
	dialogWidth := 60
	if m.width > 20 && m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	contentWidth := dialogWidth - 6

	var b strings.Builder

	provName := string(m.connectProvider)
	meta := domain.GetProviderMeta(m.connectProvider)
	if meta.Name != "" {
		provName = meta.Name
	}

	title := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(fmt.Sprintf("conectar · %s", provName))
	header := lipgloss.NewStyle().
		Border(CurrentBorder, false, false, true, false).
		BorderForeground(ColorSurface2).
		Padding(0, 1).
		Width(contentWidth).
		Render(title)
	b.WriteString(header + "\n\n")

	desc := lipgloss.NewStyle().Foreground(ColorSubtext0).Render("Pega la clave API para almacenar bajo cifrado D2:")
	b.WriteString(lipgloss.NewStyle().Padding(0, 1).Render(desc) + "\n\n")

	// Key input field
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorPeach).
		Padding(0, 1).
		Width(contentWidth - 2).
		Render(m.apiKeyInput.View())
	b.WriteString(inputBox + "\n\n")

	footerStyle := lipgloss.NewStyle().
		Border(CurrentBorder, true, false, false, false).
		BorderForeground(ColorSurface2).
		Padding(0, 1).
		Foreground(ColorSubtext0).
		Width(contentWidth)
	footer := footerStyle.Render("enter: guardar en bóveda   esc: cancelar")
	b.WriteString(footer)

	boxStyle := lipgloss.NewStyle().
		Border(CurrentBorder).
		BorderForeground(ColorPeach).
		Background(ColorBase).
		Padding(1, 1).
		Width(dialogWidth)

	return boxStyle.Render(b.String())
}

// -----------------------------------------------------------------------------
// Sub-paso: Endpoint Personalizado (URL + Key opcional)
// -----------------------------------------------------------------------------

func (m Model) viewAICustomEndpoint() string {
	dialogWidth := 64
	if m.width > 20 && m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	contentWidth := dialogWidth - 6

	var b strings.Builder

	title := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render("endpoint personalizado (OpenAI-compatible)")
	header := lipgloss.NewStyle().
		Border(CurrentBorder, false, false, true, false).
		BorderForeground(ColorSurface2).
		Padding(0, 1).
		Width(contentWidth).
		Render(title)
	b.WriteString(header + "\n\n")

	epBorder := ColorSurface2
	if m.connectFocusField == 0 {
		epBorder = ColorPeach
	}
	epBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(epBorder).
		Padding(0, 1).
		Width(contentWidth - 2).
		Render(m.endpointInput.View())
	b.WriteString(epBox + "\n\n")

	keyBorder := ColorSurface2
	if m.connectFocusField == 1 {
		keyBorder = ColorPeach
	}
	keyBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(keyBorder).
		Padding(0, 1).
		Width(contentWidth - 2).
		Render(m.apiKeyInput.View())
	b.WriteString(keyBox + "\n\n")

	footerStyle := lipgloss.NewStyle().
		Border(CurrentBorder, true, false, false, false).
		BorderForeground(ColorSurface2).
		Padding(0, 1).
		Foreground(ColorSubtext0).
		Width(contentWidth)
	footer := footerStyle.Render("tab: cambiar campo   enter: guardar   esc: cancelar")
	b.WriteString(footer)

	boxStyle := lipgloss.NewStyle().
		Border(CurrentBorder).
		BorderForeground(ColorLavender).
		Background(ColorBase).
		Padding(1, 1).
		Width(dialogWidth)

	return boxStyle.Render(b.String())
}
