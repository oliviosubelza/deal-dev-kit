# deal-dev-kit — estado y pendientes

Documento de traspaso. **Sí está versionado**: `git ls-files HANDOFF.md` lo lista y
no aparece en `.gitignore` (la versión anterior de esta línea afirmaba lo contrario).
Última actualización: 2026-09-04.

---

## 1. Qué es esto

Kit de desarrollo compartido para **CRM DEAL** (Grupo Venado). Un solo repo público
`github.com/oliviosubelza/deal-dev-kit` con dos cosas:

- **Contenido del kit**: skills para el agente de IA, y el catálogo de componentes UI.
- **`tool/`**: el CLI `deal-kit` en Go, que instala ese contenido en cada proyecto.

### Estructura

```
deal-dev-kit/
├── kit.yaml            # manifiesto: qué se puede instalar, dónde va, de qué depende
├── skills/             # 7 skills, agrupadas por tipo de proyecto
├── ui-kit/             # 61 artefactos: 57 primitivos shadcn + data-table + lib + hooks + theme
└── tool/               # el CLI (go.mod propio, extraíble con subtree split)
    ├── cmd/deal-kit/
    └── internal/{kit,plan,lockfile,paths,pm,doctor,selfupdate,cli,tui}/
```

### Dos namespaces de tags (importante)

| Tag | Versiona | Dispara |
|---|---|---|
| `v0.1.7` | el binario del CLI | GoReleaser, publica binarios |
| `kit-v0.2.0` | el contenido del kit | nada; es lo que los proyectos pinean |

El workflow `release-cli` filtra por `v*`, y `kit-v*` no matchea. Está así porque los
tags con prefijo tipo `cli/v1.0.0` requieren la config `monorepo` de **GoReleaser Pro**,
que es de pago.

### Orden de publicación: primero el CLI, después el kit

El schema de `kit.yaml` está en **`version: 2`** (subió al declarar los tipos `command` y
`agent`). El CLI acepta **1 y 2**, así que sigue leyendo los kits pineados en
`kit-v0.2.0` o anteriores.

**El tag `v*` del CLI se publica ANTES del tag `kit-v*`.** Un proyecto tiene que poder
correr `deal-kit self-update` antes de poder pinear contenido que su binario viejo no
sabe leer. Al revés, un binario anterior a este cambio falla con
`kit.yaml: unsupported version 2 (expected 1)` y no hay camino de salida automático.

### Mover un tag ya publicado rompe los cachés viejos

`Fetch` cachea un clon por repositorio, así que un tag que se mueve en el remoto choca
con el que el caché ya tiene. Hasta el arreglo del `--force`, `git fetch --tags` se negaba
a adoptarlo y **todo** comando fallaba con `exit status 1` sin explicación, incluido
`status` — no había salida dentro de la herramienta.

Ya está arreglado: el fetch usa `--force` y `--prune-tags`, y el caché adopta lo que diga
el remoto. Pero el arreglo vive en el binario, así que **una máquina con un CLI anterior
sigue rota** hasta que borre su caché a mano:

```
~/.cache/deal-kit/kits/<slug>-<hash>                    # Linux/macOS
%LocalAppData%\deal-kit\kits\<slug>-<hash>             # Windows
```

Pasó de verdad con `kit-v0.2.0`: el caché tenía `c4b1a57` y el remoto `03af755`.
Preferir un tag nuevo antes que mover uno publicado.

---

## 2. RESUELTO: `kit-v0.2.0` ya apunta a las 7 skills escritas

Esta sección decía que el tag estaba publicado con 6 skills en `TODO`. Verificado
directamente (`git rev-list -n 1 kit-v0.2.0`, `git ls-remote --tags origin`): **`kit-v0.2.0`
apunta al commit `03af755` — "feat(skills): write the seven skills from the architecture
briefing" — tanto local como en `origin`.** Ese commit escribe las 7 skills y elimina
`general/pr-workflow` (ver §5). No hace falta retagear.

---

## 3. Estado actual

### Publicado y funcionando

- Instaladores: `install.sh` (Linux/macOS/WSL) e `install.ps1` (Windows). Ambos
  verifican SHA-256. Probados de punta a punta.
- `deal-kit self-update` (desde v0.1.6).
- Fetch por git con cache en `~/.cache/deal-kit/kits/`.
- 61 componentes instalables con resolución transitiva y reescritura de imports.

### Comandos

