# Documentación de deal-dev-kit

**En resumen:** `deal-dev-kit` es el repositorio único que contiene las convenciones,
skills de IA y componentes UI compartidos de CRM DEAL, más el CLI `deal` que los
instala y mantiene sincronizados en cada repositorio del proyecto. Esta carpeta
documenta qué contiene, cómo funciona por dentro y cómo presentarlo.

## Cómo leer esta documentación

| Si tiene... | Empiece por |
|---|---|
| 5 minutos | [`01-que-es.md`](01-que-es.md) — el problema que resuelve y los principios de diseño |
| que dar la demo mañana | [`07-guia-de-demo.md`](07-guia-de-demo.md) — mensaje central, guion y preguntas probables |
| que instalar el kit en un repo | [`03-cli.md`](03-cli.md) — instalación y comandos |
| que entender cómo decide qué sobrescribir | [`04-motor-de-sincronizacion.md`](04-motor-de-sincronizacion.md) |
| que cortar un release | [`05-versionado-y-distribucion.md`](05-versionado-y-distribucion.md) |
| que instalar o depurar Engram | [`06-engram.md`](06-engram.md) |
| que saber exactamente qué hay hoy | [`02-inventario.md`](02-inventario.md) — tabla completa de artefactos |

## Documentos

| Documento | Contenido |
|---|---|
| [`01-que-es.md`](01-que-es.md) | El problema, qué es el kit, principios de diseño y su razón |
| [`02-inventario.md`](02-inventario.md) | Todo lo que el kit tiene hoy: skills, commands, agents, config, ui-kit, perfiles por tipo |
| [`03-cli.md`](03-cli.md) | Instalación del CLI, tabla de comandos, flujos paso a paso |
| [`04-motor-de-sincronizacion.md`](04-motor-de-sincronizacion.md) | El plan, la clasificación de archivos, el lockfile, `ensure_line`/`ensure_json` |
| [`05-versionado-y-distribucion.md`](05-versionado-y-distribucion.md) | Los dos namespaces de tags, CI, releases, self-update |
| [`06-engram.md`](06-engram.md) | Memoria persistente del agente: instalación, estados, riesgos |
| [`07-guia-de-demo.md`](07-guia-de-demo.md) | Guion de demo, argumentos por público, preguntas probables, estado honesto |

## Diagramas

Los diagramas viven en `docs/diagrams/`: el `.excalidraw` es la fuente
editable, y el `.svg` es lo que muestran los documentos. Para cambiar uno, se
edita el `.excalidraw` y se vuelve a exportar el `.svg`.

| Archivo | Qué muestra |
|---|---|
| `diagrams/01-panorama.svg` | Vista general: repo del kit → GitHub → CLI `deal` → repos del proyecto → Claude Code |
| `diagrams/02-que-instala.svg` | Qué recibe cada tipo de proyecto (backend / web / mobile) |
| `diagrams/03-flujo-install.svg` | Secuencia de `deal init` / `deal install` |
| `diagrams/04-reglas-de-propiedad.svg` | Clasificación de archivos + `ensure_line` / `ensure_json` |
| `diagrams/05-versionado.svg` | `v*` vs `kit-v*`, CI, releases, self-update |
| `diagrams/06-engram.svg` | Instalación de Engram y sincronización de memorias |
| `diagrams/07-agente-en-accion.svg` | Qué se activa durante una sesión de Claude Code, cómo y en qué momento |

## Fuentes

Todo lo escrito acá está verificado contra el código y los documentos del
repositorio: `README.md`, `HANDOFF.md`, `kit.yaml`, `tool/cmd/deal-kit/`,
`tool/internal/**`, `tool/scripts/`, `.github/workflows/`,
`tool/.goreleaser.yaml`, `skills/**/SKILL.md`, `commands/**`, `agents/**`,
`config/persona.md` y `ui-kit/`. Donde `HANDOFF.md` marca algo como pendiente,
esta documentación lo reporta como pendiente — no se completan huecos por
conveniencia narrativa.
