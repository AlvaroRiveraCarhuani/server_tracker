# SOLV Server Tracker — Roadmap y Definición de Alcance

## 1. Visión y Destino
Construir una plataforma de **Observabilidad Activa, ChatOps y AIOps** para infraestructuras Docker On-Premise que combine:
- **Data Plane (Go)**: Agente ultraligero (<0.1% CPU), onboarding sin archivos `.env`, extracción matemática precisa de métricas desde `/var/run/docker.sock`, buffer circular FIFO ante caídas de red y TUI para trabajo profundo.
- **Control Plane (FastAPI)**: Servidor central con validación de firmas HMAC-SHA256, series de tiempo en PostgreSQL/TimescaleDB, servidor MCP para modelos de lenguaje y ChatOps en Telegram con tokens efímeros y cero RCE.

---

## 2. Matriz Comparativa de Mercado y Open Source

| Característica | SOLV Server Tracker | Lazydocker / ctop | Portainer | Datadog / New Relic | Prometheus + Grafana |
|---|---|---|---|---|---|
| **Enfoque Principal** | Observabilidad activa + ChatOps + AIOps | Monitoreo interactivo local en TUI | Gestión visual web de Docker | Monitoreo corporativo full-stack SaaS | Métricas pasivas y dashboards web |
| **Interfaz de Usuario** | TUI ligera (Deep Work) + Telegram | Solo TUI local en terminal | Web Dashboard pesado | Web Dashboard SaaS pesado | Web Dashboard |
| **Sobrecarga de CPU** | Ultrabaja (< 0.1% en daemon) | Media (renderiza solo en sesión abierta) | Media (~1-2% en background) | Media/Alta (~2-5% CPU) | Media (Node Exporter + Scrape) |
| **Control Remoto Seguro** | Reverse WebSocket + Whitelist (Cero RCE) | N/A (solo local) | Acceso web directo (vector de riesgo) | Agente propietario | N/A (solo lectura) |
| **AIOps / Model Context Protocol (MCP)** | Nativo (Claude Code, Cursor, OpenCode) | No | No | AI propietaria asistida | No nativo |
| **Prevención de Costos (Circuit Breaker)** | Nativo (Corte automático de red por Egress) | No | No | Solo alertas de billing post-facturación | Solo alertas |
| **Gestión de Secretos** | Keyring del SO + Bóveda AES-256-GCM (Zero-.env) | Variables / Config plano | Base de datos interna | API Keys en plano | Archivos de configuración |

---

## 3. Límites y Antipatrones (Fuera de Alcance)
- **NO es un panel web pesado**: No se construirán interfaces SPA complejas (React/Vue). La interacción humana vive en la TUI y la automatización en MCP/Telegram.
- **Cero RCE (D1)**: Prohibida la ejecución de comandos arbitrarios (`docker exec`). Las órdenes se limitan a la lista blanca: `restart`, `stop`, `isolate_network`.
- **Cero `.env` en el host (D2)**: El agente nunca dependerá de variables en texto plano en servidores de producción.
- **Cero Emojis en código/TUI**: Estética sobria, técnica, minimalista y profesional.

---

## 4. Fases del Proyecto

### Fase 1: Data Plane Core (Go Agent) [COMPLETADA]
- [x] Arquitectura Hexagonal (dominio y puertos).
- [x] Recolector Docker Socket con cálculo de deltas de CPU %, RAM real (sin `inactive_file`) y Egress.
- [x] Bóveda de credenciales en cascada (D-Bus Keyring -> AES-256-GCM + Argon2id en `~/.solv/vault.enc` -> Env vars).
- [x] Ring Buffer circular en memoria con descarte FIFO (máx 10 MB).
- [x] Cliente de transporte saliente con firma HMAC-SHA256.
- [x] CLI, Daemon desacoplado y TUI interactiva Bubbletea.

### Fase 2: Control Plane Core (FastAPI & Storage) [COMPLETADA]
- [x] Dependencia de validación HMAC-SHA256 y ventana de timestamp de 300s (protección anti-replay).
- [x] Modelos SQLAlchemy para `Host` y `ContainerMetric` (Time-Series).
- [x] Endpoints REST `POST /api/v1/telemetry/ingest`, `GET /hosts` y `GET /{host_id}/live`.
- [x] Herramientas del Servidor MCP (`get_infrastructure_overview`, `detect_anomalies_and_egress_spikes`).
- [x] Módulo Telegram ChatOps con botones firmados con HMAC y TTL de 60 segundos (D5).

### Fase 3: UX y Optimización de la TUI (Baja Carga Cognitiva) [COMPLETADA]
- [x] Semántica de glifos ASCII `[OK]`, `[--]`, `[||]`, `[!!]` con paleta Catppuccin Mocha.
- [x] Alerta predictiva de riesgo OOM al 85% de memoria Working Set.
- [x] Semáforo financiero de Egress alineado a la derecha (<500K, 5M, 50M).
- [x] Transición Master-Detail a pantalla completa para logs (`l` o `Enter`) con Breadcrumbs (`[<] Volver (Esc)`).
- [x] Búsqueda y filtrado reactivo en tiempo real con tecla `/`.
- [x] Blindaje ANSI con `lipgloss.Width()` y layouts colapsables en `View()`.

### Fase 4: Canal Reverso Saliente en Tiempo Real y Triaje AIOps [COMPLETADA]
- [x] Conexión saliente persistente iniciada por el agente Go hacia FastAPI (WebSocket con HMAC-SHA256).
- [x] Despacho inmediato de órdenes de control (`restart`, `stop`, `isolate_network`) al agente correspondiente.
- [x] Registro de auditoría de acciones ejecutadas en el host (`audit_logs`).
- [x] Triaje AIOps en tiempo real con OpenRouter para diagnóstico de incidentes en 2 líneas.

### Fase 5: Remediación en Caliente por Teclado en la TUI (Go Data Plane)
- [ ] Atajos de remediación directa sobre el contenedor seleccionado: `r` (restart), `s` (stop), `x` (isolate network).
- [ ] Modal interactivo de confirmación segura con Lip Gloss (`¿Confirmar acción en "container"? [y/N]`).
- [ ] Ejecución en el motor Docker a través de `ports.CollectorPort.ExecuteRemediation`.
- [ ] Notificación de estado en tiempo real en la barra de status (`[OK] Reiniciado en 850ms`).

### Fase 6: Diagnóstico Pasivo Zero-Prompt con OpenRouter en la TUI
- [ ] Detección desatendida de fallos y salidas anómalas (código != 0, OOMKilled 137, CrashLoopBackOff).
- [ ] Extracción y envío en segundo plano de logs de crash hacia OpenRouter.
- [ ] Banner contextual inferior en la TUI con diagnóstico en 1 línea y causa raíz sin necesidad de prompts manuales.

### Fase 7: Sparklines y Tendencias de CPU/RAM en Tiempo Real
- [ ] Historial circular en memoria de los últimos 30 deltas de CPU y Working Set RAM por contenedor.
- [ ] Renderizado de sparklines con caracteres ASCII (` ▂▃▅▆█`) en la tabla principal y en vista de detalle.

### Fase 8: Distribución, Script de Instalación y Release v1.0.0
- [ ] Script de onboarding de un solo comando (`curl -sSL https://... | sh`).
- [ ] Pipeline de CI para compilar binarios multi-arquitectura (`linux/amd64`, `linux/arm64`) en cada tag de versión.
- [ ] Documentación técnica de despliegue en producción.