```
(ninguno)     TUI interactiva
new <dir>     crea el proyecto con su generador oficial, después instala
init          detecta el tipo e instala el perfil
add <id>...   instala artefactos adicionales
update        mueve el pin del kit y re-sincroniza
status        qué hay instalado y si cambió
doctor        qué herramientas externas están instaladas
self-update   reemplaza el binario por el último release
```

Flags: `--repo --ref --offline --kit-dir --type --here --dry-run --yes --no-deps --check`

### Cobertura de tests

| Paquete | Cobertura |
|---|---|
| `paths` | 100% |
| `plan` | 91.7% |
| `kit` | 88.9% |
| `lockfile` | 88.9% |
| `doctor` | 88.2% |
| `tui` | 79.4% |
| `selfupdate` | 73.2% |
| `pm` | 70.3% |
| `cmd/deal-kit` | 43.2% |
| **`cli`** | **37.9%** ← sigue siendo la más baja |

---

## 4. Pendientes

### 4.1 Deuda de tests: `internal/cli` en 37.9%

Era 3.6%. Subió al arreglar los artefactos huérfanos del lockfile, que obligó a testear
`Status`, `Init`, `Add` y `Update` de punta a punta (`orphan_test.go`).

Sigue siendo el paquete más bajo, y **`new.go` y `selfupdate.go` siguen sin tests**. Es
justo el código que ejecuta comandos externos y reemplaza el propio binario, probado
solo a mano. Es lo que atacaría primero.

Lo que sí tiene tests: `ProjectRoot` (9 casos, en `cli_test.go`) y el ciclo completo de
huérfanos en `orphan_test.go`.

### 4.2 `deal-kit adopt`

Un proyecto que pierde el `deal-kit.lock`, o que ya tiene componentes copiados a mano
del `ui-kit` viejo, no tiene salida: el CLI ve archivos que no escribió y se niega a
tocarlos (correcto), pero no hay forma de decirle "estos son tuyos, registralos".

Debería: comparar el archivo local con el del kit, y si son idénticos, registrarlo en el
lockfile sin reescribir nada. Escenario garantizado cuando migren proyectos existentes.

**Resuelto aparte, no confundir**: el caso en que el lockfile referencia un artefacto que
`kit.yaml` ya no declara. Eso rompía `status` y `update` con `unknown artifact`, y ya está
arreglado: el artefacto se reporta `ORPHANED` y `update` borra sus archivos y su entrada,
salvo que hayan divergido. Lo de arriba sigue abierto y es distinto — ahí el lockfile no
existe o nunca registró esos archivos.

**Misma clase, todavía abierto**: un artefacto que `kit.yaml` **sí** declara, pero cuyo
`applies_to` dejó de cubrir el tipo del proyecto, aborta con `"%q does not apply to project
type %s"`. Sacar `web` del `applies_to` de un artefacto instalado reproduce el mismo callejón
sin salida. Quedó fuera de la política decidida para huérfanos.

### 4.3 Cómo se comparten los schemas Zod entre web y móvil

Las skills dicen "compartidos con móvil" porque la presentación lo dice, pero es
**polyrepo**: son dos repos distintos y la presentación nunca dice cómo se comparten
físicamente.

Hoy existe `/mnt/c/SoftwareDevelopment/shared-types/` con ~17 archivos `.ts`, **sin
`package.json` y sin git**. O sea: se copian a mano.

Es un candidato directo a artefacto del kit, y más crítico que los componentes: si web y
móvil divergen en el schema de un pedido, rompe en runtime, no en el editor.

### 4.4 `doctor` usa `ForWeb()` para los tres tipos

`internal/cli/new.go` llama `doctor.Check(doctor.ForWeb())` sin importar el tipo de
proyecto. Hoy no molesta (los tres necesitan Node), pero va a mentir cuando móvil pida
`eas` o backend pida `docker`. Falta `doctor.ForBackend()` y `doctor.ForMobile()`.

`claude` y `engram` se agregaron a `ForWeb()` **a propósito** (§10): son herramientas
del desarrollador, no del proyecto, así que aparecer en los tres tipos es el resultado
buscado, no una fuga. Ambas son opcionales (`Required` en su cero), así que nunca
bloquean.

### 4.5 Generador de módulos y features

Propuesto y pospuesto. La unidad que se crea decenas de veces no es `src/`, es un módulo:

```
deal-kit scaffold module orders     # backend hexagonal, 4 capas con archivos reales
deal-kit scaffold feature orders    # web: api/ components/ hooks/ store/ schemas/
```

