---
name: fastapi-backend-architecture
description: Estándares, patrones de diseño y guías de seguridad para el Control Plane en FastAPI, TimescaleDB, Servidor MCP y ChatOps en Telegram.
license: MIT
metadata:
  author: Equipo SOLV
  version: "1.0"
---

## Estándares del Backend en FastAPI

### 1. Ingestión y Middleware de Seguridad
- Cada petición de ingestión de telemetría debe validar la cabecera `X-Solv-Signature` con HMAC-SHA256 contra el secreto del agente registrado.
- Timestamps con desfase mayor a 300 segundos son rechazados para evitar ataques de replay.

### 2. Almacenamiento de Series Temporales
- PostgreSQL con hypertables de TimescaleDB para `container_metrics` (particionado por tiempo).
- Política de retención: Datos crudos 14 días, agregados horarios (rollups) 90 días.

### 3. Servidor MCP (Model Context Protocol)
- FastAPI expone herramientas nativas SSE / stdio para orquestadores LLM:
  - `get_container_metrics(host_id, container_name, window)`
  - `get_financial_egress_alerts(host_id)`
  - `diagnose_container_failure(container_id, logs_tail)`
  - `request_remediation_action(host_id, container_id, action)`

### 4. ChatOps en Telegram
- El webhook de recepción valida el token del bot y la lista blanca de User IDs autorizados.
- Los botones interactivos contienen un payload firmado con HMAC y expiración de 60 segundos.
- Las órdenes de remediación se transmiten al agente Go correspondiente mediante el canal reverso activo de WebSocket.
