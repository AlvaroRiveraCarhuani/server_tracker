# Diseño Técnico: Diagnóstico Pasivo Zero-Prompt con OpenRouter en la TUI (06-tui-zero-prompt-ai)

## Flujo de Datos en el Agente Go

```
[ Cursor sobre contenedor ]
            │
            ▼ ¿Está anómalo? (STOPPED / OOM_RISK / CRASH)
      SI ───┴─── NO ──> No hace nada (UI limpia)
      │
      ▼ ¿Está en cache?
      SI ───> Muestra diagnóstico guardado en el banner
      NO
      │
      ▼ Dispara tea.Cmd asíncrono
  [ Extrae logs tail 50 ] ──> [ HTTP POST OpenRouter (Gemma/OpenRouter) ]
                                            │
                                            ▼
                          [ Recibe diagnosisMsg ]
                                    │
                                    ▼
                         [ Guarda en cache ]
                                    │
                                    ▼
                     [ Renderiza Banner AIOps ]
```

## Formato del Banner en la TUI

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ [AIOps] Causa: Pool de conexiones agotado -> Aumentar max_connections       │
└─────────────────────────────────────────────────────────────────────────────┘
```
- Borde superior/inferior: Catppuccin Surface1 (`#45475a`).
- Etiqueta `[AIOps]`: Catppuccin Mauve (`#cba6f7`) en negrita.
- Texto: Catppuccin Subtext1 (`#bac2de`).