**Decidido explícitamente que NO** se scaffoldean árboles de carpetas vacías: git no
versiona directorios vacíos, y un directorio sin archivos no comunica nada que la skill
de arquitectura no diga mejor.

### 4.6 `deal-kit lint`

Reglas puntuales y falsables, cada una nombrando su violación:

```
✗ src/components/ui existe — el estándar es src/shared/ui
```

**Decidido explícitamente que NO** se hace un chequeo por porcentaje ("cumple 72%"): no
hay denominador honesto, no es accionable, y entrena al equipo a ignorarlo.

### 4.7 Pantalla de "Update available" en la TUI

Estilo gentle-ai: caja con `2.3.0 → 2.5.0`, link a release notes, y menú
Update now / View changes / Keep current. Hoy el update vive dentro de "Estado del
proyecto". El usuario dijo "eso aún no".

### 4.8 Sin verificar

- `deal-kit new` en Windows (probado solo en WSL).
- `install.ps1` lo verificó el usuario, yo no pude (no hay PowerShell en la máquina).

---

## 5. Decisiones tomadas, con su razón

No relitigar sin motivo nuevo.

| Decisión | Razón |
|---|---|
| Un repo, no dos | El CLI en `tool/` con `go.mod` propio; extraíble con subtree split si el kit pasa a privado. |
| Go para el CLI | Cross-compila a binario estático: el equipo no instala runtime. Solo el autor necesita Go. |
| Copiar código fuente, no publicar paquete npm | GitHub Packages exige PAT por dev y por CI; npm privado se paga. Además copiar el fuente evita el problema de que Tailwind v4 no escanea `node_modules`. |
| No usar shadcn registry | Resuelve el diff contra ediciones locales, y se decidió que los componentes no se editan en el proyecto. |
| Bubble Tea para la TUI | Ya es convención del equipo (la skill `go-testing` cubre `teatest`). |
| La TUI es capa sobre `internal/plan` | Nunca un camino paralelo, o `--yes` y la TUI divergen. |
| Solo `y` aplica un plan | `enter` navega hacia la pantalla de plan; aceptarlo como confirmación permitió que dos Enter instalaran 74 archivos en Windows. |
| Tabla única de comandos | Dispatch y reconocimiento de nombres derivan de ella. Dos veces un comando nuevo cayó al browser por tener dos listas. |
| `--here` es opt-in | Sin él, un `cd` mal tipeado haría crecer 74 archivos donde no va. |
| No scaffoldear con wizard propio | Se corre el generador oficial no interactivo. Backend (NestJS) y móvil (Expo) no tienen uno, así que se imprime el comando en vez de medio-automatizar. |
| Skills y componentes en pantallas separadas | Mezclar una convención y un Button en una lista hace imposible saber qué se está instalando. |
| `ui-kit/base` requiere `web/ui` | Todo componente depende de `base`, así que instalar cualquier componente arrastra la skill del catálogo. |
| Solo lo que la presentación afirma | Sin inventar convenciones, sin marcar huecos. |
| `general/pr-workflow` eliminada | Todo lo que diría ya está en `general/conventions`. |
| `command` instala por nombre "leaf", no aplanado | El nombre de archivo de un command ES lo que un humano escribe (`/generate-schema`); un prefijo de grupo lo contradice y el equipo de collections ya documentó `/generate-schema` sin prefijo. `skill` y `agent` sí se quedan aplanados: nadie tipea el nombre de una skill (la carga el modelo por descripción) ni el de un agent (lo referencia el orquestador). Ver §9. |

---

## 6. Trampas descubiertas (no volver a pisarlas)

- **Los componentes importan `@/components/ui/...` pero se instalan en `src/shared/ui/`.**
  Sin reescritura, cada import queda roto. Resuelto con `import_rewrites` en `kit.yaml`,
  aplicado en `internal/plan/rewrite.go`. **Asume que el proyecto mapea `@/` → `src/`.**
- **lipgloss rellena con espacios cada línea de un `Render` multilínea.** Nunca poner
  `\n` dentro de `Render`.
- **`fmt` con `%-22s` cuenta los códigos ANSI como ancho.** Paddear con `lipgloss.Width`.
- **Las columnas de una fila deben sumar exactamente el ancho de contenido**, que es el
  ancho del panel **menos su padding**.
- **`create-vite` trata su argumento como relativo aunque sea absoluto.** Pasarle el
  directorio como lo tipeó el usuario.
