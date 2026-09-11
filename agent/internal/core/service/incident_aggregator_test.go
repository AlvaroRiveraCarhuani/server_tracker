package service

import (
	"testing"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestIncidentAggregator_GroupingAndTopology(t *testing.T) {
	agg := NewIncidentAggregator(30)
	now := time.Now()

	// Contenedores anómalos en el mismo compose project "stack_web"
	c1 := domain.ContainerMetric{
		ID:              "c1",
		Name:            "postgres",
		Status:          "Exited (137)",
		ComposeProject:  "stack_web",
		Networks:        []string{"web_net"},
		LastStateChange: now.Add(-10 * time.Second),
	}
	c2 := domain.ContainerMetric{
		ID:              "c2",
		Name:            "api-node",
		Status:          "Exited (1)",
		ComposeProject:  "stack_web",
		Networks:        []string{"web_net"},
		LastStateChange: now.Add(-5 * time.Second),
	}

	// Contenedor anómalo en otra red y sin compose (coincidencia sin topología)
	c3 := domain.ContainerMetric{
		ID:              "c3",
		Name:            "isolated-worker",
		Status:          "Exited (1)",
		ComposeProject:  "",
		Networks:        []string{"isolated_net"},
		LastStateChange: now.Add(-5 * time.Second),
	}

	metrics := []domain.ContainerMetric{c1, c2, c3}
	graph := NewDependencyGraph(metrics)

	opened := agg.IngestAnomalies(metrics, graph, nil, now)
	if len(opened) != 1 {
		t.Fatalf("expected 1 incident opened, got %d", len(opened))
	}

	inc := opened[0]
	if inc.GroupName != "stack_web" {
		t.Errorf("expected group name 'stack_web', got %q", inc.GroupName)
	}
	if len(inc.Members) != 4 { // c1.Name, c1.ID, c2.Name, c2.ID
		t.Errorf("expected 4 entries in members map, got %d", len(inc.Members))
	}

	// isolated-worker no debe ser miembro
	if agg.GetActiveIncidentFor("isolated-worker") != nil {
		t.Errorf("expected isolated-worker to NOT be in any active incident")
	}

	// postgres y api-node deben pertenecer al mismo incidente
	incP := agg.GetActiveIncidentFor("postgres")
	incA := agg.GetActiveIncidentFor("api-node")
	if incP == nil || incA == nil || incP.ID != incA.ID {
		t.Errorf("expected postgres and api-node to share the same incident")
	}
}

func TestIncidentAggregator_CoincidenceWithoutTopology_NoIncident(t *testing.T) {
	agg := NewIncidentAggregator(30)
	now := time.Now()

	// Dos contenedores sin compose ni redes compartidas
	c1 := domain.ContainerMetric{
		ID:              "c1",
		Name:            "db-service",
		Status:          "Exited (1)",
		ComposeProject:  "proj-a",
		Networks:        []string{"net-a"},
		LastStateChange: now,
	}
	c2 := domain.ContainerMetric{
		ID:              "c2",
		Name:            "cache-service",
		Status:          "Exited (1)",
		ComposeProject:  "proj-b",
		Networks:        []string{"net-b"},
		LastStateChange: now,
	}

	metrics := []domain.ContainerMetric{c1, c2}
	graph := NewDependencyGraph(metrics)

	opened := agg.IngestAnomalies(metrics, graph, nil, now)
	if len(opened) != 0 {
		t.Fatalf("expected 0 incidents opened for containers without hard topology, got %d", len(opened))
	}

	if agg.GetActiveIncidentFor("db-service") != nil {
		t.Errorf("expected no active incident for db-service")
	}
	if agg.GetActiveIncidentFor("cache-service") != nil {
		t.Errorf("expected no active incident for cache-service")
	}
}

func TestIncidentAggregator_SlowCascadeAdjunctionWithGraphEdge(t *testing.T) {
	agg := NewIncidentAggregator(30)
	t0 := time.Now().Add(-70 * time.Second)

	// Iniciar incidente con postgres y redis en solv_net
	c1 := domain.ContainerMetric{
		ID:              "c1",
		Name:            "postgres",
		Status:          "Exited (137)",
		Networks:        []string{"solv_net"},
		LastStateChange: t0,
	}
	c2 := domain.ContainerMetric{
		ID:              "c2",
		Name:            "redis",
		Status:          "Exited (1)",
		Networks:        []string{"solv_net"},
		LastStateChange: t0.Add(5 * time.Second),
	}

	metrics0 := []domain.ContainerMetric{c1, c2}
	graph0 := NewDependencyGraph(metrics0)
	opened := agg.IngestAnomalies(metrics0, graph0, nil, t0)
	if len(opened) != 1 {
		t.Fatalf("expected 1 incident opened at t0, got %d", len(opened))
	}

	// 70 segundos después (fuera de ventana de 30s, pero < 5m): api-node falla y tiene arista hacia postgres
	t1 := time.Now()
	c3 := domain.ContainerMetric{
		ID:              "c3",
		Name:            "api-node",
		Status:          "Exited (1)",
		Networks:        []string{"solv_net"},
		EnvVars:         []string{"DB_HOST=postgres"},
		LastStateChange: t1,
	}

	allMetrics := []domain.ContainerMetric{c1, c2, c3}
	graph1 := NewDependencyGraph(allMetrics)

	// api-node debe adjuntarse gracias a la arista depende-de y tiempo < 5 min
	agg.IngestAnomalies([]domain.ContainerMetric{c3}, graph1, nil, t1)

	incApi := agg.GetActiveIncidentFor("api-node")
	if incApi == nil {
		t.Fatalf("expected api-node to be attached to active incident")
	}
	if incApi.ID != opened[0].ID {
		t.Errorf("expected api-node to be attached to initial incident %s, got %s", opened[0].ID, incApi.ID)
	}
}

func TestIncidentAggregator_ConfidenceStates(t *testing.T) {
	// Caso 1: Confirmado (evento más antiguo + arista de grafo corroborando)
	t.Run("Confirmado", func(t *testing.T) {
		agg := NewIncidentAggregator(30)
		now := time.Now()

		c1 := domain.ContainerMetric{
			ID:              "c1",
			Name:            "postgres",
			Status:          "Exited (137)",
			Networks:        []string{"solv_net"},
			LastStateChange: now.Add(-10 * time.Second),
		}
		c2 := domain.ContainerMetric{
			ID:              "c2",
			Name:            "api-node",
			Status:          "Exited (1)",
			Networks:        []string{"solv_net"},
			EnvVars:         []string{"DATABASE_URL=postgres://postgres:5432"},
			LastStateChange: now.Add(-2 * time.Second),
		}

		metrics := []domain.ContainerMetric{c1, c2}
		graph := NewDependencyGraph(metrics)

		opened := agg.IngestAnomalies(metrics, graph, nil, now)
		if len(opened) != 1 {
			t.Fatalf("expected 1 incident, got %d", len(opened))
		}
		if opened[0].OriginConfidence != domain.ConfidenceConfirmed {
			t.Errorf("expected ConfidenceConfirmed, got %v", opened[0].OriginConfidence)
		}
		if opened[0].CandidateOrigin != "postgres" {
			t.Errorf("expected CandidateOrigin postgres, got %q", opened[0].CandidateOrigin)
		}
	})

	// Caso 2: Probable (evento más antiguo claro, pero sin aristas de grafo)
	t.Run("Probable", func(t *testing.T) {
		agg := NewIncidentAggregator(30)
		now := time.Now()

		c1 := domain.ContainerMetric{
			ID:              "c1",
			Name:            "postgres",
			Status:          "Exited (137)",
			Networks:        []string{"solv_net"},
			LastStateChange: now.Add(-20 * time.Second),
		}
		c2 := domain.ContainerMetric{
			ID:              "c2",
			Name:            "cache",
			Status:          "Exited (1)",
			Networks:        []string{"solv_net"},
			LastStateChange: now.Add(-2 * time.Second),
		}

		metrics := []domain.ContainerMetric{c1, c2}
		graph := NewDependencyGraph(metrics) // Sin EnvVars de dependencia

		opened := agg.IngestAnomalies(metrics, graph, nil, now)
		if len(opened) != 1 {
			t.Fatalf("expected 1 incident, got %d", len(opened))
		}
		if opened[0].OriginConfidence != domain.ConfidenceProbable {
			t.Errorf("expected ConfidenceProbable, got %v", opened[0].OriginConfidence)
		}
	})

	// Caso 3: s/d (eventos simultáneos sin aristas de grafo)
	t.Run("SinDeterminar_Simultaneos", func(t *testing.T) {
		agg := NewIncidentAggregator(30)
		now := time.Now()

		c1 := domain.ContainerMetric{
			ID:              "c1",
			Name:            "worker-a",
			Status:          "Exited (1)",
			Networks:        []string{"solv_net"},
			LastStateChange: now,
		}
		c2 := domain.ContainerMetric{
			ID:              "c2",
			Name:            "worker-b",
			Status:          "Exited (1)",
			Networks:        []string{"solv_net"},
			LastStateChange: now, // Mismo tick exacto
		}

		metrics := []domain.ContainerMetric{c1, c2}
		graph := NewDependencyGraph(metrics)

		opened := agg.IngestAnomalies(metrics, graph, nil, now)
		if len(opened) != 1 {
			t.Fatalf("expected 1 incident, got %d", len(opened))
		}
		if opened[0].OriginConfidence != domain.ConfidenceUndetermined {
			t.Errorf("expected ConfidenceUndetermined, got %v", opened[0].OriginConfidence)
		}
	})
}

func TestIncidentAggregator_ManualOverride(t *testing.T) {
	agg := NewIncidentAggregator(30)
	now := time.Now()

	c1 := domain.ContainerMetric{
		ID:              "c1",
		Name:            "worker-a",
		Status:          "Exited (1)",
		Networks:        []string{"solv_net"},
		LastStateChange: now,
	}
	c2 := domain.ContainerMetric{
		ID:              "c2",
		Name:            "worker-b",
		Status:          "Exited (1)",
		Networks:        []string{"solv_net"},
		LastStateChange: now,
	}

	metrics := []domain.ContainerMetric{c1, c2}
	graph := NewDependencyGraph(metrics)
	opened := agg.IngestAnomalies(metrics, graph, nil, now)
	if len(opened) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(opened))
	}
	inc := opened[0]

	// Manual override sobre worker-b
	ok := agg.SetManualOrigin(inc.ID, "worker-b")
	if !ok {
		t.Fatalf("expected SetManualOrigin to succeed")
	}
	if inc.ManualOrigin != "worker-b" {
		t.Errorf("expected ManualOrigin 'worker-b', got %q", inc.ManualOrigin)
	}
	if inc.EffectiveOrigin() != "worker-b" {
		t.Errorf("expected EffectiveOrigin 'worker-b', got %q", inc.EffectiveOrigin())
	}
}

