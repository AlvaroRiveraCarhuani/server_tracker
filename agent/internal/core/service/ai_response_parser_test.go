package service

import (
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestAIResponseParser_ExtractJSONBlock(t *testing.T) {
	parser := NewAIResponseParser()

	tests := []struct {
		name      string
		input     string
		expected  string
		wantFound bool
	}{
		{
			name:      "JSON puro",
			input:     `{"root_cause":"OOMKilled","severity":"critical","suggested_action":"restart","confidence":"high"}`,
			expected:  `{"root_cause":"OOMKilled","severity":"critical","suggested_action":"restart","confidence":"high"}`,
			wantFound: true,
		},
		{
			name:      "Prosa antes y después con saludo",
			input:     "Hola operador! Aquí está mi diagnóstico:\n{\"root_cause\":\"Fuga de memoria\",\"severity\":\"critical\",\"suggested_action\":\"restart\",\"confidence\":\"high\"}\nEspero te sirva.",
			expected:  `{"root_cause":"Fuga de memoria","severity":"critical","suggested_action":"restart","confidence":"high"}`,
			wantFound: true,
		},
		{
			name:      "Llaves anidadas",
			input:     `Texto previo {"root_cause":"crash", "extra":{"code": 137}, "severity":"critical"} texto final`,
			expected:  `{"root_cause":"crash", "extra":{"code": 137}, "severity":"critical"}`,
			wantFound: true,
		},
		{
			name:      "Sin JSON",
			input:     "Solo texto libre sin llaves",
			expected:  "",
			wantFound: false,
		},
		{
			name:      "Llave abierta sin cerrar",
			input:     `{"root_cause":"incompleto"`,
			expected:  "",
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := parser.ExtractJSONBlock(tc.input)
			if found != tc.wantFound {
				t.Fatalf("expected found=%v, got=%v", tc.wantFound, found)
			}
			if got != tc.expected {
				t.Errorf("expected block %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestAIResponseParser_Parse_ValidProse(t *testing.T) {
	parser := NewAIResponseParser()
	raw := "Estimado operador:\n{\"root_cause\":\"Fuga de memoria en worker\",\"severity\":\"critical\",\"suggested_action\":\"restart\",\"confidence\":\"high\"}\nSaludos cordiales!"

	res := parser.Parse(raw, domain.TokenUsage{TotalTokens: 42}, "Exited (137)")

	if res.Level != domain.LevelAI {
		t.Errorf("expected LevelAI, got %v", res.Level)
	}
	if res.RootCause != "Fuga de memoria en worker" {
		t.Errorf("expected clean root cause, got %q", res.RootCause)
	}
	if res.Severity != "critical" {
		t.Errorf("expected critical, got %q", res.Severity)
	}
	if res.SuggestedAction != "restart" {
		t.Errorf("expected restart, got %q", res.SuggestedAction)
	}
	if res.Confidence != "high" {
		t.Errorf("expected high, got %q", res.Confidence)
	}
}

func TestAIResponseParser_MissingConfidence_ForcesLow(t *testing.T) {
	parser := NewAIResponseParser()
	raw := `{"root_cause":"Timeout en upstream","severity":"warning","suggested_action":"none"}`

	res := parser.Parse(raw, domain.TokenUsage{}, "Up 2 hours")

	if res.Level != domain.LevelAI {
		t.Errorf("expected LevelAI, got %v", res.Level)
	}
	if res.Confidence != "low" {
		t.Errorf("expected confidence forced to low, got %q", res.Confidence)
	}
}

func TestAIResponseParser_InvalidActionDelete_ForcesNoneAndLow(t *testing.T) {
	parser := NewAIResponseParser()
	raw := `{"root_cause":"Disco lleno","severity":"critical","suggested_action":"delete","confidence":"high"}`

	res := parser.Parse(raw, domain.TokenUsage{}, "Exited (1)")

	if res.SuggestedAction != "none" {
		t.Errorf("expected action 'delete' to fall back to 'none', got %q", res.SuggestedAction)
	}
	if res.Confidence != "low" {
		t.Errorf("expected confidence forced to low due to fallback action, got %q", res.Confidence)
	}
}

func TestAIResponseParser_EmptyRootCause_FallsBackToAIPartial(t *testing.T) {
	parser := NewAIResponseParser()
	raw := `{"root_cause":"","severity":"critical","suggested_action":"restart","confidence":"high"}`

	res := parser.Parse(raw, domain.TokenUsage{}, "Exited (137)")

	if res.Level != domain.LevelAIPartial {
		t.Errorf("expected LevelAIPartial, got %v", res.Level)
	}
}

func TestAIResponseParser_NoJSON_FallsBackToAIPartial(t *testing.T) {
	parser := NewAIResponseParser()
	raw := "Parece que el contenedor falló por falta de descriptores de archivo."

	res := parser.Parse(raw, domain.TokenUsage{}, "Exited (1)")

	if res.Level != domain.LevelAIPartial {
		t.Errorf("expected LevelAIPartial, got %v", res.Level)
	}
	if res.RootCause != raw {
		t.Errorf("expected raw string as root cause, got %q", res.RootCause)
	}
}

func TestAIResponseParser_IncidentFields_AndSanitize(t *testing.T) {
	parser := NewAIResponseParser()
	raw := `{"root_cause":"Cascada por OOM en postgres","severity":"critical","suggested_action":"restart","confidence":"high","origin_container":"postgres","cascade":["postgres","api-node","nginx","external-rogue"]}`

	res := parser.Parse(raw, domain.TokenUsage{}, "Exited (137)")
	if res.OriginContainer != "postgres" {
		t.Errorf("expected origin_container 'postgres', got %q", res.OriginContainer)
	}
	if len(res.Cascade) != 4 {
		t.Fatalf("expected 4 cascade members, got %d", len(res.Cascade))
	}

	validGroup := map[string]bool{
		"postgres": true,
		"api-node": true,
		"nginx":    true,
	}

	sanitized := parser.SanitizeIncidentMembers(res, validGroup)
	if sanitized.OriginContainer != "postgres" {
		t.Errorf("expected origin_container 'postgres', got %q", sanitized.OriginContainer)
	}
	if len(sanitized.Cascade) != 3 {
		t.Fatalf("expected 3 cascade members after sanitize, got %d", len(sanitized.Cascade))
	}
	for _, m := range sanitized.Cascade {
		if m == "external-rogue" {
			t.Errorf("expected 'external-rogue' to be filtered out")
		}
	}

	// Test case where origin is outside group
	invalidOrigin := res
	invalidOrigin.OriginContainer = "hacker-bot"
	sanitizedInvalid := parser.SanitizeIncidentMembers(invalidOrigin, validGroup)
	if sanitizedInvalid.OriginContainer != "" {
		t.Errorf("expected invalid origin to be empty, got %q", sanitizedInvalid.OriginContainer)
	}
}
