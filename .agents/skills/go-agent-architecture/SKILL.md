---
name: go-agent-architecture
description: Estándares, patrones de diseño y guías para el agente host en Go (Bubbletea TUI, Docker Socket, Ring Buffer, Keyring fallback, HMAC, UX de baja carga cognitiva).
license: MIT
metadata:
  author: Equipo SOLV
  version: "1.1"
---

## Estándares del Agente en Go

### 1. Arquitectura Hexagonal (Puertos y Adaptadores)
- `core/domain`: Entidades de métricas de contenedores, alertas y estado del host.
- `core/ports`: Interfaces `CollectorPort`, `VaultPort`, `BufferPort`, `TransportPort`.
- `infrastructure/docker`: Adaptador para Docker Engine SDK / socket Unix `/var/run/docker.sock`.
- `infrastructure/vault`: Adaptador para Keyring del SO con fallback a bóveda cifrada AES-256-GCM + Argon2id.
- `infrastructure/buffer`: Ring Buffer circular FIFO en memoria con límite fijo (máx 10 MB).
- `infrastructure/transport`: Cliente HTTP/WebSocket con firmador HMAC-SHA256.
- `delivery/tui`: Modelo-Vista-Actualización con Bubbletea y Lip Gloss (Catppuccin Mocha).
- `delivery/cli`: Asistente interactivo de onboarding y ejecución en modo daemon.

### 2. Principios de UX en la TUI (Baja Carga Cognitiva y Cero Emojis)
- **Regla del Primer Segundo**: El estado del clúster debe entenderse en < 1 segundo de contacto visual.
- **Cero Emojis**: Usar glifos ASCII sobrios y consistentes:
  - `[OK]` (Verde `#a6e3a1`): Saludable.
  - `[--]` (Gris `#6c7086`): Detenido intencionalmente (atenuado en el fondo).
  - `[||]` (Amarillo `#f9e2af`): Pausado / transitorio.
  - `[!!]` (Rojo `#f38ba8` + Bold): Caída crítica o riesgo OOM inminente.
- **Alerta Predictiva de OOM al 85%**: Si el Working Set de RAM (`memory.current - inactive_file`) supera el 85% del límite, se activa `[!!] OOM_RISK` antes de la intervención del OOM Killer de Linux.
- **Semáforo de Egress Financiero**: Valores alineados a la derecha (`lipgloss.Position(lipgloss.Right)`):
  - `< 500 KB/s`: Subtext1 (`#bac2de`) atenuado.
  - `500 KB/s - 5 MB/s`: Text (`#cdd6f4`) estándar.
  - `5 MB/s - 50 MB/s`: Peach (`#fab387`) alerta cálida.
  - `> 50 MB/s`: Red (`#f38ba8`) + Bold (riesgo financiero / exfiltración).
- **Flujo Master-Detail a Pantalla Completa (Model Swapping)**:
  - Descartar cuadrantes múltiples pequeños que inducen fatiga visual.
  - Tecla `l` o `Enter` cambia al visor de logs a pantalla completa (`viewport.Model`) con Breadcrumb en el Header (`[<] Volver (Esc) | Logs: <nombre> | Estado: [OK]`).
  - Tecla `Esc` o `h` regresa a la flota sin perder la posición del cursor.
- **Filtrado Reactivo con `/`**: Input interactivo que colapsa la tabla en tiempo real mientras se tipea.

### 3. Blindaje ANSI y Rendimiento
- **Cálculo de Ancho de Celdas**: Usar exclusivamente `lipgloss.Width()` (nunca `len()`) para evitar desalineaciones provocadas por códigos de escape ANSI.
- **Truncado Seguro**: Truncar cadenas de texto *antes* de inyectarles estilos de color.
- **Consumo 0.0% CPU en Reposo**: Retorno condicional en `View()` y comandos asíncronos `tea.Cmd`.
