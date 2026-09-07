# Diseño Técnico: Control Plane en FastAPI (02-fastapi-control-plane)

## Arquitectura de Componentes

```
server_tracker/
├── auth/
│   └── hmac_auth.py             # Verificación de firma X-Solv-Signature y timestamp
├── models/
│   ├── base.py                  # Base declarativa SQLAlchemy
│   ├── host.py                  # Entidad Host / Agente
│   └── metric.py                # Entidad ContainerMetric (Time-Series)
├── schemas/
│   └── telemetry.py             # Esquemas Pydantic v2 (IngestPayload, HostSummary)
├── routers/
│   ├── telemetry.py             # POST /ingest, GET /hosts, GET /{host_id}/live
│   └── telegram_webhook.py      # Receptor de webhooks y validación de tokens efímeros
├── notifications/
│   ├── base.py                  # Interfaz abstracta de notificación
│   ├── discord.py               # Estrategia Discord
│   └── telegram.py              # Estrategia Telegram con botones HMAC efímeros
├── mcp_server.py                # Servidor MCP con herramientas de diagnóstico AIOps
├── database.py                  # Conexión asíncrona/síncrona con PostgreSQL/TimescaleDB
├── main.py                      # Punto de entrada FastAPI montando routers y MCP
└── tests/
    ├── conftest.py              # Fixtures de Pytest con base de datos en memoria SQLite/Postgres
    ├── test_hmac_auth.py        # Pruebas unitarias de seguridad criptográfica
    ├── test_telemetry_ingest.py # Pruebas de endpoints de ingestión y consulta
    └── test_telegram_guard.py   # Pruebas de tokens efímeros de ChatOps
```

## Decisiones de Arquitectura (ADRs)

### ADR-04: Verificación HMAC en Middleware/Dependencia FastAPI
- **Contexto**: Las peticiones de ingestión contienen métricas sensibles y no deben usar cookies ni sesiones.
- **Decisión**: Se implementa como una dependencia `Depends(verify_hmac_signature)` que lee el raw body (`bytes`), calcula HMAC-SHA256 con el `shared_secret` del host y valida la cabecera `X-Solv-Signature`.

### ADR-05: Tokens de Callback en Telegram con TTL de 60s
- **Contexto**: Un botón de Telegram en un chat grupal o reenviado puede ser presionado por cualquier usuario en cualquier momento.
- **Decisión**: El `callback_data` contiene `action:host:container:timestamp:hmac_signature`. El webhook rechaza peticiones si `now - timestamp > 60s` o si el HMAC es inválido.