- **Los goldens tienen que ser deterministas.** El path de `t.TempDir()` cambia entre
  corridas, y el header lo trunca, así que hay que fijar `ProjectRoot` **antes** de
  renderizar, no scrubbear después.
- **Go cachea resultados de test** aunque cambie un archivo fuera del paquete. Usar
  `-count=1` al validar `kit.yaml`.
- **El kit es un repo git**, así que chequear `.git` antes que `kit.yaml` lo aceptaba
  como proyecto.
- **Windows: el Enter que lanza el comando se filtra al proceso** (ConPTY entrega
  key-down y key-up).
- **PowerShell 5.1 negocia TLS 1.0 por defecto** y GitHub lo rechaza.
- **El skill original del ui-kit tenía 3 nombres de export mal**: `PortalContainer`,
  `Chart`, `Resizable` no existen. Verificar símbolos contra el código, no confiar.

---

## 7. Cómo trabajar en esto

```bash
export PATH=$PATH:/usr/local/go/bin     # Go 1.22 local; el toolchain 1.24.2 se baja solo
cd /mnt/c/SoftwareDevelopment/deal-dev-kit/tool

go build -o /tmp/deal-kit ./cmd/deal-kit
go test ./... -count=1
go test ./internal/tui/ -update          # regenerar goldens
gofmt -l . && go vet ./...
```

Proyecto de prueba: `/mnt/c/SoftwareDevelopment/deal-test/crm-deal-web`
(y `/mnt/c/SoftwareDevelopment/crm-deal-web`, que se usó para probar `--here`).

```bash
cd /mnt/c/SoftwareDevelopment/deal-test/crm-deal-web
/tmp/deal-kit --kit-dir /mnt/c/SoftwareDevelopment/deal-dev-kit    # kit local, no el de GitHub
```

**`--kit-dir` es obligatorio para probar cambios locales del kit.** Sin él baja el de
GitHub y no vas a ver tus cambios.

### Test que más valor tiene

`internal/kit/repo_manifest_test.go` parsea el `kit.yaml` **real** en cada corrida:
verifica que cada perfil resuelva, que cada `src` exista en disco, y que el `name` del
frontmatter de cada `SKILL.md` coincida con su nombre aplanado.

---

## 8. Contexto del CRM DEAL

De la presentación del coordinador técnico (agosto 2026,
`~/Downloads/CRM_DEAL_Presentacion_Equipo (1).pptx`, 22 slides). **Etapa de diseño:
ninguno de los tres repos existe todavía.**

Polyrepo, tres tipos:

| Tipo | Repo | Stack |
|---|---|---|
| `backend` | `crm-deal-*-service` | NestJS, hexagonal, 4 capas |
| `web` | `crm-deal-web` | React + Vite, feature-based |
| `mobile` | `crm-deal-mobile` | React Native + Expo, React Native Paper |

- El design system web vive en **`src/shared/ui/`** (no `src/components/ui`).
- Móvil usa **React Native Paper**: el catálogo shadcn es solo web.
- Norte-Sur pasa por el API Gateway; Este-Oeste va directo por DNS interno.

**Lo que la presentación NO define** (no inventarlo; preguntarle al coordinador):
convención de nombres de archivo, alias de imports, dónde van los tests, si se usan
barrels, y cómo se comparten físicamente los schemas Zod.

La presentación menciona dos documentos más detallados —"Estructura de Carpetas" y
"Seguridad de la Arquitectura"—. **Corrección: el de seguridad SÍ está en la máquina.**
`~/Downloads/DEAL_Security_Architecture_Review*.pdf` (tres copias idénticas, la más
vieja del 2026-08-14) contiene la revisión de arquitectura de seguridad, autor Marcel Del
Castillo (Gerente de Sistemas y BI). Su contenido ya alimentó la skill `general-security`
(§9). "Estructura de Carpetas" sigue sin aparecer.

---

## 9. `command` / `agent` como nuevos tipos de artefacto (agent-artifact-taxonomy)

Extiende el modelo de artefactos de `skill|component|config` a también `command|agent`.
Los tres tipos "flat" (`skill`, `command`, `agent`) comparten la regla de no declarar
`dest` propio, pero **no** comparten cómo se deriva el nombre de archivo — ver la
corrección más abajo.

- `type: command` → instala en `.claude/commands/<leaf>.md`, donde `<leaf>` es solo el
  último segmento del id (**corregido**: antes era el id aplanado completo — ver
  "Corrección: nombre de archivo de un command" abajo). Frontmatter solo necesita
  `description` (no hay `name`: el nombre lo da el archivo).
