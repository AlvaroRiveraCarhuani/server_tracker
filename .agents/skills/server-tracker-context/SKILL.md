---
name: server-tracker-context
description: Arquitectura general del sistema, contratos de datos, modelo de amenazas y decisiones de diseño inamovibles (D1-D5) de SOLV Server Tracker.
license: MIT
metadata:
  author: Equipo SOLV
  version: "1.0"
---

## Visión General

SOLV Server Tracker es una plataforma de Observabilidad Activa, ChatOps y AIOps para infraestructuras Docker On-Premise.

## Decisiones Inamovibles de Arquitectura (D1–D5)

### D1: Canal de Control Reverso (Cero RCE)
- El agente en Go inicia una conexión saliente hacia FastAPI mediante WebSocket/gRPC con TLS.
- El host nunca expone puertos escuchando hacia la red.
- Acciones permitidas: `restart`, `stop`, `isolate_network`.
- Acciones prohibidas: `exec`, ejecución de comandos de shell arbitrarios.

### D2: Cascada de Fallback de Credenciales
- Primario: Keyring nativo del SO (`SecretService` / D-Bus).
- Secundario (servidores headless/SSH): Archivo cifrado con AES-256-GCM (`~/.solv/vault.enc`, permisos `0600`) derivado con Argon2id.
- Terciario (CI/CD y tests): Variables de entorno (`SOLV_SERVER_URL`, `SOLV_AGENT_SECRET`).

### D3: Cortocircuito Financiero Declarativo
- Evalúa la tasa de tráfico de salida (Egress) por contenedor.
- Configurable mediante Docker Labels en cada contenedor:
  - `solv.egress.limit`: ej. `100MB`
  - `solv.egress.window`: ej. `5m`
  - `solv.egress.action`: `alert` (por defecto) | `isolate` | `stop`
- Contenedores sin etiquetas aplican `alert` preventivo (*dry-run*).

### D4: Cálculo de Métricas y Ring Buffer
- Las métricas de CPU y Red se calculan mediante deltas discretos entre dos lecturas temporales.
- Memoria: se descuenta el caché inactivo (`inactive_file`) de `usage` para reflejar el consumo real de RAM.
- Un Ring Buffer en memoria (máximo 10 MB) protege las métricas durante caídas de red aplicando política FIFO.

### D5: Seguridad en Telegram ChatOps
- Lista blanca de User IDs / Chat IDs verificada en middleware.
- Los botones interactivos integran un payload firmado con HMAC, timestamp y TTL de 60 segundos.
- Callbacks vencidos o alterados son rechazados de inmediato con registro en auditoría.
