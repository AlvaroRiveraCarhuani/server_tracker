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
	if len(out) == 0 {
		t.Errorf("expected output for maxPoints=3, got empty")
	}
}

func TestRenderGradientBar(t *testing.T) {
	// 0% debe tener corchetes y caracteres tenues
	zeroBar := RenderGradientBar(0, 100, 10)
	if !strings.HasPrefix(zeroBar, "[") || !strings.HasSuffix(zeroBar, "]") {
		t.Fatalf("expected brackets in gradient bar, got %s", zeroBar)
	}

	// 50% debe contener bloques
	midBar := RenderGradientBar(50, 100, 10)
	if !strings.Contains(midBar, "█") {
		t.Errorf("expected 50%% bar to contain filled blocks '█', got %s", midBar)
	}

	// 100% debe estar completamente lleno
	fullBar := RenderGradientBar(100, 100, 10)
	if strings.Contains(fullBar, "░") {
		t.Errorf("expected 100%% bar to have no empty slots '░', got %s", fullBar)
	}
}

func TestMetricHistory_TrendVectorAndPeak(t *testing.T) {
	h := &MetricHistory{}

	// Menos de 2 muestras -> estable
	trend0 := h.CalculateCPUTrend()
	if trend0.Symbol != "≈" {
		t.Errorf("expected ≈ for empty history, got %s", trend0.Symbol)
	}

	// Añadir subida brusca: 10.0 -> 25.0 (+15.0%)
	h.AddSample(10.0, 120.0)
	h.AddSample(25.0, 150.0)

	trendUp := h.CalculateCPUTrend()
	if trendUp.Symbol != "▲" || trendUp.Delta != 15.0 {
		t.Errorf("expected ▲ with delta 15.0, got %s delta %f", trendUp.Symbol, trendUp.Delta)
	}

	// Añadir bajada: 25.0 -> 12.0 (-13.0%)
	h.AddSample(12.0, 200.0)
	trendDown := h.CalculateCPUTrend()
	if trendDown.Symbol != "▼" || trendDown.Delta != -13.0 {
		t.Errorf("expected ▼ with delta -13.0, got %s delta %f", trendDown.Symbol, trendDown.Delta)
	}

	// Verificar Mín, Máx y Pico RAM
	minCPU, maxCPU := h.CPUMinMax()
	if minCPU != 10.0 || maxCPU != 25.0 {
		t.Errorf("expected min 10.0 max 25.0, got min %f max %f", minCPU, maxCPU)
	}

	peakRAM := h.PeakRAM()
	if peakRAM != 200.0 {
		t.Errorf("expected peak RAM 200.0, got %f", peakRAM)
	}
}

