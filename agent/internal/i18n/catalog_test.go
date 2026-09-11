package i18n

import (
	"testing"
)

func TestI18n_NamedPlaceholders(t *testing.T) {
	// Prueba de placeholders nombrados en ES y EN
	es := T(LangES, "incident.probable_info", map[string]interface{}{
		"group":  "solv_net",
		"origin": "postgres",
		"count":  2,
		"tag":    "[AI]",
	})
	expectedES := "[INC] solv_net · origen prob. postgres -> 2 · [AI] · [d]"
	if es != expectedES {
		t.Errorf("expected %q, got %q", expectedES, es)
	}

	en := T(LangEN, "incident.probable_info", map[string]interface{}{
		"group":  "solv_net",
		"origin": "postgres",
		"count":  2,
		"tag":    "[AI]",
	})
	expectedEN := "[INC] solv_net · probable origin postgres -> 2 · [AI] · [d]"
	if en != expectedEN {
		t.Errorf("expected %q, got %q", expectedEN, en)
	}
}

func TestI18n_MissingKeyReturnsKeyItself(t *testing.T) {
	// Criterio C1: Si la clave no existe, renderiza la key misma (nunca crash ni vacío)
	missingKey := "non.existent.key"
	res := T(LangES, missingKey)
	if res != missingKey {
		t.Errorf("expected missing key %q to return itself, got %q", missingKey, res)
	}
}

func TestI18n_UnknownLanguageFallsBackToEnglish(t *testing.T) {
	// Criterio C2: Valor desconocido cae a EN sin crash
	res := T("fr", "fleet.status.ok")
	expected := "healthy"
	if res != expected {
		t.Errorf("expected fallback to EN %q, got %q", expected, res)
	}
}

func TestI18n_WordOrderInvertedTest(t *testing.T) {
	// Criterio 5: Caso con orden de palabras alterado
	diagES := T(LangES, "diagnosis.title", map[string]interface{}{"name": "redis_cache"})
	if diagES != "diagnóstico · redis_cache" {
		t.Errorf("unexpected ES: %q", diagES)
	}

	diagEN := T(LangEN, "diagnosis.title", map[string]interface{}{"name": "redis_cache"})
	if diagEN != "diagnosis · redis_cache" {
		t.Errorf("unexpected EN: %q", diagEN)
	}
}
