# Tareas: Agente Data Plane en Go (01-go-data-plane-agent)

## Fase 1: Dominio y Puertos del Núcleo
- [x] 1.1 Inicializar módulo `go.mod` en `agent/` con dependencias requeridas.
- [x] 1.2 Implementar entidades de dominio en `agent/internal/core/domain/` (`ContainerMetric`, `HostTelemetry`, `RemediationAction`).
- [x] 1.3 Definir interfaces en `agent/internal/core/ports/` (`CollectorPort`, `VaultPort`, `BufferPort`, `TransportPort`).

## Fase 2: Buffer Circular (TDD)
- [x] 2.1 Escribir tests unitarios para `RingBuffer` (encolado normal, desborde FIFO, vaciado ordenado).
- [x] 2.2 Implementar `RingBuffer` en `agent/internal/infrastructure/buffer/ring.go`.
- [x] 2.3 Verificar que todos los tests del Ring Buffer pasen.

## Fase 3: Bóveda de Credenciales y Fallback (TDD)
- [x] 3.1 Escribir tests unitarios para derivación Argon2id y cifrado/descifrado AES-256-GCM.
- [x] 3.2 Implementar adaptador de bóveda en `agent/internal/infrastructure/vault/vault.go` con permisos `0600`.
- [x] 3.3 Verificar que todos los tests de la bóveda pasen.

## Fase 4: Recolector Docker Socket y Matemáticas de Métricas (TDD)
- [x] 4.1 Escribir tests unitarios con datos simulados para cálculo de CPU delta, RAM neta y Egress.
- [x] 4.2 Implementar recolector en `agent/internal/infrastructure/docker/collector.go` usando socket Unix `/var/run/docker.sock`.
- [x] 4.3 Verificar que todos los tests del recolector pasen.

## Fase 5: Transporte Saliente y Firmador HMAC (TDD)
- [x] 5.1 Escribir tests unitarios para firma HMAC-SHA256 y verificación de payloads.
- [x] 5.2 Implementar cliente de transporte en `agent/internal/infrastructure/transport/client.go`.
- [x] 5.3 Verificar que todos los tests de transporte pasen.

## Fase 6: Delivery (CLI, Daemon y TUI)
- [x] 6.1 Implementar punto de entrada en `agent/cmd/solv-agent/main.go` con subcomandos `daemon` y `tui`.
- [x] 6.2 Implementar vista Bubbletea en `agent/internal/delivery/tui/` con paleta Catppuccin Mocha y límite a 2 FPS.
- [x] 6.3 Ejecutar conjunto de tests de integración para validar el flujo completo.
