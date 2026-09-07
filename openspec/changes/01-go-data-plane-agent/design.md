# Diseño Técnico: Agente Data Plane en Go

## Arquitectura del Componente

Arquitectura Hexagonal desacoplada bajo el directorio `agent/`:

```
agent/
├── cmd/
│   └── solv-agent/
│       └── main.go                  # Punto de entrada CLI/Daemon/TUI
├── internal/
│   ├── core/
│   │   ├── domain/                  # Entidades puras (Metric, Container, Host, TelemetryBatch)
│   │   └── ports/                   # Interfaces (Collector, Vault, Buffer, Transport)
│   ├── infrastructure/
│   │   ├── docker/                  # Adaptador Docker Socket (/var/run/docker.sock)
│   │   ├── vault/                   # Adaptador Keyring con fallback a AES-256-GCM
│   │   ├── buffer/                  # Implementación Ring Buffer FIFO en memoria
│   │   └── transport/               # Cliente HTTP/WebSocket con firmador HMAC-SHA256
│   └── delivery/
│       ├── cli/                     # Comandos Cobra/Flag para onboarding y daemon
│       └── tui/                     # Interfaz Bubbletea + Lip Gloss (Catppuccin Mocha)
├── go.mod
└── go.sum
```

## Decisiones de Arquitectura (ADRs)

### ADR-01: Ring Buffer con descarte FIFO para contrapresión
- **Contexto**: Si el backend FastAPI se cae, el agente no puede consumir memoria indefinidamente.
- **Decisión**: Un `RingBuffer` con capacidad configurable (default 1000 lotes / máx 10 MB) que descarta los lotes más viejos cuando se llena.

### ADR-02: Bóveda cifrada local con Argon2id + AES-256-GCM
- **Contexto**: Servidores headless no tienen D-Bus activo para Keyring del SO.
- **Decisión**: Si `zalando/go-keyring` falla, se guarda la configuración cifrada en `~/.solv/vault.enc` con clave derivada por Argon2id (tiempo 1, memoria 64MB, paralelismo 4) y permisos `0600`.

### ADR-03: Cálculo de CPU con salvaguarda de deltas
- **Contexto**: La API de Docker devuelve acumuladores de tiempo de CPU. Reinicios de contenedor producen deltas inválidos.
- **Decisión**: Si `system_cpu_delta <= 0` o `cpu_delta < 0`, la función de cálculo retorna `0.0` y no propaga errores ni distorsiones.

## Contrato de Datos (Payload JSON)

```json
{
  "host_id": "srv-prod-01",
  "timestamp": 1741340000,
  "containers": [
    {
      "id": "7f8b9a1c2d3e",
      "name": "api-gateway",
      "image": "nginx:1.27-alpine",
      "status": "running",
      "cpu_percent": 3.45,
      "ram_bytes": 45088768,
      "ram_limit_bytes": 536870912,
      "egress_bytes_sec": 12800,
      "ingress_bytes_sec": 45000,
      "pids": 4
    }
  ]
}
```
Cabecera HTTP: `X-Solv-Signature: <hex_hmac_sha256>`
