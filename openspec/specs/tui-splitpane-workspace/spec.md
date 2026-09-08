# Especificación: TUI Split-Pane Workspace

## 1. Comportamiento del Layout Split-Pane
- La TUI debe presentar una división horizontal en dos paneles cuando el ancho de terminal sea $\ge 80$ columnas:
  - **Panel de Contenedores (Izquierdo):** Ancho dedicado de 36 a 40 columnas con cabecera estándar `CONTENEDORES (N)`. Lista compacta de contenedores sin wrap de texto, con indicador de foco, glifo de estado, icono tipográfico Nerd Font, nombre y porcentaje de CPU.
  - **Paginación Vertical:** La lista debe implementar ventana deslizante según la altura de la terminal para evitar scroll forzado del buffer y asegurar que los bordes superiores de los paneles nunca se corten.
  - **Panel de Detalle (Derecho):** Ocupa el espacio restante horizontalmente. Renderiza de forma instantánea la ficha técnica del contenedor en foco sin exigir presionar `Enter`.
- Si el ancho de terminal es $< 80$ columnas, el layout debe colapsar ordenadamente a una vista tabular simple de una sola columna para preservar legibilidad.

## 2. Salto a Shell Local
- El atajo de teclado `e` debe suspender el ciclo de eventos de Bubbletea y transferir el control de la terminal al proceso nativo `docker exec -it <id> /bin/sh` o `/bin/bash`.
- Tras la terminación del proceso hijo (`exit`), Bubbletea debe retomar el control de la terminal, limpiar la pantalla (`AltScreen`) y redibujar el estado actual sin requerir reinicio del agente.
- Esta funcionalidad solo está disponible localmente y queda estrictamente prohibida su invocación a través del canal de control remoto.

## 3. Configuración de IA Estilo OpenCode y Tracking de Costos (AIOps Observability)
- **Selector de Modelos Minimalista (`c`)**:
  - La pulsación de `c` despliega un modal centrado de baja carga cognitiva con buscador interactivo en vivo (`Search`).
  - Lista filtrada de modelos organizada en secciones: `Active` (modelo y proveedor en uso), `Configured` (modelos con credencial lista en bóveda), `Other Providers` y `+ Custom Model ID...`.
  - Cada fila exhibe el nombre legible del modelo y a la derecha el nombre del proveedor junto a la máscara de clave (`••••1234` o `[Sin Clave]`).
  - Atajos rápidos: `Enter` para seleccionar modelo, `Ctrl+A` para abrir el panel de conexión/rotación de clave del proveedor y `Esc` para salir.
- **Conexión y Rotación de Credenciales (`Ctrl+A`)**:
  - Panel secundario para ingresar la API Key con input seguro (`EchoPassword`) y la URL base opcional del endpoint (para proxies o instancias locales de Ollama).
  - Almacenamiento directo y exclusivo en la bóveda del agente bajo Blindaje D2 (cifrado AES-256-GCM + Argon2id en `~/.solv/vault.enc`, permisos `0600`).
- **Soporte de Modelos Personalizados**:
  - Opción interactiva `+ Custom Model ID...` para ingresar cualquier identificador de modelo (ej. `deepseek/deepseek-r1`, `anthropic/claude-3-7-sonnet`, `qwen/qwen-2.5-coder-32b-instruct`).
- **Observabilidad de Tokens y Costos en USD**:
  - Cada inferencia de triaje (`Zero-Prompt Triage`) captura el uso de tokens (`PromptTokens`, `CompletionTokens`, `TotalTokens`).
  - Cálculo automático del costo estimado en USD según el proveedor/modelo (OpenRouter directo, Anthropic/OpenAI por tarifas de catálogo, Ollama `$0.00`).
  - Visualización del consumo en la ficha de diagnóstico del contenedor y contador acumulado de sesión en la barra de estado de la TUI.

## 4. Modales Flotantes (Overlay) y Navegación Horizontal
- Los modales de confirmación (`r`, `s`, `x`) y configuración (`c`) se renderizan superpuestos (*overlay*) sobre el Split-Pane sin sustituir ni borrar la vista base de la flota de fondo.
- El modal se calcula y ubica centrado tanto vertical como horizontalmente.
- Los botones de acción se presentan en una fila horizontal (`[ Confirmar ]` a la izquierda y `[ Cancelar ]` a la derecha) en cajas con bordes redondeados tipo tarjeta.
- La navegación de foco entre botones responde congruentemente a las teclas `←` (izquierda), `→` (derecha), `h`, `l` y `Tab`, además de accesos directos por tecla (`y`, `n`, `Esc`).
- El nombre del contenedor e ID se muestran en una sola línea horizontal con truncado inteligente si excede el ancho interior para evitar saltos de línea desalineados.