func TestIncidentAggregator_QuietPeriodClosure(t *testing.T) {
	agg := NewIncidentAggregator(10) // 10s ventana -> 20s quiet
	now := time.Now()

	c1 := domain.ContainerMetric{
		ID:              "c1",
		Name:            "svc1",
		Status:          "Exited (1)",
		Networks:        []string{"test_net"},
		LastStateChange: now,
	}
	c2 := domain.ContainerMetric{
		ID:              "c2",
		Name:            "svc2",
		Status:          "Exited (1)",
		Networks:        []string{"test_net"},
		LastStateChange: now,
	}

	opened := agg.IngestAnomalies([]domain.ContainerMetric{c1, c2}, nil, nil, now)
	if len(opened) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(opened))
	}

	// 15 segundos después: aún en período quieto (20s)
	closed := agg.CheckQuietPeriods(now.Add(15 * time.Second))
	if len(closed) != 0 {
		t.Errorf("expected no incidents closed at +15s, got %d", len(closed))
	}
	if agg.GetActiveIncidentFor("svc1") == nil {
		t.Errorf("expected incident to remain active at +15s")
	}

	// 25 segundos después: superó el período quieto -> debe cerrar
	closed = agg.CheckQuietPeriods(now.Add(25 * time.Second))
	if len(closed) != 1 {
		t.Errorf("expected 1 incident closed at +25s, got %d", len(closed))
	}
	if agg.GetActiveIncidentFor("svc1") != nil {
		t.Errorf("expected incident for svc1 to be closed")
	}
}

