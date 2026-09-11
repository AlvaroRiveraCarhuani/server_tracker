package service

import (
	"encoding/json"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

// Whitelists para validación de enums del contrato de salida estructurado
var (
	validSeverities = map[string]bool{
		"critical": true,
		"warning":  true,
		"info":     true,
	}

	validActions = map[string]bool{
		"restart": true,
		"stop":    true,
		"isolate": true,
		"none":    true,
	}

	validConfidences = map[string]bool{
		"high":   true,
		"medium": true,
		"low":    true,
	}
)

// AIResponseParser procesa las respuestas crudas de los modelos de lenguaje extrayendo
// bloques JSON balanceados y aplicando recuperación por campo con fallbacks seguros.
type AIResponseParser struct{}

// NewAIResponseParser inicializa una nueva instancia del parser.
func NewAIResponseParser() *AIResponseParser {
	return &AIResponseParser{}
}

// ExtractJSONBlock busca y extrae el primer bloque {...} balanceado en el texto.
func (p *AIResponseParser) ExtractJSONBlock(text string) (string, bool) {
	start := strings.Index(text, "{")
	if start == -1 {
		return "", false
	}

	depth := 0
	inString := false
	var escapeNext bool

	for i := start; i < len(text); i++ {
		char := text[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		if char == '\\' && inString {
			escapeNext = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			if char == '{' {
				depth++
			} else if char == '}' {
				depth--
				if depth == 0 {
					return text[start : i+1], true
				}
			}
		}
	}

	return "", false
}

// Parse procesa el texto crudo del modelo según las reglas S1:
// 1. Extrae el primer bloque {...} balanceado.
// 2. Si no hay bloque o el JSON no parsea -> Nivel 1 [AI~] con texto raw truncado a 1 línea.
// 3. Valida campo por campo:
//    - root_cause ausente o vacío -> Nivel 1 [AI~].
//    - severity inválida/ausente -> derivada de la señal, y fuerza confidence a low.
//    - suggested_action inválida (ej: delete) -> none, y fuerza confidence a low.
//    - confidence inválida/ausente -> low. Si cualquier campo se defaulteó -> forzar low.
func (p *AIResponseParser) Parse(raw string, usage domain.TokenUsage, status string) domain.DiagnosisResult {
	singleLineRaw := cleanSingleLine(raw)

	block, found := p.ExtractJSONBlock(raw)
	if !found {
		return p.fallbackPartial(singleLineRaw, usage)
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal([]byte(block), &rawMap); err != nil {
		return p.fallbackPartial(singleLineRaw, usage)
	}

	// 1. root_cause
	rawRC, ok := rawMap["root_cause"]
	if !ok {
		return p.fallbackPartial(singleLineRaw, usage)
	}
	rootCause, ok := rawRC.(string)
	rootCause = strings.TrimSpace(rootCause)
	if !ok || rootCause == "" {
		return p.fallbackPartial(singleLineRaw, usage)
	}

	forcedLow := false

	// 2. severity
	severity := ""
	if rawSev, ok := rawMap["severity"]; ok {
		if sevStr, ok := rawSev.(string); ok {
			sevLower := strings.ToLower(strings.TrimSpace(sevStr))
			if validSeverities[sevLower] {
				severity = sevLower
			}
		}
	}
	if severity == "" {
		forcedLow = true
		severity = deriveSignalSeverity(status)
	}

	// 3. suggested_action
	suggestedAction := ""
	if rawAct, ok := rawMap["suggested_action"]; ok {
		if actStr, ok := rawAct.(string); ok {
			actLower := strings.ToLower(strings.TrimSpace(actStr))
			if validActions[actLower] {
				suggestedAction = actLower
			}
		}
	}
	if suggestedAction == "" {
		forcedLow = true
		suggestedAction = "none"
	}

	// 4. confidence
	confidence := ""
	if rawConf, ok := rawMap["confidence"]; ok {
		if confStr, ok := rawConf.(string); ok {
			confLower := strings.ToLower(strings.TrimSpace(confStr))
			if validConfidences[confLower] {
				confidence = confLower
			}
		}
	}
	if confidence == "" || forcedLow {
		confidence = "low"
	}

	// 5. origin_container (Ola 6)
	originContainer := ""
	if rawOrigin, ok := rawMap["origin_container"]; ok {
		if origStr, ok := rawOrigin.(string); ok {
			originContainer = strings.TrimSpace(origStr)
		}
	}

	// 6. cascade (Ola 6)
	var cascade []string
	if rawCascade, ok := rawMap["cascade"]; ok {
		if cascadeList, ok := rawCascade.([]interface{}); ok {
			for _, item := range cascadeList {
				if s, ok := item.(string); ok {
					s = strings.TrimSpace(s)
					if s != "" {
						cascade = append(cascade, s)
					}
				}
			}
		}
	}

	return domain.DiagnosisResult{
		Level:           domain.LevelAI,
		RootCause:       rootCause,
		Severity:        severity,
		SuggestedAction: suggestedAction,
		Confidence:      confidence,
		OriginContainer: originContainer,
		Cascade:         cascade,
		RawOutput:       singleLineRaw,
		TokenUsage:      usage,
	}
}

// SanitizeIncidentMembers filtra silenciosamente valores fuera del grupo conectado (Decisión 2).
func (p *AIResponseParser) SanitizeIncidentMembers(res domain.DiagnosisResult, validMembers map[string]bool) domain.DiagnosisResult {
	if res.OriginContainer != "" && !validMembers[res.OriginContainer] {
		res.OriginContainer = ""
	}

	if len(res.Cascade) > 0 {
		var filtered []string
		for _, name := range res.Cascade {
			if validMembers[name] {
				filtered = append(filtered, name)
			}
		}
		res.Cascade = filtered
	}

	return res
}

func (p *AIResponseParser) fallbackPartial(singleLineRaw string, usage domain.TokenUsage) domain.DiagnosisResult {
	truncated := singleLineRaw
	if len(truncated) > 75 {
		truncated = truncated[:72] + "..."
	}

	return domain.DiagnosisResult{
		Level:           domain.LevelAIPartial,
		RootCause:       truncated,
		Severity:        "warning",
		SuggestedAction: "none",
		Confidence:      "low",
		RawOutput:       singleLineRaw,
		TokenUsage:      usage,
	}
}

func deriveSignalSeverity(status string) string {
	sLower := strings.ToLower(status)
	if strings.Contains(sLower, "oom") || strings.Contains(sLower, "137") {
		return "critical"
	}
	if domain.ParseExitCode(status) > 0 {
		return "critical"
	}
	return "warning"
}

func cleanSingleLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}
