# Especificación: TUI Split-Pane Workspace

## 1. Comportamiento del Layout Split-Pane
- La TUI debe presentar una división horizontal en dos paneles cuando el ancho de terminal sea $\ge 80$ columnas:
  - **Panel de Flota (Izquierdo):** Ocupa entre el 25% y 30% del ancho. Lista compacta de contenedores con foco navegable mediante cursor.
  - **Panel de Detalle (Derecho):** Ocupa el espacio restante (70-75%). Renderiza de forma instantánea la ficha del contenedor en foco sin exigir presionar `Enter`.
- Si el ancho de terminal es $< 80$ columnas, el layout debe colapsar ordenadamente a una vista tabular simple de una sola columna para preservar legibilidad.

## 2. Salto a Shell Local
- El atajo de teclado `e` debe suspender el ciclo de eventos de Bubbletea y transferir el control de la terminal al proceso nativo `docker exec -it <id> /bin/sh` o `/bin/bash`.
- Tras la terminación del proceso hijo (`exit`), Bubbletea debe retomar el control de la terminal, limpiar la pantalla (`AltScreen`) y redibujar el estado actual sin requerir reinicio del agente.
- Esta funcionalidad solo está disponible localmente y queda estrictamente prohibida su invocación a través del canal de control remoto.

## 3. Configuración Segura en Caliente
- La pulsación de `c` debe abrir un modal de configuración de la clave de IA.
- El texto ingresado no debe mostrarse en claro en la pantalla ni en logs.
- La clave debe guardarse directamente en la bóveda del agente usando cifrado AES-256-GCM.
