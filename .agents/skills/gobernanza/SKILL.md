---
name: gobernanza
description: Change management, SDD traceability, ADR records, and clean commit standards without AI attribution.
license: MIT
metadata:
  author: SOLV Team
  version: "1.0"
---

## Gobernanza y Control de Cambios

### 1. Convención de Commits
- Usar Conventional Commits estricto: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`.
- **PROHIBIDO** incluir atribuciones de IA ("Co-Authored-By", "AI-assisted", etc.).

### 2. Trazabilidad SDD (Spec-Driven Development)
- Todo cambio estructural o funcional se documenta primero en `openspec/` antes de escribir código.
- Registro de Decisiones de Arquitectura (ADRs) bajo `openspec/specs/` o `docs/ADR/`.
- Mantener tests automatizados para validar cada escenario de la especificación.
