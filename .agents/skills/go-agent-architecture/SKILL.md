---
name: go-agent-architecture
description: Estándares, patrones de diseño y guías para el agente host en Go (Bubbletea TUI, Docker Socket, Ring Buffer, Keyring fallback, HMAC).
license: MIT
metadata:
  author: Equipo SOLV
  version: "1.0"
---

## Estándares del Agente en Go

### 1. Arquitectura Hexagonal (Puertos y Adaptadores)
- `core/domain`: Entidades de métricas de contenedores, alertas y estado del host.
- `core/ports`: Interfaces `CollectorPort`, `StoragePort`, `OutboundChannelPort`, `KeyringPort`.
- `infrastructure/docker`: Adaptador para Docker Engine SDK / socket Unix `/var/run/docker.sock`.
- `infrastructure/keyring`: Adaptador para Keyring del SO con fallback a bóveda cifrada AES-GCM.
- `infrastructure/transport`: Cliente HTTP/WebSocket con firmador HMAC-SHA256.
- `delivery/tui`: Modelo-Vista-Actualización con Bubbletea y Lip Gloss.
- `delivery/cli`: Asistente interactivo de onboarding y ejecución en modo daemon.

### 2. Separación entre TUI y Daemon
- El modo daemon debe operar con un consumo de CPU inferior al 0.1%.
- La TUI se conecta vía canal local/IPC y limita el renderizado a 2 FPS o actualización por eventos.
- Paleta visual: Catppuccin Mocha con alto contraste.

### 3. Cálculo de Estadísticas de Docker
- **CPU %**: `(cpu_delta / system_cpu_delta) * number_of_cpus * 100.0`. Si el delta es $\le 0$, se descarta la muestra.
- **RAM**: `memory_stats.usage - memory_stats.stats.inactive_file`.
- **Egress de Red**: Delta de `networks[*].tx_bytes` por intervalo de muestreo.

### 4. Formato de Payload de Telemetría
```json
{
  "host_id": "string",
  "timestamp": 1741340000,
  "containers": [
    {
      "id": "abc12345",
      "name": "postgres-prod",
      "cpu_percent": 12.4,
      "ram_bytes": 104857600,
      "egress_bytes_sec": 4096,
      "status": "running"
    }
  ]
}
```
Cabecera requerida: `X-Solv-Signature: hmac_sha256(payload, shared_secret)`
