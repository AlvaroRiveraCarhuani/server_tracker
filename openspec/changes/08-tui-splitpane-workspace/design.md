# Diseño Técnico: TUI Split-Pane Workspace (08-tui-splitpane-workspace)

## 1. Arquitectura Visual y Layout Split-Pane

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│ SOLV SERVER TRACKER :: OPERATOR WORKSPACE                              Host: prod-01   │
├──────────────────────────────┬─────────────────────────────────────────────────────────┤
│ [FLOTA DE CONTENEDORES]      │ [FICHA DE INSPECCIÓN ACTIVA]                            │
│                              │                                                         │
│ > [OK] solv_api     12% [■░] │ solv_api (solv/api:v1.2)           [ LOGO ASCII ]       │
│   [OK] solv_db      24% [■■] │ ID: b7e1b91191f1 | Uptime: 4d 12h  [  PostgreSQL]       │
│   [--] redis_cache   0% [░░] ├─────────────────────────────────────────────────────────┤
│   [!!] payment_svc   0% [░░] │ METRICAS EN TIEMPO REAL:                                │
│                              │  CPU:  12.5% [ ▂▃▅█] ▲ +2.1%/s  (Min: 0.5% | Max: 32%)  │
│                              │  RAM: 256MB / 1024MB [████░░░░░░░░] 25.0%               │
│                              │  NET: 12.4 KB/s [Normal]                                │
│                              ├─────────────────────────────────────────────────────────┤
│                              │ REGISTROS EN VIVO (Tail 8):                             │
│                              │  2026-09-07 20:50:11 [INFO] Worker pool initialized     │
│                              │  2026-09-07 20:50:12 [INFO] Listening on :8080          │
│                              │  2026-09-07 20:51:00 [INFO] Healthcheck OK              │
├──────────────────────────────┴─────────────────────────────────────────────────────────┤
│ [AIOps]: Triaje pasivo de incidentes contextual en 1 línea                             │
├────────────────────────────────────────────────────────────────────────────────────────┤
│ [j/k]: Navegar | [e]: Shell | [r]: Restart | [s]: Stop | [c]: Config API | [/]: Filtro │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

### Cálculo de Dimensiones Adaptativas
- `leftWidth = max(26, int(float64(m.width) * 0.28))`
- `rightWidth = max(40, m.width - leftWidth - 6)`
- Si la terminal es muy angosta (`m.width < 80`), la TUI conmuta automáticamente a la lista tabular tradicional colapsada.

---

## 2. Detección y Renderizado de Logotipos ASCII

Se implementa el submódulo `agent/internal/delivery/tui/ascii_logos.go` con arte ASCII compacto (máximo 5 líneas de alto x 18 caracteres de ancho) para las tecnologías más comunes:
- `postgres` (elefante estilizado)
- `redis` (bloques apilados)
- `nginx` (glifos simétricos)
- `traefik` (puente/gateway)
- `node` / `javascript` (hexágono)
- `golang` (mascota/gopher minimalista)
- `python` (serpientes cruzadas)
- `docker` (ballena con contenedores)
- Fallback para imágenes genéricas.

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
