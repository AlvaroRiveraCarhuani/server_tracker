package tui

import (
	"strings"
	"testing"
)

func TestTechnologyRegistry_ResolveKnown(t *testing.T) {
	registry := NewTechnologyRegistry()

	testCases := []struct {
		imageName     string
		containerName string
		expectedID    string
		expectedName  string
		expectedGlyph string
	}{
		{"postgres:16-alpine", "solv_db", "postgresql", "PostgreSQL", "󱤓"},
		{"timescale/timescaledb:latest", "tsdb", "postgresql", "PostgreSQL", "󱤓"},
		{"redis:7.2", "cache_redis", "redis", "Redis", ""},
		{"nginx:mainline", "web_proxy", "nginx", "NGINX", ""},
		{"traefik:v3.1", "edge_router", "traefik", "Traefik", "󱡠"},
		{"python:3.12-slim", "api_backend", "python", "Python", ""},
		{"node:20-alpine", "frontend_next", "node", "Node.js", ""},
		{"mysql:8.0", "legacy_db", "mysql", "MySQL", ""},
		{"golang:1.23", "worker_svc", "go", "Go", ""},
		{"docker:dind", "ci_runner", "docker", "Docker", "󰡨"},
		{"mongo:7.0", "app_mongo", "mongodb", "MongoDB", ""},
	}

	for _, tc := range testCases {
		tech := registry.Resolve(tc.imageName, tc.containerName)
		if tech.ID != tc.expectedID {
			t.Errorf("expected ID %s for (%s, %s), got %s", tc.expectedID, tc.imageName, tc.containerName, tech.ID)
		}
		if tech.Name != tc.expectedName {
			t.Errorf("expected Name %s, got %s", tc.expectedName, tech.Name)
		}
		if tech.NerdGlyph != tc.expectedGlyph {
			t.Errorf("expected Glyph %s, got %s", tc.expectedGlyph, tech.NerdGlyph)
		}
		badge := tech.Badge()
		if !strings.Contains(badge, tech.Name) {
			t.Errorf("expected badge to contain %s, got %s", tech.Name, badge)
		}
	}
}

func TestTechnologyRegistry_ResolveGenericFallback(t *testing.T) {
	registry := NewTechnologyRegistry()

	tech := registry.Resolve("ghcr.io/empresa/facturacion:v2.1", "facturacion")

	if tech.ID != "custom" {
		t.Errorf("expected ID 'custom', got %s", tech.ID)
	}
	if tech.Name != "FACTURACION" {
		t.Errorf("expected name 'FACTURACION', got %s", tech.Name)
	}
	badge := tech.Badge()
	if !strings.Contains(badge, "FACTURACION") {
		t.Errorf("expected badge to contain FACTURACION, got %s", badge)
	}
}

func TestDetectTechnology_Global(t *testing.T) {
	tech := DetectTechnology("postgres:15", "db")
	if tech.ID != "postgresql" {
		t.Errorf("expected postgresql from global helper, got %s", tech.ID)
	}
}