func TestFormatIncidentBanner_PoliciesAndConfidences(t *testing.T) {
	inc := &Incident{
		ID:               "inc-1",
		GroupName:        "solv_net",
		CandidateOrigin:  "postgres",
		OriginConfidence: domain.ConfidenceConfirmed,
		Cascade:          []string{"postgres", "api-node", "nginx"},
		Diagnosis: domain.DiagnosisResult{
			Level: domain.LevelAI,
		},
		Events: []IncidentEvent{
			{ContainerName: "postgres", Reason: "OOM"},
			{ContainerName: "api-node", Reason: "exit 1"},
			{ContainerName: "nginx", Reason: "502"},
		},
	}

	// 1. Confirmado
	bannerConf := FormatIncidentBanner(inc, domain.BannerPolicyInformativo, 80)
	if bannerConf != "[INC] solv_net · origen postgres (OOM) -> 2 · [AI] · [d]" {
		t.Errorf("unexpected confirmed banner: %s", bannerConf)
	}

	// 2. Probable + Informativo
	inc.OriginConfidence = domain.ConfidenceProbable
	bannerProbInfo := FormatIncidentBanner(inc, domain.BannerPolicyInformativo, 80)
	if bannerProbInfo != "[INC] solv_net · origen prob. postgres -> 2 · [AI] · [d]" {
		t.Errorf("unexpected probable informativo banner: %s", bannerProbInfo)
	}

	// 3. Probable + Prudente
	bannerProbPrud := FormatIncidentBanner(inc, domain.BannerPolicyPrudente, 80)
	if bannerProbPrud != "[INC] solv_net · 3 anómalos · [AI] · [d]" {
		t.Errorf("unexpected probable prudente banner: %s", bannerProbPrud)
	}

	// 4. s/d
	inc.OriginConfidence = domain.ConfidenceUndetermined
	bannerSD := FormatIncidentBanner(inc, domain.BannerPolicyInformativo, 80)
	if bannerSD != "[INC] solv_net · 3 anómalos · origen s/d · [AI] · [d]" {
		t.Errorf("unexpected s/d banner: %s", bannerSD)
	}
}
