package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	sparkGlyphs      = []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'} // U+2581 a U+2588
	sparkGlyphsASCII = []rune{'.', ':', '-', '=', '+', '*', '#', '%'}
)

const maxHistorySamples = 30

// MetricHistory almacena series de tiempo acotadas para un contenedor.
type MetricHistory struct {
	CPU []float64
	RAM []float64
}

// AddSample almacena una muestra de CPU (%) y RAM (MB) manteniendo tamaño fijo FIFO.
func (h *MetricHistory) AddSample(cpu, ram float64) {
	h.CPU = append(h.CPU, cpu)
	if len(h.CPU) > maxHistorySamples {
		h.CPU = h.CPU[len(h.CPU)-maxHistorySamples:]
	}

	h.RAM = append(h.RAM, ram)
	if len(h.RAM) > maxHistorySamples {
		h.RAM = h.RAM[len(h.RAM)-maxHistorySamples:]
	}
}

// CPUMinMax devuelve el mínimo y máximo de CPU registrado en el buffer.
func (h *MetricHistory) CPUMinMax() (min, max float64) {
	if len(h.CPU) == 0 {
		return 0, 0
	}
	min = h.CPU[0]
	max = h.CPU[0]
	for _, v := range h.CPU {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

// PeakRAM devuelve el pico histórico máximo de RAM en MB dentro del buffer.
func (h *MetricHistory) PeakRAM() float64 {
	var peak float64
	for _, v := range h.RAM {
		if v > peak {
			peak = v
		}
	}
	return peak
}

// TrendVector describe el delta direccional y estado de aceleración temporal.
type TrendVector struct {
	Delta  float64
	Symbol string
	Label  string
	Style  lipgloss.Style
}

// CalculateCPUTrend calcula la tasa de cambio entre las últimas dos muestras.
func (h *MetricHistory) CalculateCPUTrend() TrendVector {
	if len(h.CPU) < 2 {
		return TrendVector{
			Delta:  0.0,
			Symbol: "≈",
			Label:  "estable",
			Style:  lipgloss.NewStyle().Foreground(ColorSubtext0),
		}
	}

	n := len(h.CPU)
	delta := h.CPU[n-1] - h.CPU[n-2]

	if delta >= 2.0 {
		return TrendVector{
			Delta:  delta,
			Symbol: "▲",
			Label:  fmt.Sprintf("+%.1f%%/s", delta),
			Style:  lipgloss.NewStyle().Foreground(ColorPeach).Bold(true),
		}
	} else if delta <= -2.0 {
		return TrendVector{
			Delta:  delta,
			Symbol: "▼",
			Label:  fmt.Sprintf("%.1f%%/s", delta),
			Style:  lipgloss.NewStyle().Foreground(ColorGreen),
		}
	}

	return TrendVector{
		Delta:  delta,
		Symbol: "≈",
		Label:  "estable",
		Style:  lipgloss.NewStyle().Foreground(ColorSubtext0),
	}
}

// RenderGradientBar dibuja una barra proporcional con gradiente térmico posicional estilo btop.
// Segmentos: 0-60% verde, 60-80% durazno/amarillo, 80-100% rojo. Inactivo: ░ tenue con contraste garantizado.
func RenderGradientBar(current, limit float64, width int) string {
	if width <= 0 {
		return ""
	}
	if limit <= 0 {
		limit = 100.0
	}

	ratio := current / limit
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1.0 {
		ratio = 1.0
	}

	filled := int(math.Round(ratio * float64(width)))
	if current > 0 && filled == 0 {
		filled = 1
	}

	styleGreen := lipgloss.NewStyle().Foreground(ColorGreen)
	stylePeach := lipgloss.NewStyle().Foreground(ColorPeach)
	styleRed := lipgloss.NewStyle().Foreground(ColorRed)
	styleDim := lipgloss.NewStyle().Foreground(ColorOverlay0)

	var b strings.Builder
	b.WriteString("[")

	for i := 0; i < width; i++ {
		if i < filled {
			cellRatio := float64(i+1) / float64(width)
			if cellRatio <= 0.60 {
				b.WriteString(styleGreen.Render("█"))
			} else if cellRatio <= 0.80 {
				b.WriteString(stylePeach.Render("█"))
			} else {
				b.WriteString(styleRed.Render("█"))
			}
		} else {
			b.WriteString(styleDim.Render("░"))
		}
	}

	b.WriteString("]")
	return b.String()
}

// RenderSparkline genera una cadena gráfica con glifos de bloque a partir de los datos numéricos.
func RenderSparkline(values []float64, minVal, maxVal float64, maxPoints int) string {
	if len(values) == 0 {
		return ""
	}

	samples := values
	if maxPoints > 0 && len(samples) > maxPoints {
		samples = samples[len(samples)-maxPoints:]
	}

	if minVal >= maxVal {
		minVal = samples[0]
		maxVal = samples[0]
		for _, v := range samples {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
		if minVal == maxVal {
			if maxVal == 0 {
				maxVal = 1.0
			} else {
				minVal = 0
			}
		}
	}

	glyphs := sparkGlyphs
	if !NerdFontsMode {
		glyphs = sparkGlyphsASCII
	}
	numGlyphs := len(glyphs)

	var b strings.Builder

	for _, v := range samples {
		clamped := v
		if clamped < minVal {
			clamped = minVal
		}
		if clamped > maxVal {
			clamped = maxVal
		}

		ratio := (clamped - minVal) / (maxVal - minVal)
		glyphIdx := int(ratio * float64(numGlyphs-1))
		if glyphIdx < 0 {
			glyphIdx = 0
		} else if glyphIdx >= numGlyphs {
			glyphIdx = numGlyphs - 1
		}

		style := StyleSparklineNormal
		if ratio >= 0.85 {
			style = StyleSparklineDanger
		} else if ratio >= 0.65 {
			style = StyleSparklineWarning
		}

		b.WriteString(style.Render(string(glyphs[glyphIdx])))
	}

	return b.String()
}