- `type: agent` → instala en `.claude/agents/<id-aplanado>.md`. Frontmatter necesita
  `name` (debe matchear el id aplanado, igual que una skill), `description`, `model`,
  `tools`.
- Ambos, igual que las skills, se validan contra el `kit.yaml` real en
  `repo_manifest_test.go`, ahora generalizado a un `switch` sobre los tres tipos con
  frontmatter (antes solo cubría `skill`).
- `tool/internal/kit/frontmatter.go` (nuevo): `CheckFrontmatterName` y
  `CheckDescriptionPresent`, extraídos del chequeo que antes vivía inline en
  `repo_manifest_test.go`. Parsean solo el bloque `---`...`---` inicial (no todo el
  archivo) y comparan por igualdad, no por substring — la versión anterior aceptaba un
  nombre más largo con el prefijo correcto, o un `name:`/`description:` en cualquier
  parte del cuerpo. También aceptan un valor citado (`name: "web-ui"`).

Contenido de la primera porción:

- `skills/general/security/SKILL.md` (`general-security`, applies_to backend/web/mobile) —
  reglas de la revisión de arquitectura de seguridad (§8): TTL de token, scopes
  `verb:resource`, rotación de refresh, allowlist CORS, geoblocking, rate limit, masking
  de PII, TLS/AES-256, y que el API Gateway es quien las hace cumplir. No repite dónde
  se guarda el token (eso ya lo dicen `web-architecture` y `mobile-architecture`).
- `commands/{backend,web,mobile}/generate-schema.md` — backend escribe el DTO en
  `interface/dto/` con `nestjs-zod`; web y mobile escriben el schema en
  `features/<feature>/schemas/`. Ninguno copia/sincroniza entre repos: eso sigue sin
  definirse (§4.3).
- `agents/backend/review-security.md` (`backend-review-security`) — agente de solo
  lectura (`Read, Grep, Glob`). El cuerpo es un puntero: lee
  `.claude/skills/general-security/SKILL.md` primero y audita contra eso. No repite
  ningún valor concreto de la skill, y no emite un token tipo `STATUS: FAILED_*` — corre
  interactivo, no en CI.

Los cinco quedaron cableados en `kit.yaml`: `general/security` en los tres perfiles,
`backend/generate-schema` + `backend/review-security` en `backend`, `web/generate-schema`
en `web`, `mobile/generate-schema` en `mobile`.

**Verificación**: `go test ./... -count=1` (10/10 paquetes), `gofmt -l .` limpio,
`go vet ./...` limpio, `internal/cli` sin diff. E2E manual **solo pudo probarse
parcialmente**: el único proyecto de prueba disponible es
`/mnt/c/SoftwareDevelopment/deal-test/crm-deal-web` (perfil `web`), y su
`deal-kit.lock` quedó con una entrada obsoleta (`general/pr-workflow`, eliminada en
`03af755`, ver §5) que hace que `status`/`update` fallen con `unknown artifact` — un bug
preexistente y no relacionado a este cambio, no reparado porque el fixture queda fuera
del alcance de edición de esta sesión. **No hay proyecto de prueba `backend` ni
`mobile`.** Ningún perfil quedó verificado de punta a punta contra un binario real en
esta sesión; solo verificado vía tests Go y por inspección.

Sin operaciones de git: todo queda sin commitear en el working tree, apilado sobre WU1
(tipos `command`/`agent`, `frontmatter.go`, `plan.go`). PR 1 = plomería de WU1; PR 2 =
este contenido + wiring de `kit.yaml`, sobre PR 1 (`stacked-to-main`).

### Corrección: nombre de archivo de un `command`

`CommandFile()` usaba `InstallName()` (id aplanado), igual que `SkillDir()`/`AgentFile()`.
Instalar de verdad `web/generate-schema` en el proyecto de prueba mostró el problema: el
archivo quedaba en `.claude/commands/web-generate-schema.md`, invocable como
`/web-generate-schema` — contradice `/generate-schema`, que es lo que el equipo de
collections ya documentó y tipea. El binario real hizo evidente lo que el test suite
completo, `gofmt` y `go vet` no detectaron: ningún test afirmaba qué escribiría un
humano.

- `CommandFile()` ahora usa `LeafName()` (nuevo, en `kit.go`, junto a `InstallName()`):
  último segmento del id, sin prefijo de grupo. `SkillDir()`/`AgentFile()` **no**
  cambiaron — se quedan aplanados a propósito.
