# Diseño Técnico: TUI de Baja Carga Cognitiva (03-tui-ux-low-cognitive-load)

## Arquitectura de Componentes de la TUI

```
agent/internal/delivery/tui/
├── tui.go                   # Model principal con máquina de estados (Fleet, Logs, Filter)
├── theme.go                 # Constantes de color Catppuccin Mocha y estilos precomputados
├── filter.go                # Manejo del input de búsqueda con '/'
└── logs.go                  # Componente viewport.Model para Live Tail de Docker
```

## Máquina de Estados del Modelo TUI

```
                    ┌─────────────────────────┐
                    │    stateFleetTable      │ ◄────────┐
                    │ (Vista Master de Flota) │          │
                    └────────────┬────────────┘          │
                                 │                       │
                tecla '/'        │ tecla 'l' o 'Enter'   │ tecla 'Esc' o 'h'
         ┌───────────────────────┴──────────┐            │
         ▼                                  ▼            │
┌──────────────────┐               ┌──────────────────┐  │
│  stateFiltering  │               │ stateLogViewer   ├──┘
│ (Input reactivo) │               │(Logs fullscreen) │
└──────────────────┘               └──────────────────┘
```

## Decisiones de Arquitectura (ADRs)

### ADR-06: Model Swapping vs Paneles Divididos
- **Contexto**: Las herramientas que dividen la pantalla en cuadrantes (Lazydocker) comprimen los logs y saturan la atención.
- **Decisión**: Se implementa *Model Swapping* en Bubbletea. La tabla y el visor de logs usan el 100% del viewport de la terminal de manera excluyente, con migas de pan en el Header para preservar el contexto mental.

### ADR-07: Semáforo Cromático de Egress y Detección OOM
- **Contexto**: Un número plano en texto no permite detección en 1 segundo de anomalías financieras ni fallos de memoria.
- **Decisión**: Se calculan estilos dinámicos:
  - RAM > 85% $\rightarrow$ Estado `[!!] OOM_RISK` en Rojo (`#f38ba8`).
  - Egress > 50 MB/s $\rightarrow$ Rojo (`#f38ba8`) Bold; Egress 5-50 MB/s $\rightarrow$ Peach (`#fab387`).
