# Tareas: TUI Split-Pane Workspace y Espacio de Trabajo (08-tui-splitpane-workspace)

## Fase 1: Motor de Logos Tipográficos y Detección de Imágenes
- [x] 1.1 Crear catálogo tipográfico con iconos Nerd Font y badges en `agent/internal/delivery/tui/ascii_logos.go`.
- [x] 1.2 Implementar función clasificadora de tecnologías a partir del nombre de la imagen Docker.
- [x] 1.3 Crear tests unitarios en `agent/internal/delivery/tui/ascii_logos_test.go`.

## Fase 2: Layout Split-Pane Reactivo en Bubbletea
- [x] 2.1 Refactorizar `viewTable()` para soportar el modo Split-Pane (panel izquierdo de flota + panel derecho de ficha técnica viva).
- [x] 2.2 Conectar el desplazamiento de cursor (`j`/`k`, flechas, mouse) para actualizar la ficha técnica del panel derecho de forma reactiva instantánea.
- [x] 2.3 Integrar tarjetas de métricas cuantitativas, medidores térmicos y acciones en el panel derecho.

## Fase 3: Salto Interactivo a Shell (`Attach Shell` vía `e`)
- [x] 3.1 Implementar comando `tea.ExecProcess` para invocar `/bin/sh` dentro del contenedor activo.
- [x] 3.2 Manejar la restauración del estado de la terminal al retornar del shell con `tea.ClearScreen` sin corrupción gráfica.
- [x] 3.3 Validar que respete el blindaje de seguridad D1 (cero RCE por red, ejecución 100% local en TTY).

## Fase 4: Modal In-TUI para Configuración Segura de API Key (`c`)
- [x] 4.1 Implementar modal flotante con `textinput` con máscara de contraseña para ingresar la API Key de OpenRouter.
- [x] 4.2 Conectar el guardado a la bóveda cifrada local (`vault.enc` con AES-256-GCM y fallback Keyring).
- [x] 4.3 Actualizar el cliente de inferencia AIOps en tiempo real sin requerir reinicio del proceso.

## Fase 5: Verificación, Cobertura y Roadmap
- [x] 5.1 Ejecutar conjunto completo de tests con `-race` y validar que todos pasen en verde.
- [x] 5.2 Compilar binario final de `solv-agent`.
- [x] 5.3 Actualizar `docs/ROADMAP.md` marcando el progreso.