- Nueva invariante en `repo_manifest_test.go`, contra el `kit.yaml` real: dentro de un
  mismo project type, ningún par de commands instalables puede compartir `LeafName()`
  (el prefijo de grupo ya no los distingue). Hoy se cumple trivialmente (un
  `generate-schema` por tipo); `TestDuplicateCommandLeaf*` en `manifest_test.go` prueba
  que la invariante sí dispara, con un par sintético que colisiona a propósito — no se
  tocó el `kit.yaml` real para probarlo.
- Migración observada contra `/mnt/c/SoftwareDevelopment/deal-test/crm-deal-web`: el
  `deal-kit.lock` tenía `web/generate-schema → .claude/commands/web-generate-schema.md`
  (ruta vieja). Tras el cambio, `deal-kit status` marcó ese artefacto `OUTDATED` (no
  huérfano silencioso) y `deal-kit add web/generate-schema --dry-run` planeó
  `create .claude/commands/generate-schema.md` + `delete .claude/commands/web-generate-schema.md`
  automáticamente — la ruta de limpieza que `plan.Build` ya tenía para "el artefacto ya
  no produce este archivo, y el proyecto no lo editó localmente" cubre un cambio de
  destino sin intervención manual. Se aplicó de verdad (`--yes --no-deps`) en ese
  fixture de scratch para completar la verificación: el archivo viejo se borró, el nuevo
  quedó en `.claude/commands/generate-schema.md`, y `status` volvió a `ok`.
- `CommandFile()` y sus tests (`manifest_test.go`, `plan_test.go`) van en PR 1 (plomería,
  sin contenido). La invariante en `repo_manifest_test.go` va en PR 2 (corre contra el
  `kit.yaml` real, que es contenido de PR 2).

---

## 10. Engram para Claude Code (`feat/kit-engram-claude-code`)

Entrada nueva del menú de la TUI que instala el plugin **Engram** en Claude Code con
**alcance `user` (global)**, ejecutando el CLI `claude`. La TUI solo junta el
consentimiento; la ejecución pasa en la terminal normal **después** de que Bubble Tea
sale, igual que el install de dependencias.

### Qué se agregó

| Archivo | Qué hace |
|---|---|
| `tool/internal/engram/engram.go` | paquete nuevo: detección, plan inmutable, ejecución |
| `tool/internal/engram/engram_test.go` | fake `Runner`, y un E2E con `claude` simulado y `HOME` temporal |
| `tool/internal/tui/model.go` | `screenEngram`, entrada de menú, tecla `y`, `EngramIntent()` |
| `tool/internal/tui/view.go` | `engramLines()` + helpers compartidos `proseAt`/`keyLinesAt`/`clip` |
| `tool/internal/cli/interactive.go` | resuelve el estado antes de abrir la TUI y ejecuta después |
| `tool/internal/cli/render.go` | `renderEngram` (stdout/stderr separados) |
| `tool/internal/doctor/doctor.go` | `claude` y `engram` como herramientas **opcionales** |
| `tool/internal/execenv/execenv.go` | paquete nuevo: el entorno saneado que comparten `kit` y `engram` |

### Decisiones

| Decisión | Razón |
|---|---|
| Se ejecuta `claude`, no se editan sus archivos | `claude` es dueño de ese formato; un segundo escritor se desincroniza en el primer cambio de formato. |
| Los argumentos son constantes del paquete | Nunca salen de `kit.yaml`, de un flag ni del entorno. Un instalador que toma sus argumentos de datos se puede apuntar a otro repo editando datos. Nunca hay shell ni `sh -c`. |
| El `#<tag>` va solo en `marketplace add` | Es el único comando que acepta ref; `install` no. Hoy fijado en `v1.20.0`. |
| `--yes` en `install`, no en `enable` | `install` lo requiere sin TTY; `enable` no tiene el flag. El gate real de consentimiento es la tecla `y` de la TUI. |
| Identidad del marketplace por el campo `repo` | El JSON no trae URL completa ni ref. Se compara sin distinguir mayúsculas, `.git` ni barras. |
| Un marketplace `engram` con otro `repo` **nunca** se toca | El nombre lo ocupó algo que el usuario configuró; reemplazarlo es decisión suya. |
| JSON ilegible ⇒ `StateUnknown`, no "no instalado" | Adivinar "no instalado" hace que deal-kit vuelva a agregar un marketplace que ya está. |
| Se ejecuta la ruta que resolvió el `Lookup`, no el nombre pelado | Resolver una cosa y ejecutar otra es cómo un test "hermético" termina corriendo el `claude` real. Se descubrió así: la primera versión del E2E ejecutó el binario de verdad. |
| Solo scope `user` cuenta como instalado | Una copia con scope `project` es otra decisión y no puede hacer parecer hecho el install global. |
| `engram setup claude-code` queda **fuera** | Registra el MCP: cambia permisos y otros archivos globales. Se avisa que queda pendiente. |
| Se sacó la entrada "Actualizar el kit" del menú | `u` ya actualiza desde "Estado del proyecto" y su leyenda lo dice. `catalog_test.go` exige ≤ 6 entradas y el menú tiene que seguir siendo escaneable. |
| `EngramIntent()` separado de `Result()` | `Result()` contesta "¿hubo sync del kit?". Sobrecargarlo haría que un booleano signifique dos cosas sin relación. |
| Engram no es artefacto de `kit.yaml` | Escribe en la config global del usuario, no en el proyecto. Por eso "Instalar todo" no lo incluye — hay un test que lo fija. |

