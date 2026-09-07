# Tareas: Canal Reverso WebSocket y Triaje AIOps con OpenRouter (04-reverse-websocket-and-aiops-triage)

## Fase 1: Endpoint WebSocket y Connection Manager en FastAPI
- [x] 1.1 Implementar `ConnectionManager` en `services/connection_manager.py` para mapear conexiones activas por `host_id`.
- [x] 1.2 Implementar router `routers/websocket_channel.py` con handshake autenticado HMAC.
- [x] 1.3 Implementar modelo `AuditLog` en `models/metrics.py`.
- [x] 1.4 Escribir tests unitarios en `tests/test_websocket_channel.py`.

## Fase 2: Cliente WebSocket Saliente en Go con Auto-Reconexión
- [x] 2.1 Escribir tests unitarios para mensajería y dispatch de comandos en `agent/internal/infrastructure/transport/ws_client_test.go`.
- [x] 2.2 Implementar cliente WebSocket en `agent/internal/infrastructure/transport/ws_client.go` con Gorilla WebSocket y Exponential Backoff.
- [x] 2.3 Integrar bucle de escucha de comandos en `agent/internal/delivery/cli/daemon.go`.

## Fase 3: Triaje AIOps con OpenRouter
- [x] 3.1 Implementar `services/ai_triage.py` con cliente HTTP asíncrono hacia `https://openrouter.ai/api/v1/chat/completions`.
- [x] 3.2 Integrar triaje en alertas de Telegram cuando se detecten fallos o peticiones MCP.
- [x] 3.3 Escribir tests con mock de OpenRouter en `tests/test_ai_triage.py`.

## Fase 4: Integración de Remediación en Caliente y Verificación E2E
- [x] 4.1 Conectar webhook de Telegram para despachar la orden validada hacia el WebSocket activo del host.
- [x] 4.2 Registrar auditoría de cada acción en `audit_logs`.
- [x] 4.3 Ejecutar batería completa de pruebas (Go y Python) en verde.

