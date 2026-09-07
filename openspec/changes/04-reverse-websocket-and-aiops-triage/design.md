# Diseño Técnico: Canal Reverso WebSocket y Triaje AIOps con OpenRouter (04-reverse-websocket-and-aiops-triage)

## Arquitectura de Componentes y Flujo Bidireccional

```
[ Telegram / MCP / IA ]
        │
        ▼ (Callback / Tool Request)
┌───────────────────────────────────────────────────┐
│ FastAPI Control Plane                             │
│  ├── ConnectionManager (Mapeo host_id -> WebSocket)│
│  ├── services/ai_triage.py (OpenRouter LLM)       │
│  ├── models/metrics.py (AuditLog table)           │
│  └── routers/websocket_channel.py                 │
└────────────────────────┬──────────────────────────┘
                         │ (WebSocket saliente TLS)
                         │ {"action": "restart", "container_id": "c1"}
                         ▼
┌───────────────────────────────────────────────────┐
│ Go Data Plane Host Agent                          │
│  ├── transport/ws_client.go (Gorilla WebSocket)   │
│  ├── core/ports (ExecuteRemediation)              │
│  └── infrastructure/docker (Docker SDK Engine)    │
└───────────────────────────────────────────────────┘
```

## Protocolo de Mensajería JSON

### 1. Comando Enviado por el Servidor (Control Plane $\rightarrow$ Data Plane)
```json
{
  "id": "cmd-8f3a-4b91",
  "action": "restart",
  "container_id": "7f8b9a1c2d3e",
  "timestamp": 1741340000
}
```

### 2. Confirmación Enviada por el Agente (Data Plane $\rightarrow$ Control Plane)
```json
{
  "id": "cmd-8f3a-4b91",
  "success": true,
  "message": "Contenedor reiniciado exitosamente en 1.2s",
  "error": null,
  "timestamp": 1741340002
}
```
