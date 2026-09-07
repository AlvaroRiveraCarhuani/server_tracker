# Propuesta: Agente Host Data Plane en Go (Telemetría, Zero-YAML y TUI)

## Intención

Implementar el agente host en Go (Data Plane), ligero y con política estricta de cero RCE, para SOLV Server Tracker. El agente se encarga de extraer métricas precisas de Docker directamente desde el socket del host (`/var/run/docker.sock`), gestionar credenciales sin archivos `.env` mediante una cadena de fallback en el Keyring del SO, firmar la telemetría con HMAC-SHA256, almacenar métricas en un buffer circular durante caídas de red y ofrecer una TUI en terminal para trabajo profundo.

## Alcance

### Dentro del Alcance
- **Núcleo y Arquitectura Hexagonal**: Modelos de dominio para `ContainerMetrics`, `HostTelemetry` y evaluación de límites de red.
- **Recolector desde Docker Socket**: Extracción directa de estadísticas (`/var/run/docker.sock`) calculando delta de CPU %, RAM real (descontando `inactive_file`) y bytes de salida por segundo (Egress).
- **Cascada de Almacenamiento de Credenciales**: 
  1. Keyring del SO vía SecretService / D-Bus.
  2. Bóveda local cifrada con AES-256-GCM (`~/.solv/vault.enc`, permisos `0600`) derivada con Argon2id para servidores headless.
  3. Variables de entorno para entornos CI/CD.
- **Buffer Circular Deslizante (Ring Buffer)**: Buffer FIFO en memoria (máximo 10 MB) para tolerar desconexiones de red sin fugas de memoria.
- **Firmador Zero-Trust HMAC**: Firma criptográfica HMAC-SHA256 en cada payload JSON con cabecera de timestamp.
- **Modos de Ejecución Duales**:
  - Modo Daemon: Bucle en segundo plano con consumo mínimo (<0.1% CPU).
  - Modo TUI: Panel interactivo en Bubbletea / Lip Gloss con paleta Catppuccin Mocha y límite de 2 FPS.
- **Cliente Reverso Saliente**: Conexión saliente vía WebSocket/HTTP con TLS hacia el Control Plane (cero puertos abiertos en el host).

### Fuera del Alcance
- Migraciones de base de datos en el Control Plane (se manejan en el cambio del backend).
- Webhooks de Telegram y triaje con LLM (se manejan en el cambio del backend).
- Ejecución de comandos arbitrarios (`exec`) dentro de contenedores (estrictamente prohibido por la regla D1).

## Capacidades

### Nuevas Capacidades
- `docker-telemetry-collector`: Extracción y cálculo matemático de deltas de CPU, RAM y red desde el socket de Docker.
- `keyring-fallback-vault`: Persistencia segura de credenciales con Keyring del SO y fallback local cifrado.
- `telemetry-ring-buffer`: Buffer circular acotado en memoria para control de contrapresión y resiliencia sin red.
- `hmac-transport-client`: Cliente de transporte reverso saliente con firma HMAC-SHA256 de payloads.
- `agent-tui`: Interfaz interactiva de terminal para inspección de contenedores.

### Capacidades Modificadas
- Ninguna (implementación inicial del núcleo).

## Enfoque Técnico

1. **Arquitectura**: Implementar Arquitectura Hexagonal estricta bajo `agent/`:
   - `agent/cmd/solv-agent/main.go`
   - `agent/internal/core/domain`
   - `agent/internal/core/ports`
   - `agent/internal/infrastructure/docker`
   - `agent/internal/infrastructure/vault`
   - `agent/internal/infrastructure/transport`
   - `agent/internal/delivery/tui`
   - `agent/internal/delivery/cli`
2. **Estrategia de Pruebas**:
   - Tests unitarios con pruebas basadas en tablas para el cálculo de deltas, desborde FIFO del Ring Buffer y verificación HMAC.
   - Cliente Docker simulado (Mock) para pruebas de flujo de métricas.
3. **Seguridad y Blindajes**:
   - Descartar deltas de CPU del sistema cuando sean menores o iguales a cero.
   - Acceso de solo lectura al socket cuando sea posible; restringir acciones remotas a `restart`, `stop` e `isolate_network`.

## Plan de Rollback

El agente en Go es un binario autocontenido que corre en espacio de usuario/daemon en el host. No altera esquemas de base de datos ni configuraciones globales de Docker. El rollback consiste en detener el proceso y eliminar `~/.solv/vault.enc` si fue creado.