### Flags

- `--dry-run`: consulta y muestra; `y` no produce intención. Se re-chequea en
  `browse()` además de en la pantalla: una mutación tan lejos del flag merece dos gates.
- `--offline`: permite las consultas (son lecturas locales) y bloquea lo que descarga.
  Un plan que solo tiene `enable` **sí** se permite: no contacta nada.
- `--yes`: **no** saltea la confirmación de la TUI.
- `--no-deps`: no aplica acá.

### Trampas nuevas

- **`Plan.Steps()` tenía que copiar en profundidad.** Copiar solo el slice deja
  compartido el array de cada `Args`, y quien reescribiera un elemento cambiaba lo que
  se ejecuta. Hay test.
- **Un token más largo que el panel no se puede word-wrappear.** La URL del marketplace
  mide 63 caracteres; a 40 columnas desborda. Se agregó `clip()` y un test de ancho a
  6 anchos × 6 estados, midiendo con `lipgloss.Width`.

### Correcciones de la revisión 4R

Cinco hallazgos de la revisión, corregidos sobre el working tree de la rama.

| # | Qué estaba mal | Cómo quedó |
|---|---|---|
| 1 | Los comandos mutantes no imprimían nada: `ExecRunner.Run` bufferaba stdout y `Apply` descartaba esos bytes. `marketplace add` clona un repo (~13s acá, minutos en una red lenta) con la terminal muda, indistinguible de un cuelgue. | `Runner` ahora tiene dos métodos. `Run` sigue capturando stdout para parsear JSON (y stderr aparte, o una línea de warning contamina el parse). `RunStream` escribe stdout y stderr a un `io.Writer` en vivo; `Apply` recibe ese writer y `installEngram` le pasa `e.Stdout`. En tests el writer es un buffer o `nil`, así que nada deja de ser hermético. |
| 2 | `Ctrl+C` mataba el proceso: `grep -rn "os/signal"` en `tool/` no devolvía nada. El camino elegante de `Apply` (chequeo de `ctx.Err()` + `detectFresh` sobre contexto desprendido) era código muerto desde la terminal. | `engramInstallContext()` en `internal/cli/interactive.go` arma el contexto con `signal.NotifyContext` para SIGINT/SIGTERM además del timeout. Solo alrededor del install de Engram: manejo global de señales es otra decisión. La señal llega a todo el grupo de procesos, así que `claude`/`git` mueren solos; lo que se gana es que deal-kit sobreviva a re-consultar y decir qué quedó instalado. |
| 3 | `Applied()` era `Err == nil` y nunca miraba `Status`. Con las tres mutaciones OK y el re-chequeo final fallando (JSON ilegible, o presupuesto agotado tras un clone lento), el CLI salía 0 e imprimía "desconocido" por **stdout** como si fuera informativo. Además ese re-chequeo corría sobre el contexto compartido, no sobre uno fresco como el de la ruta de falla. | El re-chequeo del loop usa `detectFresh` (mismo tratamiento que la ruta de falla, con el porqué en el comentario). Se agregó `Outcome.Verified()` = `Err == nil && Status.State == StateReady`. `installEngram` distingue tres finales: **verificado** (sale 0), **aplicado pero no verificable** (error + advertencia por stderr, sale 1) y **falló** (como antes). Un install genuinamente exitoso sigue saliendo 0. |
| 4 | `leakedGitVars`/`sanitizedEnv` estaba duplicado casi textual entre `internal/engram` e `internal/kit`. Es lista de seguridad: existe para que un `GIT_DIR` heredado no redirija el clone del marketplace. Dos copias divergen apenas se arregla una. | Nuevo paquete `internal/execenv` (`LeakedGitVars`, `Sanitized`). Se eligió un paquete propio y no exportarlo desde `internal/kit` porque `internal/engram` no tiene nada que ver con el fetch del kit y no debe depender de él. Cada paquete conserva un `sanitizedEnv()` de una línea que delega, así los tests existentes de ambos siguen valiendo. |
| 5 | `--offline` tenía un solo gate, dentro de `engramBlocked()` en la TUI. `installEngram` nunca veía `e.Offline`, al revés de `--dry-run`, que sí se re-chequea en `browse()` con el comentario "una mutación tan lejos del flag merece dos gates". | `installEngram` rechaza el plan si `e.Offline && p.NeedsDownload()`. `needsDownload` se movió de `internal/tui/model.go` a `Plan.NeedsDownload()` para que el gate de la TUI y el del CLI no puedan responder distinto. Habilitar un plugin ya en disco sigue permitido offline. |

