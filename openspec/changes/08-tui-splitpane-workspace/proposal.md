# Propuesta: TUI Split-Pane Workspace e Interactividad en Caliente (08-tui-splitpane-workspace)

## 1. Declaración del Problema
La interfaz actual de SOLV Server Tracker presenta una vista tabular clásica Master que requiere transiciones modales (`Enter` y `Esc`) para examinar registros o inspeccionar métricas detalladas. Si bien es funcional, impone una fricción cognitiva al operar incidentes en caliente:
1. **Navegación secuencial bloqueante:** No es posible observar los registros y la ficha técnica de un contenedor mientras se recorre la lista de procesos con las flechas del teclado.
2. **Configuración de IA externa y tosca:** Activar el triaje pasivo exige exportar variables de entorno en la shell del sistema operativo (`export OPENROUTER_API_KEY=...`), alejándose de la experiencia autocontenida de herramientas modernas como OpenCode o LazyGit.
3. **Observabilidad puramente pasiva:** El operador debe abandonar la TUI y abrir otra terminal para ejecutar comandos frecuentes como abrir una shell interactiva (`docker exec -it`), examinar procesos internos (`top`) o auditar mapeos de puertos y volúmenes.

## 2. Visión y Propuesta de Solución
Evolucionar la TUI de un monitor pasivo hacia un **Espacio de Trabajo Operativo (Operator Workstation)**:

- **Layout Split-Pane Reactivo (Master-Detail simultáneo):**
  - **Panel Izquierdo (30% ancho):** Lista compacta de contenedores con glifo de estado (`[OK]`, `[--]`, `[!!]`), nombre del contenedor y medidor térmico rápido.
  - **Panel Derecho (70% ancho):** Ficha técnica viva que se actualiza instantáneamente al desplazar el cursor con `j`/`k` o flechas (sin necesidad de presionar `Enter`).
- **Logotipos ASCII de Tecnología:** Detección de la imagen base (`postgres`, `redis`, `traefik`, `nginx`, `golang`, `python`, `node`, `docker`) y renderizado de arte ASCII estilizado en el panel superior derecho.
- **Acceso Directo a Shell (`Attach Shell` vía tecla `e`):** Suspensión temporal de la TUI mediante `tea.ExecProcess` para abrir una terminal interactiva nativa dentro del contenedor seleccionado (`/bin/bash` o `/bin/sh`), retornando a la TUI de inmediato al salir.
- **Modal de Configuración In-TUI (tecla `c`):** Entrada interactiva de la clave de OpenRouter con máscara de contraseña, persistida directamente en la bóveda local cifrada con AES-256-GCM (`~/.solv/vault.enc`).
- **Inspección Rápida de Recursos (Runbook de 1 tecla):**
  - `p`: Lista de procesos internos del contenedor (`docker top`).
  - `n`: Mapeo de redes, IPs internas y puertos expuestos (con alerta de seguridad en `0.0.0.0`).
  - `v`: Puntos de montaje y volúmenes asociados.

## 3. Criterios de Éxito
- La navegación por la lista izquierda actualiza el panel derecho a 60 FPS sin latencia perceptible.
- Presionar `e` entrega una TTY interactiva local en menos de 300 ms sin romper el buffer de terminal al retornar.
- La clave de OpenRouter ingresada en la TUI persiste entre reinicios en la bóveda cifrada sin archivos `.env` en texto plano.
