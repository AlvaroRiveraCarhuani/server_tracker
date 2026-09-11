package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/service"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

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
		return overlayModal(baseView, m.viewAIModal(), m.width, m.height)
	}

	if m.activeState == stateThemeModal {
		return overlayModal(baseView, m.viewThemeModal(), m.width, m.height)
	}

	if m.activeState == stateDiagnosisModal {
		return overlayModal(baseView, m.viewDiagnosisModal(), m.width, m.height)
	}

	if m.activeState == stateNetworkModal {
		return overlayModal(baseView, m.viewNetworkModal(), m.width, m.height)
	}

	return baseView
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

		auxLines := 5
		if m.lastError != "" {
			auxLines++
		}
		if len(filtered) > 0 && m.cursor < len(filtered) && m.isAnomalous(filtered[m.cursor]) {
			auxLines += 3
		}
		if m.activeState == stateFiltering {
			auxLines++
		}
		panelHeight := max(4, m.height-auxLines)

		// Panel Izquierdo: Lista de Contenedores (Master)
		var leftContent strings.Builder
		innerLeftW := leftWidth - 4 // Ancho libre dentro de StyleCard (borde + padding)
		nameW := max(8, innerLeftW-14)

		renderRow := func(i int, c domain.ContainerMetric, pinned bool) string {
			glyph, _, statusStyle := FormatStatus(c.Status, c.RAMBytes, c.RAMLimitBytes)
			tech := DetectTechnology(c.Image, c.Name)

			nameStr := truncate(c.Name, nameW)
			namePadded := fmt.Sprintf("%-*s", nameW, nameStr)

			pinIcon := " "
			if pinned {
				if NerdFontsMode {
					pinIcon = "󰤱"
				} else {
					pinIcon = "^"
				}
			}

			techGlyph := tech.NerdGlyph
			if !NerdFontsMode {
				techGlyph = "*"
			}

			cpuStr := fmt.Sprintf("%3.0f%%", c.CPUPercent)

			if i == m.cursor {
				bg := ColorSurface0
				bgStyle := lipgloss.NewStyle().Background(bg)

				var prefixStyled string
				if pinned {
					prefixStyled = lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Background(bg).Render(">" + pinIcon)
				} else {
					prefixStyled = lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Background(bg).Render("> ")
				}

				statusGlyphStyled := statusStyle.Background(bg).Render(glyph)
				midSp1 := bgStyle.Render(" ")
				techGlyphStyled := lipgloss.NewStyle().Foreground(tech.Color).Background(bg).Render(techGlyph)
				midSp2 := bgStyle.Render(" ")
				nameStyled := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Background(bg).Render(namePadded)
				midSp3 := bgStyle.Render(" ")

				cpuColor := ColorSubtext0
				if c.CPUPercent >= 80 {
					cpuColor = ColorRed
				} else if c.CPUPercent >= 50 {
					cpuColor = ColorPeach
				}
				cpuStyle := lipgloss.NewStyle().Foreground(cpuColor).Background(bg)
				if c.CPUPercent >= 80 {
					cpuStyle = cpuStyle.Bold(true)
				}
				cpuStyled := cpuStyle.Render(cpuStr)

				rowLine := prefixStyled + statusGlyphStyled + midSp1 + techGlyphStyled + midSp2 + nameStyled + midSp3 + cpuStyled
				curW := lipgloss.Width(rowLine)
				if curW < innerLeftW {
					rowLine += bgStyle.Render(strings.Repeat(" ", innerLeftW-curW))
				}
				return rowLine + "\n"
			}

			// Fila normal (sin foco)
			var prefixStyled string
			if pinned {
				prefixStyled = lipgloss.NewStyle().Foreground(ColorPeach).Render(" " + pinIcon)
			} else {
				prefixStyled = lipgloss.NewStyle().Foreground(ColorOverlay2).Render("  ")
			}
			nameStyled := lipgloss.NewStyle().Foreground(ColorSubtext1).Render(namePadded)

			var cpuStyled string
			if c.CPUPercent >= 80 {
				cpuStyled = lipgloss.NewStyle().Foreground(ColorRed).Bold(true).Render(cpuStr)
			} else if c.CPUPercent >= 50 {
				cpuStyled = lipgloss.NewStyle().Foreground(ColorPeach).Render(cpuStr)
			} else {
				cpuStyled = lipgloss.NewStyle().Foreground(ColorSubtext0).Render(cpuStr)
			}
			techGlyphStyled := lipgloss.NewStyle().Foreground(tech.Color).Render(techGlyph)

			rowLine := fmt.Sprintf("%s%s %s %s %s", prefixStyled, statusStyle.Render(glyph), techGlyphStyled, nameStyled, cpuStyled)
			return lipgloss.NewStyle().Width(innerLeftW).Render(rowLine) + "\n"
		}

		total := len(filtered)
		numPinned := 0
		for _, c := range filtered {
			if m.isPinned(c.Name) {
				numPinned++
			}
		}

		if total == 0 {
			leftContent.WriteString(StyleCardTitle.Render("CONTENEDORES (0)") + "\n\n")
			if m.filterValue != "" {
				leftContent.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("Sin resultados para '%s'", m.filterValue)))
			} else {
				leftContent.WriteString(lipgloss.NewStyle().Foreground(ColorSubtext0).Render("Escaneando socket..."))
			}
		} else if numPinned > 0 {
			leftContent.WriteString(StyleCardTitle.Render(fmt.Sprintf("FIJADOS (%d)", numPinned)) + "\n\n")

			availRows := max(3, panelHeight-3)
			pinnedCount := min(numPinned, max(1, availRows/2))
			unpinnedRows := max(1, availRows-pinnedCount)

			pinStart := 0
			if m.cursor < numPinned && m.cursor >= pinnedCount {
				pinStart = m.cursor - pinnedCount + 1
			}
			pinEnd := min(numPinned, pinStart+pinnedCount)
			if pinEnd-pinStart < pinnedCount {
				pinStart = max(0, pinEnd-pinnedCount)
			}

			for i := pinStart; i < pinEnd; i++ {
				leftContent.WriteString(renderRow(i, filtered[i], true))
			}

			if total > numPinned {
				leftContent.WriteString(lipgloss.NewStyle().Foreground(ColorOverlay2).Render(strings.Repeat("─", innerLeftW)) + "\n")

				unpinnedTotal := total - numPinned
				unpStart := 0
				if m.cursor >= numPinned {
					curInUnp := m.cursor - numPinned
					if curInUnp >= unpinnedRows {
						unpStart = curInUnp - unpinnedRows + 1
					}
				}
				unpEnd := min(unpinnedTotal, unpStart+unpinnedRows)
				if unpEnd-unpStart < unpinnedRows {
					unpStart = max(0, unpEnd-unpinnedRows)
				}

				for u := unpStart; u < unpEnd; u++ {
					idx := numPinned + u
					leftContent.WriteString(renderRow(idx, filtered[idx], false))
				}
			}
		} else {
			maxVisibleRows := max(3, panelHeight-2)
			start := 0
			if m.cursor >= maxVisibleRows {
				start = m.cursor - maxVisibleRows + 1
			}
			end := min(total, start+maxVisibleRows)
			if end-start < maxVisibleRows {
				start = max(0, end-maxVisibleRows)
			}

			leftHeader := fmt.Sprintf("CONTENEDORES (%d)", total)
			if total > maxVisibleRows {
				leftHeader = fmt.Sprintf("CONTENEDORES (%d) • %d-%d", total, start+1, end)
			}
			leftContent.WriteString(StyleCardTitle.Render(leftHeader) + "\n\n")

			for i := start; i < end; i++ {
				leftContent.WriteString(renderRow(i, filtered[i], false))
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

			pinnedBadge := ""
			if m.isPinned(sel.Name) {
				if NerdFontsMode {
					pinnedBadge = " " + lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render("[󰤱 FIJADO]")
				} else {
					pinnedBadge = " " + lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render("[^ FIJADO]")
				}
			}

			// Header del contenedor
			headerLine := fmt.Sprintf("%s  %s  %s%s", tech.Badge(), lipgloss.NewStyle().Bold(true).Foreground(ColorText).Render(sel.Name), statusBadge, pinnedBadge)
			rightContent.WriteString(headerLine + "\n")

			subInfo := lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("  Imagen: %s  |  ID: %s  |  Categoría: %s", sel.Image, sel.ID, tech.Category))
			rightContent.WriteString(subInfo + "\n\n")

			// 1. CICLO DE VIDA
			rightContent.WriteString(StyleCardTitle.Render("CICLO DE VIDA") + "\n")
			stateDesc := sel.Status
			if !sel.LastStateChange.IsZero() {
				stateDesc = fmt.Sprintf("%s · %s", sel.Status, formatDurationAgo(time.Since(sel.LastStateChange)))
			}
			rightContent.WriteString(fmt.Sprintf("  state: %s\n", stateDesc))

			policyStr := sel.RestartPolicy
			if policyStr == "" {
				policyStr = "--"
			}
			rightContent.WriteString(fmt.Sprintf("  restarts: %d en ciclo · policy: %s\n\n", sel.RestartCount, policyStr))

			// 2. VITALES
			rightContent.WriteString(StyleCardTitle.Render("VITALES") + "\n")
			cpuThrottlingStr := fmt.Sprintf("throttling %.0f%%", sel.CPUPercentThrottled)
			cpuGlyph := "[OK]"
			if sel.CPUPercent > 80.0 {
				cpuGlyph = "[!!]"
			} else if sel.CPUPercentThrottled > 0.0 {
				cpuGlyph = "[||]"
			}
			rightContent.WriteString(fmt.Sprintf("  CPU: %5.1f%% · %-18s %s\n", sel.CPUPercent, cpuThrottlingStr, cpuGlyph))

			// RAM
			ramMB := float64(sel.RAMBytes) / (1024 * 1024)
			if sel.RAMLimitBytes > 0 {
				limitMB := float64(sel.RAMLimitBytes) / (1024 * 1024)
				pct := (ramMB / limitMB) * 100.0
				ramTag := "[OK]"
				if pct >= 85.0 {
					ramTag = "[!!] riesgo OOM"
				}
				rightContent.WriteString(fmt.Sprintf("  RAM: %.0f/%.0fMB (%.0f%%)               %s\n", ramMB, limitMB, pct, ramTag))
			} else {
				rightContent.WriteString(fmt.Sprintf("  RAM: %.1f MB (Sin límite Docker)    [OK]\n", ramMB))
			}

			// RED
			egressStr, egressStyle := FormatEgress(sel.EgressBytesSec)
			rightContent.WriteString(fmt.Sprintf("  RED: egress %-16s       [OK]\n", egressStyle.Render(egressStr)))

			// Trend
			hist := m.metricsHistory[sel.ID]
			if hist != nil && len(hist.CPU) > 0 {
				trend := hist.CalculateCPUTrend()
				spark := RenderSparkline(hist.CPU, 0, 100, 8)
				rightContent.WriteString(fmt.Sprintf("  trend: %s  %s\n\n", spark, trend.Style.Render(trend.Label)))
			} else {
				rightContent.WriteString("  trend: --  estable\n\n")
			}

			// 3. DIAGNÓSTICO
			rightContent.WriteString(StyleCardTitle.Render("DIAGNÓSTICO") + "\n")
			if res, ok := m.diagnosisResults[sel.ID]; ok {
				var tagStr string
				switch res.Level {
				case domain.LevelAI:
					tagStr = StyleTagAI.Render("[AI]")
				case domain.LevelAIPartial:
					tagStr = StyleTagAIPartial.Render("[AI~]")
				case domain.LevelRule:
					tagStr = StyleTagRule.Render("[RULE]")
				default:
					tagStr = StyleTagSignal.Render("[SIG]")
				}
				cause := res.RootCause
				if len(cause) > 35 {
					cause = cause[:32] + "..."
				}
				detailHint := lipgloss.NewStyle().Foreground(ColorLavender).Render("[d] detalle")
				rightContent.WriteString(fmt.Sprintf("  %s %s · %s\n\n", tagStr, cause, detailHint))
			} else {
				solicitarHint := lipgloss.NewStyle().Foreground(ColorLavender).Render("[d] solicitar")
				rightContent.WriteString(fmt.Sprintf("  [--] sin diagnóstico · %s\n\n", solicitarHint))
			}

			// 4. CONTEXTO
			rightContent.WriteString(StyleCardTitle.Render("CONTEXTO") + "\n")
			netStr := "--"
			if len(sel.Networks) > 0 {
				netStr = strings.Join(sel.Networks, ", ")
			}
			if sel.ComposeProject != "" {
				rightContent.WriteString(fmt.Sprintf("  compose: %s · net: %s\n", sel.ComposeProject, netStr))
			} else {
				rightContent.WriteString(fmt.Sprintf("  net: %s\n", netStr))
			}

			portsStr := "--"
			if len(sel.Ports) > 0 {
				portsStr = strings.Join(sel.Ports, ", ")
			}
			rightContent.WriteString(fmt.Sprintf("  ports: %s · vols: %d\n", portsStr, sel.VolumeCount))

			depGraph := service.NewDependencyGraph(m.metrics)
			dependents := depGraph.GetDependents(sel)
			var depLine string
			if len(dependents) == 0 {
				depLine = "  dependen de mi: --\n"
			} else if len(dependents) <= 2 {
				depLine = fmt.Sprintf("  dependen de mi: %s\n", strings.Join(dependents, ", "))
			} else {
				depLine = fmt.Sprintf("  dependen de mi: %d · [n] detalle\n", len(dependents))
			}
			rightContent.WriteString(depLine)
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
			var prefixStyled, idStyled, nameStyled string
			if i == m.cursor {
				prefixStyled = lipgloss.NewStyle().Foreground(ColorPeach).Bold(true).Render("> ")
				idStyled = lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(fmt.Sprintf("%-14s", c.ID))
				nameStyled = lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(fmt.Sprintf("%-20s", truncate(c.Name, 18)))
			} else {
				prefixStyled = "  "
				idStyled = lipgloss.NewStyle().Foreground(ColorSubtext1).Render(fmt.Sprintf("%-14s", c.ID))
				nameStyled = lipgloss.NewStyle().Foreground(ColorSubtext1).Render(fmt.Sprintf("%-20s", truncate(c.Name, 18)))
			}
			line := fmt.Sprintf("%s%s %s %s", prefixStyled, idStyled, nameStyled, statusStyle.Render(fmt.Sprintf("%s %s", glyph, statusText)))
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
			res, hasRes := m.diagnosisResults[selected.ID]
			var tagStyled string
			var diagText string

			if hasRes {
				diagText = res.RootCause
				switch res.Level {
				case domain.LevelAI:
					tagStyled = StyleTagAI.Render("[AI]")
				case domain.LevelAIPartial:
					tagStyled = StyleTagAIPartial.Render("[AI~]")
				case domain.LevelRule:
					tagStyled = StyleTagRule.Render("[RULE]")
				default:
					tagStyled = StyleTagSignal.Render("[SIG]")
				}
			} else {
				diag, cached := m.diagnosisCache[selected.ID]
				if cached {
					tagStyled = StyleTagAI.Render("[AI]")
					diagText = diag
				} else if m.triagePending[selected.ID] {
					tagStyled = StyleTagAI.Render("[AI]")
					diagText = "Analizando causa raíz con IA..."
				} else {
					tagStyled = StyleTagSignal.Render("[SIG]")
					diagText = "Pendiente de diagnóstico analítico..."
				}
			}

			usage, hasUsage := m.lastDiagnosisUsage[selected.ID]
			usageBadge := ""
			if hasUsage && usage.TotalTokens > 0 {
				usageBadge = lipgloss.NewStyle().Foreground(ColorSubtext0).Render(fmt.Sprintf("  [%d tok • ~$%.4f]", usage.TotalTokens, usage.EstimatedCostUSD))
			}

			if strings.Contains(diagText, "no configurado") {
				diagText = "Diagnóstico no configurado · [c]"
			}

			// En modo MANUAL, si el banner es [RULE] o [SIG], agregar hint explícito para inferir con IA
			manualHint := ""
			if m.aiConfig.SelectionMode == domain.SelectionManual && (!hasRes || res.Level == domain.LevelRule || res.Level == domain.LevelSignal) {
				if m.width <= 90 {
					manualHint = lipgloss.NewStyle().Foreground(ColorLavender).Render(" · [i] IA")
				} else {
					manualHint = lipgloss.NewStyle().Foreground(ColorLavender).Render(" · [i] solicitar diagnóstico IA")
				}
			}

			detailHint := ""
			if hasRes && res.RecurrenceCount >= 3 {
				detailHint = lipgloss.NewStyle().Foreground(ColorSubtext0).Render(" · [d] detalle")
			}

			bannerWidth := max(40, m.width-2)
			rawBannerLen := lipgloss.Width(tagStyled) + 1 + lipgloss.Width(diagText) + lipgloss.Width(manualHint) + lipgloss.Width(detailHint) + lipgloss.Width(usageBadge)
			if rawBannerLen > bannerWidth {
				if hasRes && res.RecurrenceCount >= 3 {
					recTag := fmt.Sprintf("recurrente (%d/1h)", res.RecurrenceCount)
					if strings.Contains(diagText, recTag) {
						baseText := strings.TrimSpace(strings.Replace(diagText, recTag, "", 1))
						availForBase := bannerWidth - lipgloss.Width(tagStyled) - 1 - lipgloss.Width(recTag) - 1 - lipgloss.Width(detailHint) - lipgloss.Width(manualHint) - lipgloss.Width(usageBadge) - 3
						if availForBase > 5 {
							baseText = truncate(baseText, availForBase)
							diagText = fmt.Sprintf("%s %s", baseText, recTag)
						}
					} else {
						avail := bannerWidth - lipgloss.Width(tagStyled) - lipgloss.Width(manualHint) - lipgloss.Width(detailHint) - lipgloss.Width(usageBadge) - 3
						if avail > 10 {
							diagText = truncate(diagText, avail)
						}
					}
				} else {
					avail := bannerWidth - lipgloss.Width(tagStyled) - lipgloss.Width(manualHint) - lipgloss.Width(detailHint) - lipgloss.Width(usageBadge) - 3
					if avail > 10 {
						diagText = truncate(diagText, avail)
					}
				}
			}

			bannerContent := fmt.Sprintf("%s %s%s%s%s", tagStyled, diagText, detailHint, manualHint, usageBadge)
			b.WriteString(StyleAIOpsBanner.Width(bannerWidth).Render(bannerContent) + "\n")
		}
	}

	// Barra inferior / Filtro
	if m.activeState == stateFiltering {
		b.WriteString(m.filterInput.View())
	} else {
		if m.statusMessage != "" && time.Now().Before(m.statusExpiry) {
			b.WriteString(StyleStatusBar.Render(m.statusMessage))
		} else {
			filterTag := ""
			if m.filterValue != "" {
				filterTag = fmt.Sprintf(" [Filtro: '%s']", m.filterValue)
			}
			aiStatsTag := ""
			if m.sessionTokensUsed > 0 {
				aiStatsTag = fmt.Sprintf("  |  %d tok (~$%.3f)", m.sessionTokensUsed, m.sessionCostUSD)
			}

			// Token V0 de Modo AIOps
			modeToken := ""
			switch m.aiConfig.SelectionMode {
			case domain.SelectionAuto:
				modeToken = "[Tab] modo: AUTO (deriva por severidad)"
			case domain.SelectionFast:
				fastM := domain.GetAssignedModel(domain.SlotFast, m.aiConfig.SlotPolicy)
				disp := fastM.DisplayName
				if disp == "" {
					disp = fastM.ID
				}
				modeToken = fmt.Sprintf("[Tab] modo: FAST · %s (%s)", disp, modelTag(fastM))
			case domain.SelectionDeep:
				deepM := domain.GetAssignedModel(domain.SlotDeep, m.aiConfig.SlotPolicy)
				disp := deepM.DisplayName
				if disp == "" {
					disp = deepM.ID
				}
				modeToken = fmt.Sprintf("[Tab] modo: DEEP · %s (%s)", disp, modelTag(deepM))
			case domain.SelectionManual:
				modeToken = fmt.Sprintf("[Tab] modo: MANUAL · %s", m.aiConfig.ActiveModel)
			default:
				modeToken = "[Tab] modo: AUTO (deriva por severidad)"
			}

			shortcuts := fmt.Sprintf("%s  |  [j/k]: Navegar  |  [p]: Fijar  |  [l/Enter]: Logs  |  [c]: IA  |  [t]: Temas  |  [/]: Filtro%s%s", modeToken, filterTag, aiStatsTag)
			b.WriteString(StyleStatusBar.Render(shortcuts))
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
