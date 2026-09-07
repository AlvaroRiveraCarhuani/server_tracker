package tui

import (
	"strings"
	"testing"
)

func TestMetricHistory_FIFO_Capacity(t *testing.T) {
	h := &MetricHistory{}

	for i := 1; i <= 45; i++ {
		h.AddSample(float64(i), float64(i*10))
	}

	if len(h.CPU) != maxHistorySamples {
		t.Fatalf("expected CPU history length %d, got %d", maxHistorySamples, len(h.CPU))
	}
	if len(h.RAM) != maxHistorySamples {
		t.Fatalf("expected RAM history length %d, got %d", maxHistorySamples, len(h.RAM))
	}

	// Las primeras 15 deben haber sido descartadas; el primer valor debe ser 16
	if h.CPU[0] != 16.0 {
		t.Errorf("expected oldest CPU sample to be 16.0, got %f", h.CPU[0])
	}
	if h.CPU[len(h.CPU)-1] != 45.0 {
		t.Errorf("expected newest CPU sample to be 45.0, got %f", h.CPU[len(h.CPU)-1])
	}
}

func TestRenderSparkline_Empty(t *testing.T) {
	out := RenderSparkline(nil, 0, 100, 10)
	if out != "" {
		t.Errorf("expected empty string for nil slice, got %q", out)
	}

	out = RenderSparkline([]float64{}, 0, 100, 10)
	if out != "" {
		t.Errorf("expected empty string for empty slice, got %q", out)
	}
}

func TestRenderSparkline_ClampingAndScaling(t *testing.T) {
	samples := []float64{0.0, 50.0, 100.0}
	out := RenderSparkline(samples, 0, 100, 3)

	// La salida debe contener los glifos correspondientes al min, medio y max
	if !strings.Contains(out, " ") {
		t.Errorf("expected sparkline to contain minimum glyph ' ', got: %s", out)
	}
	if !strings.Contains(out, "█") {
		t.Errorf("expected sparkline to contain maximum glyph '█', got: %s", out)
	}
}

func TestRenderSparkline_IdenticalValues(t *testing.T) {
	samples := []float64{25.0, 25.0, 25.0}
	out := RenderSparkline(samples, 25.0, 25.0, 3)
	if len(out) == 0 {
		t.Errorf("expected non-empty output for identical values, got empty")
	}
}

func TestRenderSparkline_MaxPoints(t *testing.T) {
	samples := []float64{10, 20, 30, 40, 50, 60, 70}
	// Limitar a los últimos 3 puntos
	out := RenderSparkline(samples, 0, 100, 3)
	// RenderSparkline devuelve una cadena con estilos Lipgloss; al menos debe renderizar sin pánico
	if len(out) == 0 {
		t.Errorf("expected output for maxPoints=3, got empty")
	}
}