Tests que lo fijan:

| Fix | Test |
|---|---|
| 1 | `engram.TestMutatingStepsStreamTheirOutputToTheCaller`, `engram.TestQueryOutputNeverReachesTheLiveWriter`, `engram.TestExecRunnerStreamsBeforeTheCommandExits` (el script no termina hasta que el test vio su primera línea: prueba que el stream es real y no un buffer volcado al salir) |
| 2 | `cli.TestAnInterruptedInstallReportsWhatLandedInsteadOfDying` (manda un SIGINT de verdad al proceso de test desde dentro del comando mutante) |
| 3 | `engram.TestSucceedingCommandsWithAnUnreadableRecheckAreNotVerified`, `engram.TestAFinishedInstallIsVerified`, `engram.TestTheFinalRecheckDoesNotInheritAnExhaustedBudget`, `cli.TestInstallEngramDoesNotReportAnUnverifiableRunAsSuccess` |
| 4 | `kit.TestSanitizedEnvDropsRepositoryOverrides` y `engram.TestSanitizedEnvDropsLeakedGitVariables`, sin cambios: ahora ejercitan el helper compartido |
| 5 | `cli.TestInstallEngramRefusesToDownloadWhileOffline`, `cli.TestInstallEngramStillEnablesWhileOffline`, y el ya existente `tui.TestOfflineStillAllowsEnablingWhatIsAlreadyOnDisk` |

Los dos tests de la #2 y la #3 se verificaron al revés: sin la corrección, el de la
interrupción muere con `signal: interrupt` (el binario de test lo mata la señal) y el
del presupuesto agotado falla reportando estado `0` (`StateUnknown`).

### Riesgo aceptado: `MarketplaceTag` pinea un tag **mutable**

`MarketplaceTag = "v1.20.0"` es un tag de git, y un tag se puede mover. Quien controle
el repo del marketplace puede reapuntarlo a otro commit y el próximo
`marketplace add` traería ese contenido. Un SHA de 40 caracteres sería inmutable.

**No se puede cerrar, y no es un TODO.** Verificado: `claude plugin marketplace add
<url>#<ref>` rechaza un SHA con `Remote branch not found`, porque Claude Code implementa
el fragmento como `git clone --branch <ref>`, y `--branch` solo acepta ramas y tags.
Reproducible fuera de `claude`:

```
$ git clone --branch 583ac7e96382998c48534a91b5f44e282d44430e origin clone
fatal: Remote branch 583ac7e96382998c48534a91b5f44e282d44430e not found in upstream origin
```

Queda como riesgo aceptado y documentado: el pin por tag es lo más fuerte que la
interfaz de `claude` permite hoy. Si alguna versión de Claude Code acepta un SHA,
recién ahí se puede cerrar. Nadie lo "arregla" cambiando la constante por un SHA: eso
rompe el install.

### Verificación

`gofmt -l .` limpio · `go vet ./...` limpio · `go test ./... -count=1` 12/12 paquetes
(entra `internal/execenv`, sin tests propios: lo cubren los dos que lo usan) ·
goldens regenerados sin diff (`menu`, `engram-confirm`, `engram-ready`, `engram-conflict`).

**No se corrió un install real**: esta máquina ya tiene el marketplace agregado y el
plugin instalado y habilitado, así que un run de verdad no ejercitaría el camino
interesante y tocaría configuración global. Se llegó hasta `--dry-run`.
