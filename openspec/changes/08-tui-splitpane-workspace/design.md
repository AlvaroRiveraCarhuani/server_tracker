# Diseño Técnico: TUI Split-Pane Workspace (08-tui-splitpane-workspace)

## 1. Arquitectura Visual y Layout Split-Pane

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│ SOLV SERVER TRACKER :: OPERATOR WORKSPACE                              Host: prod-01   │
├──────────────────────────────────────┬─────────────────────────────────────────────────┤
│ FLOTA (4)                            │ [󱤓 POSTGRESQL]  solv_db  [RUNNING]              │
│                                      │ Imagen: postgres:16-alpine | ID: b7e1b91191f1   │
│ > [OK] 󰡨 solv_api           12%     ├─────────────────────────────────────────────────┤
│   [OK] 󱤓 solv_db             4%     │ METRICAS EN TIEMPO REAL:                        │
│   [--]  redis_cache         0%     │   CPU:  12.5% [████░░░░░░] [ ▂▃▅] ▲ +2.1%/s     │
│   [!!] 󱡠 traefik_gw         85%     │   RAM: 256 MB / 1024 MB [████░░░░░░░░] 25.0%    │
│                                      │   RED: Salida: 12.4 KB/s                        │
│                                      │                                                 │
│                                      │ ACCIONES DISPONIBLES:                           │
│                                      │   [l/Enter] Logs en vivo  •  [e] Shell          │
├──────────────────────────────────────┴─────────────────────────────────────────────────┤
│ [AIOps] Diagnóstico en tiempo real: saturación de memoria por working set alto.        │
├────────────────────────────────────────────────────────────────────────────────────────┤
│ [j/k]: Navegar | [e]: Shell | [r]: Restart | [s]: Stop | [c]: Clave IA | [/]: Filtro   │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

### Cálculo de Dimensiones y Paginación
- `leftWidth = min(40, max(36, int(float64(m.width) * 0.32)))`
- `rightWidth = max(40, m.width - leftWidth - 3)`
- Ancho interno útil de la lista izquierda: `leftWidth - 4` caracteres fijos (garantía anti-wrap).
- Paginación vertical en ventana deslizante centrada en el cursor para no exceder la altura de la terminal ni provocar scrolling no deseado.
- Si la terminal es muy angosta (`m.width < 80`), la TUI conmuta automáticamente a la lista tabular tradicional colapsada.

---

## 2. Detección y Renderizado de Badges Tipográficas y Nerd Fonts

Se implementa el submódulo `agent/internal/delivery/tui/ascii_logos.go` con `TechnologyRegistry` y soporte para glifos vectoriales Nerd Font con paleta Catppuccin Mocha:
- `postgresql`: `󱤓` (ColorBlue)
- `docker`: `󰡨` (ColorBlue)
- `redis`: `` (ColorRed)
- `nginx`: `` (ColorGreen)
- `traefik`: `󱡠` (ColorTeal)
- `mysql`: `` (ColorPeach)
- `mongodb`: `` (ColorGreen)
- `node`: `` (ColorGreen)
- `python`: `` (ColorYellow)
- `golang`: `󰟓` (ColorTeal)
- Fallback para imágenes genéricas: `󰡨` (ColorLavender).

---

## 3. Salto Interactivo a Shell Local (`tea.ExecProcess`)

### Flujo de Ejecución Seguro (Cero RCE)
El comando no pasa por red ni protocolos web; es un proceso hijo directo de la TTY del operador:

```
[ Usuario pulsa 'e' ]
         │
         ▼
[ Obtiene ID del contenedor seleccionado ]
         │
         ▼
[ Invoca tea.ExecProcess(exec.Command("docker", "exec", "-it", id, "sh")) ]
         │
         ├── Suspende Bubbletea (libera terminal nativa)
         ├── Conecta stdin/stdout/stderr a la TTY del usuario
         └── Al salir el usuario con 'exit':
                  │
                  ▼
         [ Restaura TUI Bubbletea sin parpadeos ni pérdidas de estado ]
```

---

## 4. Modal de Configuración In-TUI de la API Key (`c`)

### Flujo de Persistencia
1. El usuario pulsa `c` en cualquier momento de la TUI.
2. Se abre un modal flotante con `textinput.Model` configurado en `EchoMode = textinput.EchoPassword`.
3. Al dar `Enter`:
   - Se cifra con AES-256-GCM y clave derivada con Argon2id.
   - Se almacena en la bóveda segura del agente (`~/.solv/vault.enc` con permisos `0600`).
   - Se inyecta inmediatamente al `TriageClient` en memoria.
   - El banner AIOps se activa en caliente sin reiniciar la aplicación.
