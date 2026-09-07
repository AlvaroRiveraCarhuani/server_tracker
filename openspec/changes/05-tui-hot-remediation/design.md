# Diseño Técnico: Remediación en Caliente por Teclado en la TUI (05-tui-hot-remediation)

## Máquina de Estados de la TUI

```
┌────────────────────────────────────────────────────────┐
│                   stateFleetTable                      │
└──────┬──────────────────────┬───────────────────┬──────┘
       │ [l / Enter]          │ [/]               │ [r / s / x]
       ▼                      ▼                   ▼
┌──────────────┐       ┌──────────────┐    ┌───────────────────────────┐
│stateLogViewer│       │stateFiltering│    │  stateConfirmRemediation  │
└──────────────┘       └──────────────┘    └─────────────┬─────────────┘
                                                         │
                                        [y / Enter]      │      [n / Esc]
                                        (Confirmar)      │      (Cancelar)
                                                         ▼
                                           Dispara tea.Cmd asíncrono
                                           y vuelve a stateFleetTable
```

## Estructura de Mensajes y Estados

```go
type remediationAction domain.ActionType

type remediationResultMsg struct {
    action        domain.ActionType
    containerName string
    elapsed       time.Duration
    err           error
}
```

## Diseño Visual del Modal Flotante

```
╭────────────────────────────────────────────────────────╮
│  [!!] CONFIRMAR ACCION: RESTART                        │
│                                                        │
│  ¿Deseas reiniciar el contenedor "solv_db"?            │
│  ID: 307c46519f67                                      │
│                                                        │
│  [ y / Enter ] Confirmar       [ n / Esc ] Cancelar    │
╰────────────────────────────────────────────────────────╯
```
- Borde: Catppuccin Peach (`#fab387`) para advertencia.
- Fondo/Texto: Catppuccin Text (`#cdd6f4`) sobre Surface (`#313244`).
