package service

import (
	"testing"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

func TestDependencyGraph_InferenceAndInversion(t *testing.T) {
	containers := []domain.ContainerMetric{
		{
			ID:             "db-123456",
			Name:           "solv_db",
			Status:         "Exited (0)",
			Networks:       []string{"solv_net"},
			NetworkAliases: []string{"db", "database"},
			Ports:          []string{"127.0.0.1:5432->5432"},
		},
		{
			ID:             "api-123456",
			Name:           "api-node",
			Status:         "running",
			Networks:       []string{"solv_net", "public_net"},
			NetworkAliases: []string{"api"},
			EnvVars: []string{
				"DATABASE_URL=postgres://user:pass@db:5432/tracker",
				"PORT=3000",
			},
			Ports: []string{"0.0.0.0:3000->3000"},
		},
		{
			ID:       "grafana-123",
			Name:     "grafana",
			Status:   "running",
			Networks: []string{"solv_net"},
			EnvVars: []string{
				"GF_DATABASE_HOST=solv_db:5432",
			},
			Ports: []string{"0.0.0.0:3001->3000"},
		},
		{
			ID:       "isolated-99",
			Name:     "isolated-app",
			Status:   "running",
			Networks: []string{"isolated_net"},
			EnvVars: []string{
				"DATABASE_URL=postgres://db:5432/tracker", // Sin red compartida
			},
		},
	}

	g := NewDependencyGraph(containers)

	// 1. api-node debe depender de solv_db (a través del alias "db" en solv_net)
	apiDeps := g.GetDependencies(containers[1])
	if len(apiDeps) != 1 || apiDeps[0] != "solv_db" {
		t.Fatalf("se esperaba que api-node dependiera de [solv_db], obtenido: %v", apiDeps)
	}

	// 2. grafana debe depender de solv_db (a través del nombre "solv_db" en solv_net)
	grafDeps := g.GetDependencies(containers[2])
	if len(grafDeps) != 1 || grafDeps[0] != "solv_db" {
		t.Fatalf("se esperaba que grafana dependiera de [solv_db], obtenido: %v", grafDeps)
	}

	// 3. solv_db no depende de nadie
	dbDeps := g.GetDependencies(containers[0])
	if len(dbDeps) != 0 {
		t.Fatalf("se esperaba que solv_db no tuviera dependencias, obtenido: %v", dbDeps)
	}

	// 4. Inversión: de solv_db dependen api-node y grafana
	dbDependents := g.GetDependents(containers[0])
	if len(dbDependents) != 2 || dbDependents[0] != "api-node" || dbDependents[1] != "grafana" {
		t.Fatalf("se esperaba que de solv_db dependieran [api-node grafana], obtenido: %v", dbDependents)
	}

	// 5. Falso positivo controlado: isolated-app NO debe depender de solv_db porque no comparten red
	isoDeps := g.GetDependencies(containers[3])
	if len(isoDeps) != 0 {
		t.Fatalf("se esperaba que isolated-app tuviera 0 dependencias por no compartir red, obtenido: %v", isoDeps)
	}
}

func TestDependencyGraph_ImpactRadius(t *testing.T) {
	containers := []domain.ContainerMetric{
		{
			ID:       "c1",
			Name:     "db",
			Networks: []string{"solv_net"},
		},
		{
			ID:       "c2",
			Name:     "api",
			Networks: []string{"solv_net"},
		},
		{
			ID:       "c3",
			Name:     "worker",
			Networks: []string{"solv_net"},
		},
		{
			ID:       "c4",
			Name:     "other",
			Networks: []string{"other_net"},
		},
	}

	g := NewDependencyGraph(containers)

	count, text := g.GetImpactRadius(containers[0])
	if count != 2 {
		t.Fatalf("se esperaba radio de impacto 2, obtenido %d", count)
	}
	if text != "2 contenedores en solv_net" {
		t.Fatalf("texto de impacto inesperado: %s", text)
	}
}

func TestDependencyGraph_PortOwnershipAndConflict(t *testing.T) {
	containers := []domain.ContainerMetric{
		{
			ID:     "c-first",
			Name:   "nginx-prod",
			Status: "Up 2 hours",
			Ports:  []string{"0.0.0.0:8080->80", "127.0.0.1:5432->5432"},
		},
		{
			ID:     "c-second",
			Name:   "nginx-test",
			Status: "Exited (1)",
			Ports:  []string{"0.0.0.0:8080->80"},
		},
	}

	g := NewDependencyGraph(containers)

	// nginx-prod (running) debe ser dueño del puerto 8080 y 5432
	owners := g.GetPortOwners()
	if owners[8080] != "nginx-prod" {
		t.Fatalf("se esperaba que 8080 fuera de nginx-prod, obtenido: %s", owners[8080])
	}
	if owners[5432] != "nginx-prod" {
		t.Fatalf("se esperaba que 5432 fuera de nginx-prod, obtenido: %s", owners[5432])
	}

	// nginx-test (exited) debe tener conflicto en el puerto 8080 con nginx-prod
	conflicts := g.GetPortConflicts(containers[1])
	if len(conflicts) != 1 || conflicts[8080] != "nginx-prod" {
		t.Fatalf("se esperaba conflicto en 8080 retenido por nginx-prod, obtenido: %v", conflicts)
	}

	// nginx-prod no tiene conflicto consigo mismo
	prodConflicts := g.GetPortConflicts(containers[0])
	if len(prodConflicts) != 0 {
		t.Fatalf("nginx-prod no debería tener conflicto consigo mismo, obtenido: %v", prodConflicts)
	}
}

func TestPublishedPort_Glyph(t *testing.T) {
	pLoop, ok := ParsePublishedPort("127.0.0.1:5432->5432")
	if !ok || pLoop.Glyph() != "[OK] loopback" {
		t.Fatalf("se esperaba [OK] loopback, obtenido: %s", pLoop.Glyph())
	}

	pExp, ok := ParsePublishedPort("0.0.0.0:8080->8080")
	if !ok || pExp.Glyph() != "[||] expuesto" {
		t.Fatalf("se esperaba [||] expuesto, obtenido: %s", pExp.Glyph())
	}
}
