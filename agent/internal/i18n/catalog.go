package i18n

import (
	"fmt"
	"strings"
)

// Language define los idiomas soportados por el sistema.
type Language string

const (
	LangES Language = "es"
	LangEN Language = "en"
)

// NormalizeLanguage normaliza una cadena a un idioma soportado.
// Valor desconocido o vacío cae a LangEN de forma documentada sin crash (C2).
func NormalizeLanguage(val string) Language {
	clean := strings.ToLower(strings.TrimSpace(val))
	if clean == "es" || strings.HasPrefix(clean, "es_") || strings.HasPrefix(clean, "es-") {
		return LangES
	}
	return LangEN
}

// Catalog almacena las cadenas bilingües del sistema.
// Regla C1: placeholders NOMBRADOS ({name}, {n}, etc.). Prohibido %s posicional.
var catalog = map[Language]map[string]string{
	LangES: {
		// Primera ejecución (Fallback TUI)
		"first_run.title":           "solv · primera ejecución",
		"first_run.subtitle":        "idioma / language",
		"first_run.hint":            "enter: continuar · se cambia en [t]",
		"first_run.spanish":         "español",
		"first_run.english":         "english",

		// Toast de bienvenida
		"toast.onboarding":          "pulsa ? para ver atajos",

		// Preferencias
		"preferences.title":         "preferencias",
		"preferences.theme":         "temas y estilos",
		"preferences.banner_policy": "origen en banner: {policy}",
		"preferences.policy_info":   "informativo",
		"preferences.policy_prud":   "prudente",
		"preferences.language":      "idioma / language: {lang}",
		"preferences.footer":        "enter: cambiar · esc: volver",

		// Chrome general & Footer
		"chrome.help_hint":          "? ayuda",
		"chrome.exit_hint":          "ctrl+c salir",
		"chrome.session_meter":      "sesión: {tokens} tok · ~${cost}",
		"chrome.esc":                "esc",
		"chrome.enter":              "enter",

		// Vistas y tablas
		"fleet.header.status":       "ESTADO",
		"fleet.header.container":    "CONTENEDOR",
		"fleet.header.image":        "IMAGEN",
		"fleet.header.cpu":          "CPU",
		"fleet.header.memory":       "MEMORIA",
		"fleet.header.net":          "RED (E/S)",
		"fleet.header.restarts":     "REINICIOS",
		"fleet.footer.hint":         "↑/↓: navegar  ·  /: filtrar  ·  enter/l: logs  ·  d: detalle  ·  i: IA  ·  t: preferencias  ·  ?: ayuda",
		"fleet.footer.filtered":     "filtrando: '{query}'  ·  esc: limpiar  ·  enter: confirmar",

		// Split-pane y Ficha Técnica
		"fleet.title.containers":       "CONTENEDORES ({count})",
		"fleet.title.containers_page":  "CONTENEDORES ({count}) • {start}-{end}",
		"fleet.title.pinned":           "FIJADOS ({count})",
		"fleet.no_results":             "Sin resultados para '{filter}'",
		"fleet.scanning":               "Escaneando socket...",
		"fleet.select_hint":            "Selecciona un contenedor de la lista izquierda.",
		"detail.image":                 "Imagen",
		"detail.id":                    "ID",
		"detail.category":              "Categoría",
		"detail.pinned_badge":          "FIJADO",
		"detail.lifecycle":             "CICLO DE VIDA",
		"detail.restarts":              "restarts: {count} en ciclo · policy: {policy}",
		"detail.vitals":                "VITALES",
		"detail.ram_no_limit":          "RAM: {ram} MB (Sin límite Docker)",
		"detail.ram_oom_risk":          "[!!] riesgo OOM",
		"detail.network":               "RED",
		"detail.trend_stable":          "estable",
		"detail.diagnosis":             "DIAGNÓSTICO",
		"detail.part_of_incident":      "parte de incidente {group} · {hint}",
		"detail.diagnosis_hint_detail":  "[d] detalle",
		"detail.diagnosis_hint_request": "[d] solicitar",
		"detail.no_diagnosis":          "[--] sin diagnóstico · {hint}",
		"detail.context":               "CONTEXTO",
		"detail.depend_none":           "dependen de mi: --",
		"detail.depend_list":           "dependen de mi: {list}",
		"detail.depend_count":          "dependen de mi: {count} · [n] detalle",
		"status_bar.filter":            " [Filtro: '{filter}']",
		"status_bar.mode":              "modo",
		"logs.breadcrumb":              "[<] Volver (Esc) | Logs: {name} | Estado: {status}",
		"logs.footer":                  "[Esc]: Volver a la flota  |  [Flechas / Scroll]: Desplazar registros",
		"logs.no_limit":                "Sin límite Docker",
		"logs.peak":                    "Pico",
		"time.ago_seconds":             "hace {n}s",
		"time.ago_minutes":             "hace {n}m",
		"time.ago_hours":               "hace {n}h",
		"time.recent":                  "reciente",
		"evidence.restarts_current":    "{count} en ciclo actual",
		"evidence.ram_no_limit":        "{ram}MB (sin límite Docker)",
		"evidence.trend":               "tendencia",
		"evidence.held_by":             "retenido por: {owner}",
		"evidence.port":                "puerto {port}",
		"evidence.crashes_homog":       "{count} crashes en 1h ({homog}) · último {ago}",
		"evidence.prev_hypothesis":     "hipótesis previa",
		"evidence.ram_trend":           "tendencia RAM",
		"evidence.model_confidence_low": "confianza del modelo: baja",
		"evidence.reanalyzed_deep":     "re-analizado en DEEP por severidad crítica",
		"fleet.status.ok":           "saludable",
		"fleet.status.stopped":      "detenido",
		"fleet.status.paused":       "pausado",
		"fleet.status.critical":     "crítico",

		// Banners
		"banner.diagnosing":         "Analizando causa raíz con IA...",
		"banner.pending":            "Pendiente de diagnóstico analítico...",
		"banner.not_configured":     "Diagnóstico no configurado · [c]",
		"banner.recurrent":          "recurrente ({n}/1h)",
		"banner.conf_low":           "· conf baja",
		"banner.detail_hint":        "· [d] detalle",
		"banner.ai_hint_short":      "· [i] IA",
		"banner.ai_hint_long":       "· [i] solicitar diagnóstico IA",

		// Incidentes (Placeholders nombrados)
		"incident.default_group":     "incidente",
		"incident.confirmed":         "[INC] {group} · origen {origin} ({reason}) -> {count} · {tag} · [d]",
		"incident.probable_info":     "[INC] {group} · origen prob. {origin} -> {count} · {tag} · [d]",
		"incident.probable_prud":     "[INC] {group} · {count} anómalos · {tag} · [d]",
		"incident.undetermined":      "[INC] {group} · {count} anómalos · origen s/d · {tag} · [d]",
		"incident.reason_default":    "fallo",

		// Diagnóstico V4
		"diagnosis.title":           "diagnóstico · {name}",
		"diagnosis.rule_level":      "regla determinística local (sin IA)",
		"diagnosis.ai_level":        "diagnóstico asistido por IA",
		"diagnosis.sig_level":       "señal pura de contenedor",
		"diagnosis.root_cause":      "causa raíz:",
		"diagnosis.evidence":        "evidencia:",
		"diagnosis.evidence_active": "evidencia [activo • ↑/↓ para scrollear]:",
		"diagnosis.evidence_tab":    "evidencia [pulsa Tab para scrollear]:",
		"diagnosis.no_evidence":     "sin telemetría anómala registrada",
		"diagnosis.more_above":      "↑ más arriba",
		"diagnosis.more_below":      "↓ +{n} más (usa ↓ para bajar)",
		"diagnosis.more_tab":        "+{n} más · pulsa [Tab] para scrollear",
		"diagnosis.action_title":    "acción sugerida:",
		"diagnosis.action_active":   "acción sugerida [activo]:",
		"diagnosis.action_restart":  "aplicar restart",
		"diagnosis.action_stop":     "aplicar stop",
		"diagnosis.action_isolate":  "aislar de red",
		"diagnosis.action_logs":     "ver logs",
		"diagnosis.action_ai":       "solicitar diagnóstico IA",
		"diagnosis.footer_normal":   "enter: ejecutar  ·  esc: volver  ·  ↑/↓: seleccionar",
		"diagnosis.footer_ev_act":   "↑/↓: scrollear evidencia  ·  tab: ir a acciones  ·  esc: volver",
		"diagnosis.footer_ev_tab":   "enter: ejecutar  ·  tab: scrollear evidencia  ·  ↑/↓: seleccionar  ·  esc: volver",

		// Reglas Curadas (12 reglas determinísticas)
		"rule.oom_limit":        "OOMKilled: contenedor superó límite de memoria",
		"rule.oom_status":       "OOMKilled: límite de memoria excedido",
		"rule.sigterm":          "Terminado por SIGTERM/timeout",
		"rule.net_refused":      "Servicio dependiente no alcanzable",
		"rule.port_conflict":    "Conflicto de puerto en el host",
		"rule.disk_full":        "Sin espacio en disco",
		"rule.perm_denied":      "Error de permisos en volúmenes/archivos",
		"rule.exec_format":      "Mismatch de arquitectura (ej: amd64 en arm64)",
		"rule.kernel_killed":    "Proceso eliminado por el kernel (posible OOM global)",
		"rule.crash_loop":       "Fallo recurrente en el arranque",
		"rule.invalid_config":   "Error de configuración o comando de entrada inválido",
		"rule.unexpected_exit":  "Reinicio limpio pero inesperado",

		// Mensajes de Ayuda [?]
		"help.title":            "ayuda · atajos de teclado",
		"help.navigation":       "navegación",
		"help.actions":          "acciones",
		"help.views":            "vistas",
		"help.footer":           "esc: volver a la flota",

		// Mensajes de Telegram
		"telegram.alert_title":  "Alerta de contenedor anómalo en {host}",
		"telegram.btn_action":   "[{action}] Contenedor (60s)",
		"telegram.unauthorized": "Usuario no autorizado para ejecutar remediaciones",
		"telegram.expired":      "Botón expirado (TTL de {ttl}s superado)",
		"telegram.invalid_sig":  "Firma de callback inválida o alterada",
	},
	LangEN: {
		// First run (Fallback TUI)
		"first_run.title":           "solv · first run",
		"first_run.subtitle":        "language / idioma",
		"first_run.hint":            "enter: continue · change anytime with [t]",
		"first_run.spanish":         "español",
		"first_run.english":         "english",

		// Welcome toast
		"toast.onboarding":          "press ? to open the help menu with all available shortcuts",

		// Preferences
		"preferences.title":         "preferences",
		"preferences.theme":         "themes and styles",
		"preferences.banner_policy": "banner origin: {policy}",
		"preferences.policy_info":   "informative",
		"preferences.policy_prud":   "prudent",
		"preferences.language":      "language / idioma: {lang}",
		"preferences.footer":        "enter: change · esc: back",

		// General Chrome & Footer
		"chrome.help_hint":          "? help",
		"chrome.exit_hint":          "ctrl+c exit",
		"chrome.session_meter":      "session: {tokens} tok · ~${cost}",
		"chrome.esc":                "esc",
		"chrome.enter":              "enter",

		// Views and tables
		"fleet.header.status":       "STATUS",
		"fleet.header.container":    "CONTAINER",
		"fleet.header.image":        "IMAGE",
		"fleet.header.cpu":          "CPU",
		"fleet.header.memory":       "MEMORY",
		"fleet.header.net":          "NET (I/O)",
		"fleet.header.restarts":     "RESTARTS",
		"fleet.footer.hint":         "↑/↓: navigate  ·  /: filter  ·  enter/l: logs  ·  d: details  ·  i: AI  ·  t: preferences  ·  ?: help",
		"fleet.footer.filtered":     "filtering: '{query}'  ·  esc: clear  ·  enter: confirm",

		// Split-pane & Tech Sheet
		"fleet.title.containers":       "CONTAINERS ({count})",
		"fleet.title.containers_page":  "CONTAINERS ({count}) • {start}-{end}",
		"fleet.title.pinned":           "PINNED ({count})",
		"fleet.no_results":             "No results for '{filter}'",
		"fleet.scanning":               "Scanning socket...",
		"fleet.select_hint":            "Select a container from the left list.",
		"detail.image":                 "Image",
		"detail.id":                    "ID",
		"detail.category":              "Category",
		"detail.pinned_badge":          "PINNED",
		"detail.lifecycle":             "LIFECYCLE",
		"detail.restarts":              "restarts: {count} in cycle · policy: {policy}",
		"detail.vitals":                "VITALS",
		"detail.ram_no_limit":          "RAM: {ram} MB (No Docker limit)",
		"detail.ram_oom_risk":          "[!!] OOM risk",
		"detail.network":               "NETWORK",
		"detail.trend_stable":          "stable",
		"detail.diagnosis":             "DIAGNOSIS",
		"detail.part_of_incident":      "part of incident {group} · {hint}",
		"detail.diagnosis_hint_detail":  "[d] detail",
		"detail.diagnosis_hint_request": "[d] request",
		"detail.no_diagnosis":          "[--] no diagnosis · {hint}",
		"detail.context":               "CONTEXT",
		"detail.depend_none":           "depend on me: --",
		"detail.depend_list":           "depend on me: {list}",
		"detail.depend_count":          "depend on me: {count} · [n] detail",
		"status_bar.filter":            " [Filter: '{filter}']",
		"status_bar.mode":              "mode",
		"logs.breadcrumb":              "[<] Back (Esc) | Logs: {name} | Status: {status}",
		"logs.footer":                  "[Esc]: Back to fleet  |  [Arrows / Scroll]: Scroll logs",
		"logs.no_limit":                "No Docker limit",
		"logs.peak":                    "Peak",
		"time.ago_seconds":             "{n}s ago",
		"time.ago_minutes":             "{n}m ago",
		"time.ago_hours":               "{n}h ago",
		"time.recent":                  "recent",
		"evidence.restarts_current":    "{count} in current cycle",
		"evidence.ram_no_limit":        "{ram}MB (no Docker limit)",
		"evidence.trend":               "trend",
		"evidence.held_by":             "held by: {owner}",
		"evidence.port":                "port {port}",
		"evidence.crashes_homog":       "{count} crashes in 1h ({homog}) · last {ago}",
		"evidence.prev_hypothesis":     "previous hypothesis",
		"evidence.ram_trend":           "RAM trend",
		"evidence.model_confidence_low": "model confidence: low",
		"evidence.reanalyzed_deep":     "re-analyzed in DEEP due to critical severity",
		"fleet.status.ok":           "healthy",
		"fleet.status.stopped":      "stopped",
		"fleet.status.paused":       "paused",
		"fleet.status.critical":     "critical",

		// Banners
		"banner.diagnosing":         "Analyzing root cause with AI...",
		"banner.pending":            "Pending analytical diagnosis...",
		"banner.not_configured":     "Diagnosis not configured · [c]",
		"banner.recurrent":          "recurring ({n}/1h)",
		"banner.conf_low":           "· low conf",
		"banner.detail_hint":        "· [d] details",
		"banner.ai_hint_short":      "· [i] AI",
		"banner.ai_hint_long":       "· [i] request AI diagnosis",

		// Incidents (Named placeholders)
		"incident.default_group":     "incident",
		"incident.confirmed":         "[INC] {group} · origin {origin} ({reason}) -> {count} · {tag} · [d]",
		"incident.probable_info":     "[INC] {group} · probable origin {origin} -> {count} · {tag} · [d]",
		"incident.probable_prud":     "[INC] {group} · {count} anomalous · {tag} · [d]",
		"incident.undetermined":      "[INC] {group} · {count} anomalous · origin unk · {tag} · [d]",
		"incident.reason_default":    "failure",

		// Diagnosis V4
		"diagnosis.title":           "diagnosis · {name}",
		"diagnosis.rule_level":      "deterministic local rule (zero AI)",
		"diagnosis.ai_level":        "AI-assisted diagnosis",
		"diagnosis.sig_level":       "raw container signal",
		"diagnosis.root_cause":      "root cause:",
		"diagnosis.evidence":        "evidence:",
		"diagnosis.evidence_active": "evidence [active • ↑/↓ to scroll]:",
		"diagnosis.evidence_tab":    "evidence [press Tab to scroll]:",
		"diagnosis.no_evidence":     "no anomalous telemetry recorded",
		"diagnosis.more_above":      "↑ more above",
		"diagnosis.more_below":      "↓ +{n} more (use ↓ to scroll)",
		"diagnosis.more_tab":        "+{n} more · press [Tab] to scroll",
		"diagnosis.action_title":    "suggested action:",
		"diagnosis.action_active":   "suggested action [active]:",
		"diagnosis.action_restart":  "apply restart",
		"diagnosis.action_stop":     "apply stop",
		"diagnosis.action_isolate":  "isolate network",
		"diagnosis.action_logs":     "view logs",
		"diagnosis.action_ai":       "request AI diagnosis",
		"diagnosis.footer_normal":   "enter: execute  ·  esc: back  ·  ↑/↓: select",
		"diagnosis.footer_ev_act":   "↑/↓: scroll evidence  ·  tab: go to actions  ·  esc: back",
		"diagnosis.footer_ev_tab":   "enter: execute  ·  tab: scroll evidence  ·  ↑/↓: select  ·  esc: back",

		// Curated Rules (12 deterministic rules)
		"rule.oom_limit":        "OOMKilled: container exceeded memory limit",
		"rule.oom_status":       "OOMKilled: memory limit exceeded",
		"rule.sigterm":          "Terminated by SIGTERM/timeout",
		"rule.net_refused":      "Dependent service unreachable",
		"rule.port_conflict":    "Host port conflict",
		"rule.disk_full":        "No space left on device",
		"rule.perm_denied":      "Permission denied on volumes/files",
		"rule.exec_format":      "Architecture mismatch (e.g. amd64 on arm64)",
		"rule.kernel_killed":    "Process killed by kernel (possible global OOM)",
		"rule.crash_loop":       "Recurring crash on startup",
		"rule.invalid_config":   "Configuration error or invalid entrypoint",
		"rule.unexpected_exit":  "Clean but unexpected restart",

		// Help Dialog [?]
		"help.title":            "help · keyboard shortcuts",
		"help.navigation":       "navigation",
		"help.actions":          "actions",
		"help.views":            "views",
		"help.footer":           "esc: return to fleet",

		// Telegram Messages
		"telegram.alert_title":  "Anomalous container alert on {host}",
		"telegram.btn_action":   "[{action}] Container (60s)",
		"telegram.unauthorized": "Unauthorized user for remediations",
		"telegram.expired":      "Expired button (TTL of {ttl}s exceeded)",
		"telegram.invalid_sig":  "Invalid or tampered callback signature",
	},
}

// T traduce una clave con placeholders nombrados ({key}).
// Si la clave no existe en el catálogo, retorna la clave misma como fallback visible (C1).
func T(lang Language, key string, vars ...map[string]interface{}) string {
	normalized := NormalizeLanguage(string(lang))
	langMap, ok := catalog[normalized]
	if !ok {
		langMap = catalog[LangEN]
	}

	raw, found := langMap[key]
	if !found {
		// Fallback visible en desarrollo: renderiza la key misma (C1)
		return key
	}

	if len(vars) == 0 || vars[0] == nil {
		return raw
	}

	vMap := vars[0]
	res := raw
	for k, v := range vMap {
		placeholder := fmt.Sprintf("{%s}", k)
		res = strings.ReplaceAll(res, placeholder, fmt.Sprintf("%v", v))
	}

	return res
}
