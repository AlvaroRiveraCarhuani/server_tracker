package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/core/service"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/ai"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case catalogSyncedMsg:
		return m, nil

	case discoveryMsg:
		m.discoveredConnectors = msg.results
		var allDiscovered []domain.ModelRef
		for _, res := range msg.results {
			if len(res.Models) > 0 {
				allDiscovered = append(allDiscovered, res.Models...)
			}
		}
		if len(allDiscovered) > 0 {
			m.discoveredModels = allDiscovered
		}
		return m, nil

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
		if m.toastVisible {
			m.toastVisible = false
			if !m.themeConfig.OnboardingHintShown {
				m.themeConfig.OnboardingHintShown = true
				if m.vaultService != nil {
					_ = m.vaultService.SaveThemeConfig(m.themeConfig)
				}
			}
		}

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
			switch msg.String() {
			case "esc", "?", "q":
				m.activeState = stateFleetTable
				return m, nil
			case "up", "k":
				if m.overlayScrollOffset > 0 {
					m.overlayScrollOffset--
				}
				return m, nil
			case "down", "j":
				m.overlayScrollOffset++
				return m, nil
			}

		case stateThemeModal:
			switch msg.String() {
			case "esc", "q":
				m.activeState = stateFleetTable
				return m, nil
			case "up", "k":
				if m.themeListCursor > 0 {
					m.themeListCursor--
				}
				return m, nil
			case "down", "j":
				if m.themeListCursor < len(domain.AvailableThemes)-1 {
					m.themeListCursor++
				}
				return m, nil
			case "f", "F":
				m.themeConfig.NerdFonts = !m.themeConfig.NerdFonts
				ApplyTheme(m.themeConfig.ActiveTheme, m.themeConfig.BorderStyle, m.themeConfig.NerdFonts)
				if m.vaultService != nil {
					_ = m.vaultService.SaveThemeConfig(m.themeConfig)
				}
				return m, nil
			case "b", "B":
				switch m.themeConfig.BorderStyle {
				case "double":
					m.themeConfig.BorderStyle = "rounded"
				case "rounded":
					m.themeConfig.BorderStyle = "sharp"
				default:
					m.themeConfig.BorderStyle = "double"
				}
				ApplyTheme(m.themeConfig.ActiveTheme, m.themeConfig.BorderStyle, m.themeConfig.NerdFonts)
				if m.vaultService != nil {
					_ = m.vaultService.SaveThemeConfig(m.themeConfig)
				}
				return m, nil
			case "enter":
				if m.themeListCursor >= 0 && m.themeListCursor < len(domain.AvailableThemes) {
					selTheme := domain.AvailableThemes[m.themeListCursor]
					m.themeConfig.ActiveTheme = selTheme.ID
					m.themeConfig.BorderStyle = selTheme.DefaultBorder
					ApplyTheme(m.themeConfig.ActiveTheme, m.themeConfig.BorderStyle, m.themeConfig.NerdFonts)
					if m.vaultService != nil {
						_ = m.vaultService.SaveThemeConfig(m.themeConfig)
					}
					m.statusMessage = fmt.Sprintf("[OK] Tema activo: %s (%s)", selTheme.Name, strings.ToUpper(m.themeConfig.BorderStyle))
					m.statusExpiry = time.Now().Add(4 * time.Second)
					m.activeState = stateFleetTable
					return m, nil
				}
			}

		case statePreferences:
			switch msg.String() {
			case "esc", "q":
				m.activeState = stateFleetTable
				return m, nil
			case "up", "k":
				if m.preferencesCursor > 0 {
					m.preferencesCursor--
				}
				return m, nil
			case "down", "j":
				if m.preferencesCursor < 1 {
					m.preferencesCursor++
				}
				return m, nil
			case "enter":
				if m.preferencesCursor == 0 {
					m.activeState = stateThemeModal
					m.themeListCursor = 0
					for idx, th := range domain.AvailableThemes {
						if th.ID == m.themeConfig.ActiveTheme {
							m.themeListCursor = idx
							break
						}
					}
					return m, nil
				}
				if m.preferencesCursor == 1 {
					if m.themeConfig.IncidentBannerPolicy == "prudente" {
						m.themeConfig.IncidentBannerPolicy = "informativo"
					} else {
						m.themeConfig.IncidentBannerPolicy = "prudente"
					}
					if m.vaultService != nil {
						_ = m.vaultService.SaveThemeConfig(m.themeConfig)
					}
					return m, nil
				}
				return m, nil
			}

		case stateDiagnosisModal:
			if m.activeIncident != nil {
				containers := m.getIncidentContainersList()
				hasActionRow := (m.activeIncident.ManualOrigin != "" || m.activeIncident.OriginConfidence != domain.ConfidenceUndetermined) && m.activeIncident.EffectiveOrigin() != ""
				maxCursor := len(containers) - 1
				if hasActionRow {
					maxCursor = len(containers)
				}

				switch msg.String() {
				case "esc", "q":
					m.activeIncident = nil
					m.activeState = stateFleetTable
					return m, nil
				case "up", "k":
					if m.v4IncidentCursor > 0 {
						m.v4IncidentCursor--
					}
					if m.v4IncidentCursor < m.overlayScrollOffset {
						m.overlayScrollOffset = m.v4IncidentCursor
					}
					return m, nil
				case "down", "j":
					if m.v4IncidentCursor < maxCursor {
						m.v4IncidentCursor++
					}
					if m.v4IncidentCursor >= m.overlayScrollOffset+4 {
						m.overlayScrollOffset = m.v4IncidentCursor - 3
					}
					return m, nil
				case "o":
					if m.v4IncidentCursor < len(containers) {
						targetName := containers[m.v4IncidentCursor]
						if m.incidentAggregator != nil {
							m.incidentAggregator.SetManualOrigin(m.activeIncident.ID, targetName)
						}
					}
					return m, nil
				case "enter":
					if m.v4IncidentCursor < len(containers) {
						targetName := containers[m.v4IncidentCursor]
						var targetMetric domain.ContainerMetric
						if c, ok := m.activeIncident.Members[targetName]; ok {
							targetMetric = c
						} else {
							for _, c := range m.metrics {
								if strings.TrimPrefix(c.Name, "/") == targetName || c.ID == targetName {
									targetMetric = c
									break
								}
							}
						}
						m.activeIncident = nil
						m.selectedID = targetMetric.ID
						m.selectedName = targetMetric.Name
						m.selectedState = targetMetric.Status
						m.pendingContainer = targetMetric
						m.v4ActionCursor = 0
						m.activeState = stateDiagnosisModal
						return m, nil
					}
					if hasActionRow && m.v4IncidentCursor == len(containers) {
						originName := m.activeIncident.EffectiveOrigin()
						var targetMetric domain.ContainerMetric
						if c, ok := m.activeIncident.Members[originName]; ok {
							targetMetric = c
						} else {
							for _, c := range m.metrics {
								if strings.TrimPrefix(c.Name, "/") == originName || c.ID == originName {
									targetMetric = c
									break
								}
							}
						}
						m.pendingContainer = targetMetric
						m.pendingAction = domain.ActionRestart
						if m.activeIncident.Diagnosis.SuggestedAction == "stop" {
							m.pendingAction = domain.ActionStop
						} else if m.activeIncident.Diagnosis.SuggestedAction == "isolate" {
							m.pendingAction = domain.ActionIsolateNetwork
						}
						m.confirmModalBtn = 0
						m.activeState = stateConfirmRemediation
						return m, nil
					}
					return m, nil
				case "r", "s", "x":
					originName := m.activeIncident.EffectiveOrigin()
					if originName != "" {
						var targetMetric domain.ContainerMetric
						if c, ok := m.activeIncident.Members[originName]; ok {
							targetMetric = c
						} else {
							for _, c := range m.metrics {
								if strings.TrimPrefix(c.Name, "/") == originName || c.ID == originName {
									targetMetric = c
									break
								}
							}
						}
						m.pendingContainer = targetMetric
						switch msg.String() {
						case "r":
							m.pendingAction = domain.ActionRestart
						case "s":
							m.pendingAction = domain.ActionStop
						case "x":
							m.pendingAction = domain.ActionIsolateNetwork
						}
						m.confirmModalBtn = 0
						m.activeState = stateConfirmRemediation
						return m, nil
					}
					return m, nil
				case "i":
					if m.activeIncident != nil && m.activeIncident.DiagnoseCount < 2 {
						m.activeIncident.DiagnoseCount++
						m.statusMessage = "[OK] Solicitando diagnóstico de incidente con IA..."
						m.statusExpiry = time.Now().Add(4 * time.Second)
						return m, m.triggerIncidentTriage(m.activeIncident)
					}
					return m, nil
				}
				return m, nil
			}

			actions := m.GetV4Actions(m.pendingContainer, m.diagnosisResults[m.selectedID])
			evidences := BuildEvidence(m, m.pendingContainer, m.diagnosisResults[m.selectedID], "")
			hasEvScroll := m.height < 30 && len(evidences) > 4

			switch msg.String() {
			case "esc", "q":
				m.v4FocusSection = 0
				m.v4EvidenceScroll = 0
				m.activeState = stateFleetTable
				return m, nil

			case "tab":
				if hasEvScroll {
					if m.v4FocusSection == 0 {
						m.v4FocusSection = 1
					} else {
						m.v4FocusSection = 0
					}
				}
				return m, nil

			case "up", "k":
				if m.v4FocusSection == 1 {
					if m.v4EvidenceScroll > 0 {
						m.v4EvidenceScroll--
					}
				} else {
					if m.v4ActionCursor > 0 {
						m.v4ActionCursor--
					}
					if m.overlayScrollOffset > 0 {
						m.overlayScrollOffset--
					}
				}
				return m, nil

			case "down", "j":
				if m.v4FocusSection == 1 {
					if m.v4EvidenceScroll < len(evidences)-4 {
						m.v4EvidenceScroll++
					}
				} else {
					if len(actions) > 0 && m.v4ActionCursor < len(actions)-1 {
						m.v4ActionCursor++
					}
					m.overlayScrollOffset++
				}
				return m, nil

			case "enter":
				if m.v4FocusSection == 1 {
					m.v4FocusSection = 0
					return m, nil
				}
				if len(actions) > 0 && m.v4ActionCursor < len(actions) {
					act := actions[m.v4ActionCursor]
					if act.IsLogs {
						m.activeState = stateLogViewer
						m.viewport.SetContent("Cargando logs de Docker...")
						return m, m.fetchLogs(m.pendingContainer.ID, m.pendingContainer.Name)
					}
					if act.IsAI {
						m.statusMessage = "[OK] Solicitando diagnóstico con IA..."
						m.statusExpiry = time.Now().Add(4 * time.Second)
						return m, m.triggerTriageForced(m.pendingContainer)
					}
					// D1 CERO RCE: Remediación siempre pasa por el modal de confirmación
					m.pendingAction = act.ActionType
					m.confirmModalBtn = 0
					m.activeState = stateConfirmRemediation
					return m, nil
				}
				return m, nil
			case "r":
				m.pendingAction = domain.ActionRestart
				m.confirmModalBtn = 0
				m.activeState = stateConfirmRemediation
				return m, nil
			case "s":
				m.pendingAction = domain.ActionStop
				m.confirmModalBtn = 0
				m.activeState = stateConfirmRemediation
				return m, nil
			case "x":
				m.pendingAction = domain.ActionIsolateNetwork
				m.confirmModalBtn = 0
				m.activeState = stateConfirmRemediation
				return m, nil
			case "l":
				m.activeState = stateLogViewer
				m.viewport.SetContent("Cargando logs de Docker...")
				return m, m.fetchLogs(m.pendingContainer.ID, m.pendingContainer.Name)
			case "i", "I":
				m.statusMessage = "[OK] Solicitando diagnóstico con IA..."
				m.statusExpiry = time.Now().Add(4 * time.Second)
				return m, m.triggerTriageForced(m.pendingContainer)
			}

		case stateNetworkModal:
			switch msg.String() {
			case "esc", "q", "n", "N":
				m.activeState = stateFleetTable
				return m, nil
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
			switch m.aiState {
			case aiViewPolicy: // V1: Politica de Asignaciones (FAST / DEEP / AUTO)
				switch msg.String() {
				case "esc", "q", "ctrl+c":
					m.activeState = stateFleetTable
					return m, nil
				case "up", "k":
					if m.aiPolicyCursor > 0 {
						m.aiPolicyCursor--
					}
					return m, nil
				case "down", "j":
					if m.aiPolicyCursor < 2 {
						m.aiPolicyCursor++
					}
					return m, nil
				case "p", "P":
					m.aiState = aiViewProviders
					m.aiProvidersCursor = 0
					return m, nil
				case "enter":
					if m.aiPolicyCursor == 0 {
						m.aiTargetSlot = domain.SlotFast
						m.aiState = aiViewModelBrowser
						m.aiSearchActive = false
						m.aiSearchInput.Reset()
						m.aiBrowserCursor = 0
						items := m.getBrowserItems()
						for idx, it := range items {
							if it.isModel {
								m.aiBrowserCursor = idx
								break
							}
						}
						return m, nil
					} else if m.aiPolicyCursor == 1 {
						m.aiTargetSlot = domain.SlotDeep
						m.aiState = aiViewModelBrowser
						m.aiSearchActive = false
						m.aiSearchInput.Reset()
						m.aiBrowserCursor = 0
						items := m.getBrowserItems()
						for idx, it := range items {
							if it.isModel {
								m.aiBrowserCursor = idx
								break
							}
						}
						return m, nil
					}
					// AUTO (cursor == 2) es derivado de solo lectura
					return m, nil
				}

			case aiViewModelBrowser: // V2: Navegador de Modelos
				items := m.getBrowserItems()
				if m.aiSearchActive {
					switch msg.String() {
					case "esc":
						m.aiSearchActive = false
						m.aiSearchInput.Blur()
						return m, nil
					case "enter":
						m.aiSearchActive = false
						m.aiSearchInput.Blur()
						return m, nil
					default:
						var cmd tea.Cmd
						m.aiSearchInput, cmd = m.aiSearchInput.Update(msg)
						m.aiBrowserCursor = 0
						return m, cmd
					}
				}

				switch msg.String() {
				case "esc":
					m.aiState = aiViewPolicy
					return m, nil
				case "/":
					m.aiSearchActive = true
					m.aiSearchInput.Focus()
					return m, textinput.Blink
				case "f", "F":
					m.aiFilterFree = !m.aiFilterFree
					items := m.getBrowserItems()
					m.aiBrowserCursor = 0
					for idx, it := range items {
						if it.isModel {
							m.aiBrowserCursor = idx
							break
						}
					}
					return m, nil
				case "l", "L":
					m.aiFilterLocal = !m.aiFilterLocal
					items := m.getBrowserItems()
					m.aiBrowserCursor = 0
					for idx, it := range items {
						if it.isModel {
							m.aiBrowserCursor = idx
							break
						}
					}
					return m, nil
				case "up", "k":
					if m.aiBrowserCursor > 0 {
						newC := m.aiBrowserCursor - 1
						for newC >= 0 && items[newC].isGroupHeader {
							newC--
						}
						if newC >= 0 {
							m.aiBrowserCursor = newC
						}
					}
					return m, nil
				case "down", "j":
					if m.aiBrowserCursor < len(items)-1 {
						newC := m.aiBrowserCursor + 1
						for newC < len(items) && items[newC].isGroupHeader {
							newC++
						}
						if newC < len(items) {
							m.aiBrowserCursor = newC
						}
					}
					return m, nil
				case "enter":
					if m.aiBrowserCursor >= 0 && m.aiBrowserCursor < len(items) {
						it := items[m.aiBrowserCursor]
						if it.isGroupHeader {
							return m, nil
						}
						if it.isCustomAdd {
							m.aiState = aiViewCustomEndpoint
							m.endpointInput.Reset()
							m.apiKeyInput.Reset()
							m.connectFocusField = 0
							m.endpointInput.Focus()
							m.apiKeyInput.Blur()
							return m, textinput.Blink
						}
						if it.isModel {
							if m.aiConfig.SlotPolicy.Assignments == nil {
								m.aiConfig.SlotPolicy = domain.DefaultSlotPolicy()
							}
							m.aiConfig.SlotPolicy.Assignments[m.aiTargetSlot] = it.model
							m.aiConfig.ActiveProvider = it.model.ProviderID
							m.aiConfig.ActiveModel = it.model.ID

							if m.vaultService != nil {
								_ = m.vaultService.SaveAIConfig(m.aiConfig)
							}
							if tc, ok := m.triageClient.(*ai.TriageClient); ok {
								tc.SetConfig(m.aiConfig)
							}
							m.diagnosisCache = make(map[string]string)
							m.statusMessage = fmt.Sprintf("[OK] Slot %s asignado a %s", strings.ToUpper(string(m.aiTargetSlot)), it.model.DisplayName)
							m.statusExpiry = time.Now().Add(3 * time.Second)

							m.aiState = aiViewPolicy
							return m, nil
						}
					}
				}

			case aiViewProviders: // V3: Proveedores
				rows := m.getProviderRows()
				switch msg.String() {
				case "esc":
					m.aiState = aiViewPolicy
					return m, nil
				case "up", "k":
					if m.aiProvidersCursor > 0 {
						m.aiProvidersCursor--
					}
					return m, nil
				case "down", "j":
					if m.aiProvidersCursor < len(rows)-1 {
						m.aiProvidersCursor++
					}
					return m, nil
				case "r", "R":
					m.statusMessage = "[OK] Escaneando proveedores..."
					m.statusExpiry = time.Now().Add(2 * time.Second)
					return m, m.probeConnectorsCmd()
				case "a", "A":
					m.aiState = aiViewCustomEndpoint
					m.endpointInput.Reset()
					m.apiKeyInput.Reset()
					m.connectFocusField = 0
					m.endpointInput.Focus()
					m.apiKeyInput.Blur()
					return m, textinput.Blink
				case "enter":
					if m.aiProvidersCursor >= 0 && m.aiProvidersCursor < len(rows) {
						r := rows[m.aiProvidersCursor]
						if r.isCustom {
							m.aiState = aiViewCustomEndpoint
							m.endpointInput.Reset()
							m.apiKeyInput.Reset()
							m.connectFocusField = 0
							m.endpointInput.Focus()
							m.apiKeyInput.Blur()
							return m, textinput.Blink
						}
						if r.family == domain.FamilyCloudManaged {
							m.connectProvider = r.providerID
							m.aiState = aiViewKeyInput
							pCfg := m.aiConfig.Providers[r.providerID]
							m.apiKeyInput.Reset()
							m.apiKeyInput.SetValue(pCfg.APIKey)
							m.apiKeyInput.Focus()
							return m, textinput.Blink
						}
						if r.family == domain.FamilyLocalRuntime && r.status == domain.StatusDetected {
							m.aiTargetSlot = domain.SlotFast
							m.aiState = aiViewModelBrowser
							m.aiBrowserCursor = 0
							m.aiFilterLocal = true
							m.aiSearchActive = false
							m.aiSearchInput.Reset()
							return m, nil
						}
					}
				}

			case aiViewKeyInput: // Sub-paso Key Input
				switch msg.String() {
				case "esc":
					m.apiKeyInput.Blur()
					m.aiState = aiViewProviders
					return m, nil
				case "enter":
					keyVal := strings.TrimSpace(m.apiKeyInput.Value())
					prov := m.connectProvider
					pCfg := m.aiConfig.Providers[prov]
					pCfg.APIKey = keyVal
					m.aiConfig.Providers[prov] = pCfg

					if m.vaultService != nil {
						_ = m.vaultService.SaveAIConfig(m.aiConfig)
					}
					if tc, ok := m.triageClient.(*ai.TriageClient); ok {
						tc.SetConfig(m.aiConfig)
					}
					m.diagnosisCache = make(map[string]string)
					m.statusMessage = fmt.Sprintf("[OK] Clave de %s guardada en bóveda", prov)
					m.statusExpiry = time.Now().Add(3 * time.Second)
					m.apiKeyInput.Blur()
					m.aiState = aiViewProviders
					return m, nil
				default:
					var cmd tea.Cmd
					m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
					return m, cmd
				}

			case aiViewCustomEndpoint: // Sub-paso Custom Endpoint
				switch msg.String() {
				case "esc":
					m.endpointInput.Blur()
					m.apiKeyInput.Blur()
					m.aiState = aiViewProviders
					return m, nil
				case "tab", "down", "up":
					m.connectFocusField = (m.connectFocusField + 1) % 2
					if m.connectFocusField == 0 {
						m.endpointInput.Focus()
						m.apiKeyInput.Blur()
					} else {
						m.endpointInput.Blur()
						m.apiKeyInput.Focus()
					}
					return m, textinput.Blink
				case "enter":
					urlVal := strings.TrimSpace(m.endpointInput.Value())
					keyVal := strings.TrimSpace(m.apiKeyInput.Value())
					if urlVal != "" {
						pCfg := m.aiConfig.Providers[domain.ProviderCustom]
						pCfg.Endpoint = urlVal
						pCfg.APIKey = keyVal
						m.aiConfig.Providers[domain.ProviderCustom] = pCfg

						if m.vaultService != nil {
							_ = m.vaultService.SaveAIConfig(m.aiConfig)
						}
						if tc, ok := m.triageClient.(*ai.TriageClient); ok {
							tc.SetConfig(m.aiConfig)
						}
						m.diagnosisCache = make(map[string]string)
						m.statusMessage = "[OK] Endpoint personalizado configurado"
						m.statusExpiry = time.Now().Add(3 * time.Second)
					}
					m.endpointInput.Blur()
					m.apiKeyInput.Blur()
					m.aiState = aiViewProviders
					return m, nil
				default:
					var cmd tea.Cmd
					if m.connectFocusField == 0 {
						m.endpointInput, cmd = m.endpointInput.Update(msg)
					} else {
						m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
					}
					return m, cmd
				}
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
			case "tab":
				switch m.aiConfig.SelectionMode {
				case domain.SelectionAuto:
					m.aiConfig.SelectionMode = domain.SelectionFast
					m.statusMessage = "Modo FAST: inferencia rápida y estructurada"
				case domain.SelectionFast:
					m.aiConfig.SelectionMode = domain.SelectionDeep
					m.statusMessage = "Modo DEEP: análisis profundo con razonamiento"
				case domain.SelectionDeep:
					m.aiConfig.SelectionMode = domain.SelectionManual
					m.statusMessage = "Modo MANUAL: inferencia solo a demanda"
				case domain.SelectionManual:
					m.aiConfig.SelectionMode = domain.SelectionAuto
					m.statusMessage = "Modo AUTO: enrutamiento por severidad"
				default:
					m.aiConfig.SelectionMode = domain.SelectionAuto
					m.statusMessage = "Modo AUTO: enrutamiento por severidad"
				}
				if m.vaultService != nil {
					_ = m.vaultService.SaveAIConfig(m.aiConfig)
				}
				if tc, ok := m.triageClient.(*ai.TriageClient); ok {
					tc.SetConfig(m.aiConfig)
				}
				m.statusExpiry = time.Now().Add(3 * time.Second)
				return m, nil

			case "i", "I":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					c := filtered[m.cursor]
					if m.isAnomalous(c) {
						m.statusMessage = "[OK] Solicitando diagnóstico con IA..."
						m.statusExpiry = time.Now().Add(3 * time.Second)
						if cmd := m.triggerTriageForced(c); cmd != nil {
							cmds = append(cmds, cmd)
						}
						return m, tea.Batch(cmds...)
					}
				}

			case "c":
				m.overlayScrollOffset = 0
				m.activeState = stateConfigModal
				m.aiState = aiViewPolicy
				m.aiPolicyCursor = 0
				return m, nil

			case "d", "D":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					c := filtered[m.cursor]
					m.selectedID = c.ID
					m.selectedName = c.Name
					m.selectedState = c.Status
					m.pendingContainer = c
					m.v4ActionCursor = 0
					m.v4IncidentCursor = 0
					m.overlayScrollOffset = 0

					if m.incidentAggregator != nil {
						m.activeIncident = m.incidentAggregator.GetActiveIncidentFor(c.Name)
						if m.activeIncident == nil {
							m.activeIncident = m.incidentAggregator.GetActiveIncidentFor(c.ID)
						}
					} else {
						m.activeIncident = nil
					}

					m.activeState = stateDiagnosisModal
					if m.activeIncident == nil {
						if cmd := m.triggerTriageIfAnomalous(c); cmd != nil {
							return m, cmd
						}
					}
					return m, nil
				}

			case "n", "N":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					c := filtered[m.cursor]
					m.selectedID = c.ID
					m.selectedName = c.Name
					m.selectedState = c.Status
					m.pendingContainer = c
					m.overlayScrollOffset = 0
					m.activeState = stateNetworkModal
					return m, nil
				}

			case "t":
				m.overlayScrollOffset = 0
				m.activeState = statePreferences
				m.preferencesCursor = 0
				return m, nil

			case "p":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					c := filtered[m.cursor]
					m.togglePin(c.Name)
					if m.isPinned(c.Name) {
						pinGlyph := "󰤱"
						if !m.themeConfig.NerdFonts {
							pinGlyph = "^"
						}
						m.statusMessage = fmt.Sprintf("[%s] Contenedor '%s' fijado al inicio", pinGlyph, c.Name)
					} else {
						m.statusMessage = fmt.Sprintf("[-] Contenedor '%s' desanclado", c.Name)
					}
					m.statusExpiry = time.Now().Add(3 * time.Second)
					return m, nil
				}

			case "P":
				if len(m.pinnedContainers) > 0 {
					m.clearAllPins()
					m.statusMessage = "[OK] Todos los contenedores desanclados"
					m.statusExpiry = time.Now().Add(3 * time.Second)
					return m, nil
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
				m.overlayScrollOffset = 0
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
		// Cachear ÚNICAMENTE si proviene de IA (Nivel 0 [AI] o Nivel 1 [AI~])
		if msg.result.Level == domain.LevelAI || msg.result.Level == domain.LevelAIPartial {
			m.diagnosisCache[msg.containerID] = msg.diagnosis
		}
		if m.diagnosisResults == nil {
			m.diagnosisResults = make(map[string]domain.DiagnosisResult)
		}
		m.diagnosisResults[msg.containerID] = msg.result
		if m.crashJournal != nil {
			m.crashJournal.UpdateDiagnosis(msg.containerID, msg.diagnosis, string(msg.result.Level))
		}
		m.lastDiagnosisUsage[msg.containerID] = msg.usage
		if msg.usage.TotalTokens > 0 {
			m.sessionTokensUsed += msg.usage.TotalTokens
			m.sessionCostUSD += msg.usage.EstimatedCostUSD
		}
		if m.aiMeter != nil && (msg.result.Level == domain.LevelAI || msg.result.Level == domain.LevelAIPartial) {
			if msg.result.ReanalyzedDeep {
				// Contabilizar FAST + DEEP (+2 req) para este evento re-ejecutado
				m.aiMeter.Record(m.aiConfig.ActiveProvider, domain.SlotFast, m.aiConfig.ActiveModel, domain.TokenUsage{}, "", "")
				m.aiMeter.Record(m.aiConfig.ActiveProvider, domain.SlotDeep, m.aiConfig.ActiveModel, msg.usage, "", msg.result.RawOutput)
			} else {
				m.aiMeter.Record(m.aiConfig.ActiveProvider, domain.SlotFast, m.aiConfig.ActiveModel, msg.usage, "", msg.result.RawOutput)
			}
		}
		delete(m.triagePending, msg.containerID)
		return m, nil

	case incidentDiagnosisResultMsg:
		if m.incidentAggregator != nil {
			for _, inc := range m.incidentAggregator.GetAllActiveIncidents() {
				if inc.ID == msg.incidentID {
					inc.Diagnosis = msg.result
					if msg.result.OriginContainer != "" && inc.ManualOrigin == "" {
						inc.CandidateOrigin = msg.result.OriginContainer
					}
					if len(msg.result.Cascade) > 0 {
						inc.Cascade = msg.result.Cascade
					}
					break
				}
			}
		}
		if msg.usage.TotalTokens > 0 {
			m.sessionTokensUsed += msg.usage.TotalTokens
			m.sessionCostUSD += msg.usage.EstimatedCostUSD
		}
		if m.aiMeter != nil {
			m.aiMeter.Record(m.aiConfig.ActiveProvider, domain.SlotDeep, m.aiConfig.ActiveModel, msg.usage, "", msg.result.RawOutput)
		}
		return m, nil

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

			if m.isAnomalous(c) && m.crashJournal != nil {
				m.crashJournal.Record(c, "", "")
			}
		}
		for id := range m.metricsHistory {
			if !activeIDs[id] {
				delete(m.metricsHistory, id)
			}
		}

		if m.incidentAggregator != nil {
			depGraph := service.NewDependencyGraph(m.metrics)
			now := time.Now()
			newOrUpdated := m.incidentAggregator.IngestAnomalies(m.metrics, depGraph, m.crashJournal, now)
			closedIncs := m.incidentAggregator.CheckQuietPeriods(now)

			// Diagnóstico final al cierre del período quieto (Decisión I6)
			for _, inc := range closedIncs {
				if !inc.HasFinalDiagnosis && inc.DiagnoseCount < 2 && m.aiConfig.SelectionMode == domain.SelectionAuto {
					inc.HasFinalDiagnosis = true
					inc.DiagnoseCount++
					cmds = append(cmds, m.triggerIncidentTriage(inc))
				}
			}

			// Diagnóstico de apertura para nuevos incidentes (Decisión I6)
			for _, inc := range newOrUpdated {
				if inc.DiagnoseCount < 1 && m.aiConfig.SelectionMode == domain.SelectionAuto {
					inc.DiagnoseCount++
					cmds = append(cmds, m.triggerIncidentTriage(inc))
				}
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
