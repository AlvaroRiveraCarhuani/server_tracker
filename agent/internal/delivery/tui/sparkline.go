package tui

import (
	"strings"
)

var sparkGlyphs = []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

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

	var b strings.Builder
	numGlyphs := len(sparkGlyphs)

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

		b.WriteString(style.Render(string(sparkGlyphs[glyphIdx])))
	}

	return b.String()
}
