# Diseño Técnico: Sparklines y Tendencias en la TUI (07-tui-sparklines-history)

## Arquitectura de Datos

```
[ Tick de Muestreo (500ms) ]
             │
             ▼
[ []domain.ContainerMetric ]
             │
             ▼
    [ Model.metricsHistory map[string]*MetricHistory ]
             │
             ├── Capacidad máxima: 30 entradas FIFO
             ├── CPUHistory: []float64 (0.0 - 100.0%)
             └── RAMHistory: []float64 (MBs)
```

## Bloques ASCII y Mapeo
Los glifos utilizados para las sparklines corresponden a los caracteres de bloque Unicode en 8 niveles:
` ' ', '▂', '▃', '▄', '▅', '▆', '▇', '█' `

Dado un valor $V$, con $Min$ y $Max$:
$ratio = \frac{V - Min}{Max - Min}$ clamped en $[0.0, 1.0]$.
$index = \lfloor ratio \times 7 \rfloor$.

## Adaptabilidad de Columnas
- En terminales con ancho $\ge 110$ caracteres, la tabla principal incluye la columna `CPU TREND` (8 caracteres de sparkline).
- En terminales angostas (< 110), se priorizan las columnas esenciales para evitar truncados accidentales.
- En la vista de logs/detalle (`l`/`Enter`), se despliega una barra superior con la sparkline extendida de CPU (20 muestras) y RAM (20 muestras).
