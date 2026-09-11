package service

import (
	"strings"
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestCrashJournal_Deduplication(t *testing.T) {
	journal := NewCrashJournal()
	changeTime := time.Now().Add(-5 * time.Minute)

	c := domain.ContainerMetric{
		ID:              "c-dedupe",
		Name:            "test-app",
		Status:          "Exited (137) 5 minutes ago",
		LastStateChange: changeTime,
	}

	// Primer registro: debe retornar true
	recorded, key := journal.Record(c, "OOMKilled", "RULE")
	if !recorded || key != "oom" {
		t.Fatalf("se esperaba primer registro true con key 'oom', obtenido %v, key: %s", recorded, key)
	}

	// 10 llamadas subsecuentes idénticas (simulando 10 ticks de refresco)
	for i := 0; i < 10; i++ {
		rec, _ := journal.Record(c, "OOMKilled", "RULE")
		if rec {
			t.Fatalf("violación de deduplicación en iteración %d: registró evento duplicado", i)
		}
	}

	count := journal.Count(c.ID, "oom", 1*time.Hour)
	if count != 1 {
		t.Fatalf("se esperaba Count == 1 tras 10 ticks duplicados, obtenido: %d", count)
	}
}

func TestCrashJournal_RecurrenceThreshold(t *testing.T) {
	journal := NewCrashJournal()

	// 1. Primer crash
	c1 := domain.ContainerMetric{
		ID:              "c-rec",
		Name:            "api-service",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-30 * time.Minute),
	}
	journal.Record(c1, "OOM 1", "RULE")
	isRec, count := journal.IsRecurrent(c1.ID, "oom")
	if isRec || count != 1 {
		t.Fatalf("con 1 evento no debe ser recurrente, isRec: %v, count: %d", isRec, count)
	}

	// 2. Segundo crash (dentro de 1h)
	c2 := domain.ContainerMetric{
		ID:              "c-rec",
		Name:            "api-service",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-15 * time.Minute),
	}
	journal.Record(c2, "OOM 2", "RULE")
	isRec, count = journal.IsRecurrent(c2.ID, "oom")
	if isRec || count != 2 {
		t.Fatalf("con 2 eventos no debe ser recurrente (reinicio de deploy normal), isRec: %v, count: %d", isRec, count)
	}

	// 3. Tercer crash (dentro de 1h) -> Dispara recurrencia
	c3 := domain.ContainerMetric{
		ID:              "c-rec",
		Name:            "api-service",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-2 * time.Minute),
	}
	journal.Record(c3, "OOM 3", "RULE")
	isRec, count = journal.IsRecurrent(c3.ID, "oom")
	if !isRec || count != 3 {
		t.Fatalf("con 3 eventos DEBE ser recurrente, isRec: %v, count: %d", isRec, count)
	}
}

func TestCrashJournal_Homogeneity(t *testing.T) {
	journal := NewCrashJournal()

	// 2 eventos OOM
	journal.Record(domain.ContainerMetric{
		ID:              "c-hom",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-20 * time.Minute),
	}, "", "")
	journal.Record(domain.ContainerMetric{
		ID:              "c-hom",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-10 * time.Minute),
	}, "", "")

	if homog := journal.Homogeneity("c-hom", 1*time.Hour); homog != "todos OOM" {
		t.Fatalf("se esperaba 'todos OOM', obtenido: %s", homog)
	}

	// Añadir un exit-1 distinto -> debe pasar a 'mixtos'
	journal.Record(domain.ContainerMetric{
		ID:              "c-hom",
		Status:          "Exited (1)",
		LastStateChange: time.Now().Add(-5 * time.Minute),
	}, "", "")

	if homog := journal.Homogeneity("c-hom", 1*time.Hour); homog != "mixtos" {
		t.Fatalf("se esperaba 'mixtos', obtenido: %s", homog)
	}
}

func TestCrashJournal_FormatPromptBlock(t *testing.T) {
	journal := NewCrashJournal()

	// 1. Sin eventos -> debe retornar cadena vacía
	if block := journal.FormatPromptBlock("empty-container", ""); block != "" {
		t.Fatalf("se esperaba bloque vacío para contenedor sin historial, obtenido: %s", block)
	}

	// 2. Con eventos registrados
	journal.Record(domain.ContainerMetric{
		ID:              "c-ai",
		Name:            "ai-worker",
		Status:          "Exited (137)",
		LastStateChange: time.Now().Add(-10 * time.Minute),
	}, "OOM por límite insuficiente", "RULE")

	block := journal.FormatPromptBlock("ai-worker", "creciente sostenida")
	if !strings.Contains(block, "Historial reciente:") {
		t.Errorf("bloque debe contener 'Historial reciente:'")
	}
	if !strings.Contains(block, "todos OOM") {
		t.Errorf("bloque debe contener homogeneidad 'todos OOM'")
	}
	if !strings.Contains(block, "hipótesis previa: OOM por límite insuficiente") {
		t.Errorf("bloque debe contener hipótesis previa")
	}
	if !strings.Contains(block, "tendencia RAM: creciente sostenida") {
		t.Errorf("bloque debe contener tendencia RAM")
	}
	if !strings.Contains(block, "INSTRUCCIÓN: la hipótesis previa es hipótesis, no verdad.") {
		t.Errorf("bloque debe contener la salvaguarda obligatoria de instrucción")
	}
}

func TestCrashJournal_Volatility(t *testing.T) {
	// Reiniciar el agente significa instanciar un nuevo NewCrashJournal()
	journal1 := NewCrashJournal()
	journal1.Record(domain.ContainerMetric{
		ID:              "c-vol",
		Status:          "Exited (137)",
		LastStateChange: time.Now(),
	}, "OOM", "RULE")

	if journal1.Count("c-vol", "oom", 1*time.Hour) != 1 {
		t.Fatalf("journal1 debe tener 1 evento")
	}

	// Simular reinicio
	journal2 := NewCrashJournal()
	if journal2.Count("c-vol", "oom", 1*time.Hour) != 0 {
		t.Fatalf("journal2 (nuevo agente) debe estar limpio en memoria (H2)")
	}
	if last := journal2.LastEvent("c-vol"); last != nil {
		t.Fatalf("nuevo agente no debe tener eventos previos")
	}
}
