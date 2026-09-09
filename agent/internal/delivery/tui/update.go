package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
	"github.com/alvaroriverac/server_tracker_agent/internal/infrastructure/ai"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

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
			items := m.getFilteredModelItems()
			switch m.configMode {
			case configViewSelectModel:
				switch msg.String() {
				case "esc", "ctrl+c":
					m.modelSearchInput.Blur()
					m.activeState = stateFleetTable
					return m, nil
				case "up", "ctrl+p", "ctrl+k":
					if len(items) > 0 {
						newCursor := m.modelListCursor - 1
						for newCursor >= 0 && items[newCursor].isHeader {
							newCursor--
						}
						if newCursor >= 0 {
							m.modelListCursor = newCursor
						}
					}
					return m, nil
				case "down", "ctrl+n", "ctrl+j":
					if len(items) > 0 {
						newCursor := m.modelListCursor + 1
						for newCursor < len(items) && items[newCursor].isHeader {
							newCursor++
						}
						if newCursor < len(items) {
							m.modelListCursor = newCursor
						}
					}
					return m, nil
				case "ctrl+a":
					// Conectar / editar API Key del ítem seleccionado o activo
					selectedProv := m.aiConfig.ActiveProvider
					if m.modelListCursor >= 0 && m.modelListCursor < len(items) {
						it := items[m.modelListCursor]
						if !it.isHeader && !it.isCustom {
							selectedProv = it.option.Provider
						}
					}
					m.connectProvider = selectedProv
					m.configMode = configViewConnectKey
					m.connectFocusField = 0
					pCfg := m.aiConfig.Providers[selectedProv]
					m.apiKeyInput.Reset()
					m.apiKeyInput.SetValue(pCfg.APIKey)
					m.endpointInput.Reset()
					m.endpointInput.SetValue(pCfg.Endpoint)
					m.apiKeyInput.Focus()
					m.endpointInput.Blur()
					return m, textinput.Blink
				case "enter":
					if m.modelListCursor >= 0 && m.modelListCursor < len(items) {
						it := items[m.modelListCursor]
						if it.isHeader {
							return m, nil
						}
						if it.isCustom {
							m.configMode = configViewCustomModel
							m.customModelInput.Reset()
							m.customModelInput.SetValue(m.aiConfig.ActiveModel)
							m.customModelInput.Focus()
							return m, textinput.Blink
						}
						// Modelo de catálogo
						prov := it.option.Provider
						pCfg := m.aiConfig.Providers[prov]
						meta := domain.GetProviderMeta(prov)

						// Si requiere key y no tiene, pasar directo a conectar key
						if meta.RequiresKey && strings.TrimSpace(pCfg.APIKey) == "" {
							m.connectProvider = prov
							m.configMode = configViewConnectKey
							m.connectFocusField = 0
							m.apiKeyInput.Reset()
							m.endpointInput.Reset()
							m.endpointInput.SetValue(pCfg.Endpoint)
							m.apiKeyInput.Focus()
							m.endpointInput.Blur()
							m.statusMessage = fmt.Sprintf("[INFO] %s requiere clave API para activarse", meta.Name)
							m.statusExpiry = time.Now().Add(3 * time.Second)
							return m, textinput.Blink
						}

						// Activar modelo y proveedor
						m.aiConfig.ActiveProvider = prov
						m.aiConfig.ActiveModel = it.option.ID
						pCfg.DefaultModel = it.option.ID
						m.aiConfig.Providers[prov] = pCfg
						if m.vaultService != nil {
							_ = m.vaultService.SaveAIConfig(m.aiConfig)
						}
						m.triageClient = ai.NewTriageClientWithConfig(m.aiConfig)
						m.diagnosisCache = make(map[string]string)
						m.statusMessage = fmt.Sprintf("[OK] Modelo activo: %s (%s)", it.option.DisplayName, meta.Name)
						m.statusExpiry = time.Now().Add(4 * time.Second)
						m.activeState = stateFleetTable
						return m, nil
					}
				default:
					var cmd tea.Cmd
					m.modelSearchInput, cmd = m.modelSearchInput.Update(msg)
					// Reajustar cursor al primer ítem seleccionable si la lista cambia
					newItems := m.getFilteredModelItems()
					if m.modelListCursor >= len(newItems) {
						m.modelListCursor = 0
					}
					for m.modelListCursor < len(newItems) && newItems[m.modelListCursor].isHeader {
						m.modelListCursor++
					}
					return m, cmd
				}

			case configViewConnectKey:
				switch msg.String() {
				case "esc":
					m.apiKeyInput.Blur()
					m.endpointInput.Blur()
					m.configMode = configViewSelectModel
					m.modelSearchInput.Focus()
					return m, textinput.Blink
				case "tab", "down":
					m.connectFocusField = (m.connectFocusField + 1) % 2
					if m.connectFocusField == 0 {
						m.apiKeyInput.Focus()
						m.endpointInput.Blur()
					} else {
						m.apiKeyInput.Blur()
						m.endpointInput.Focus()
					}
					return m, textinput.Blink
				case "shift+tab", "up":
					m.connectFocusField = (m.connectFocusField + 1) % 2
					if m.connectFocusField == 0 {
						m.apiKeyInput.Focus()
						m.endpointInput.Blur()
					} else {
						m.apiKeyInput.Blur()
						m.endpointInput.Focus()
					}
					return m, textinput.Blink
				case "enter":
					valKey := strings.TrimSpace(m.apiKeyInput.Value())
					valEP := strings.TrimSpace(m.endpointInput.Value())
					prov := m.connectProvider
					meta := domain.GetProviderMeta(prov)
					pCfg := m.aiConfig.Providers[prov]
					pCfg.APIKey = valKey
					pCfg.Endpoint = valEP
					if pCfg.DefaultModel == "" {
						pCfg.DefaultModel = meta.DefaultModel
					}
					m.aiConfig.Providers[prov] = pCfg
					m.aiConfig.ActiveProvider = prov
					m.aiConfig.ActiveModel = pCfg.DefaultModel

					if m.vaultService != nil {
						if err := m.vaultService.SaveAIConfig(m.aiConfig); err != nil {
							m.statusMessage = fmt.Sprintf("[!!] Error guardando en bóveda: %v", err)
						} else {
							m.statusMessage = fmt.Sprintf("[OK] Clave de %s cifrada bajo Blindaje D2 (AES-256-GCM)", meta.Name)
						}
					}
					m.triageClient = ai.NewTriageClientWithConfig(m.aiConfig)
					m.diagnosisCache = make(map[string]string)
					m.statusExpiry = time.Now().Add(4 * time.Second)
					m.apiKeyInput.Blur()
					m.endpointInput.Blur()
					m.configMode = configViewSelectModel
					m.modelSearchInput.Focus()
					return m, textinput.Blink
				default:
					var cmd tea.Cmd
					if m.connectFocusField == 0 {
						m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
					} else {
						m.endpointInput, cmd = m.endpointInput.Update(msg)
					}
					return m, cmd
				}

			case configViewCustomModel:
				switch msg.String() {
				case "esc":
					m.customModelInput.Blur()
					m.configMode = configViewSelectModel
					m.modelSearchInput.Focus()
					return m, textinput.Blink
				case "enter":
					customVal := strings.TrimSpace(m.customModelInput.Value())
					if customVal != "" {
						m.aiConfig.ActiveModel = customVal
						prov := m.aiConfig.ActiveProvider
						pCfg := m.aiConfig.Providers[prov]
						pCfg.DefaultModel = customVal
						m.aiConfig.Providers[prov] = pCfg
						if m.vaultService != nil {
							_ = m.vaultService.SaveAIConfig(m.aiConfig)
						}
						m.triageClient = ai.NewTriageClientWithConfig(m.aiConfig)
						m.diagnosisCache = make(map[string]string)
						m.statusMessage = fmt.Sprintf("[OK] Modelo personalizado activo: %s", customVal)
						m.statusExpiry = time.Now().Add(4 * time.Second)
						m.activeState = stateFleetTable
						return m, nil
					}
					m.configMode = configViewSelectModel
					m.modelSearchInput.Focus()
					return m, textinput.Blink
				default:
					var cmd tea.Cmd
					m.customModelInput, cmd = m.customModelInput.Update(msg)
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
			case "c":
				m.activeState = stateConfigModal
				m.configMode = configViewSelectModel
				m.modelSearchInput.Reset()
				m.modelSearchInput.Focus()
				m.modelListCursor = 0
				items := m.getFilteredModelItems()
				for idx, it := range items {
					if it.isActive {
						m.modelListCursor = idx
						break
					}
				}
				return m, textinput.Blink

			case "t":
				m.activeState = stateThemeModal
				m.themeListCursor = 0
				for idx, th := range domain.AvailableThemes {
					if th.ID == m.themeConfig.ActiveTheme {
						m.themeListCursor = idx
						break
					}
				}
				return m, nil

			case "p":
				if len(filtered) > 0 && m.cursor < len(filtered) {
					c := filtered[m.cursor]
					m.togglePin(c.Name)
					if m.isPinned(c.Name) {
						m.statusMessage = fmt.Sprintf("[📌] Contenedor '%s' fijado al inicio de la flota", c.Name)
					} else {
						m.statusMessage = fmt.Sprintf("[-] Contenedor '%s' desanclado", c.Name)
					}
					m.statusExpiry = time.Now().Add(3 * time.Second)
					return m, nil
				}

			case "P":
				if len(m.pinnedContainers) > 0 {
					m.clearAllPins()
					m.statusMessage = "[✔] Todos los contenedores desanclados"
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
		m.diagnosisCache[msg.containerID] = msg.diagnosis
		m.lastDiagnosisUsage[msg.containerID] = msg.usage
		if msg.usage.TotalTokens > 0 {
			m.sessionTokensUsed += msg.usage.TotalTokens
			m.sessionCostUSD += msg.usage.EstimatedCostUSD
		}
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
