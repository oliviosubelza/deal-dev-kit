# deal-dev-kit — estado y pendientes

Documento de traspaso. **Sí está versionado**: `git ls-files HANDOFF.md` lo lista y
no aparece en `.gitignore` (la versión anterior de esta línea afirmaba lo contrario).
Última actualización: 2026-09-08.

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
| `ui-kit/package.json` privado, solo para CI | El kit se sigue distribuyendo copiando fuente: ese `package.json` no se publica, no buildea y no cambia nada de la instalación. Existe para que `tsc --noEmit` pueda correr en CI. Sus versiones se derivan de los bloques `npm:` de `kit.yaml`, así que se compila contra lo mismo que instalan los proyectos. |

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

cd /mnt/c/SoftwareDevelopment/deal-dev-kit/ui-kit
npm ci && npm run typecheck    # tsc --noEmit sobre los 70 .ts/.tsx del ui-kit
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
| Identidad del marketplace por `owner/name`, salga de `repo` o de `url` | **Corregido — la versión original de esta fila decía "el JSON no trae URL completa ni ref", y es falso: ver §13.** `claude plugin marketplace list --json` devuelve dos formas. El atajo `owner/name` da `source: "github"` con `repo`; una URL da `source: "git"` con `url` y `ref` y **sin** `repo`. deal-kit agrega por URL, así que la segunda forma es la suya. Se compara `owner/name` sin distinguir mayúsculas, `.git` ni barras, y una URL que no se puede reducir a `owner/name` no matchea. |
| Un marketplace `engram` con otro `repo` **nunca** se toca | El nombre lo ocupó algo que el usuario configuró; reemplazarlo es decisión suya. |
| JSON ilegible ⇒ `StateUnknown`, no "no instalado" | Adivinar "no instalado" hace que deal-kit vuelva a agregar un marketplace que ya está. |
| Se ejecuta la ruta que resolvió el `Lookup`, no el nombre pelado | Resolver una cosa y ejecutar otra es cómo un test "hermético" termina corriendo el `claude` real. Se descubrió así: la primera versión del E2E ejecutó el binario de verdad. |
| Solo scope `user` cuenta como instalado | Una copia con scope `project` es otra decisión y no puede hacer parecer hecho el install global. |
| `engram setup claude-code` queda **fuera** | **Corregido — el razonamiento original de esta fila era falso, igual que la de identidad del marketplace.** Decía que registra el MCP y que por eso quedaba pendiente. No queda pendiente nada: el plugin trae su propio `.mcp.json`, así que `plugin install` ya registra el servidor MCP. Verificado instalando contra un `HOME` vacío y leyendo `claude plugin list --json`, que devuelve `"mcpServers": {"engram": {"command": "engram", "args": ["mcp","--tools=agent"]}}`. En `docs/PLUGINS.md` de upstream ese comando es una **alternativa** al install por marketplace, no un paso posterior. La pantalla y `renderEngram` mandaban al usuario a cambiar permisos globales para nada; ver §15. |
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

---

## 11. Instalación del binario `engram` (`feat/kit-engram-claude-code`)

La TUI instalaba el **plugin** y nunca el **binario**. El plugin trae
`.mcp.json` con `{"engram": {"command": "engram", ...}}` y todos sus hooks
invocan `engram`, así que sin el binario el MCP no arranca y cada hook falla.
`Status.EngramBinaryFound()` ya lo reportaba y nadie actuaba: una máquina con el
plugin en `StateReady` y sin binario devolvía `Plan{}`.

| Archivo | Qué hace |
|---|---|
| `tool/internal/engram/binary.go` | `go install`, asset de release, checksum, extracción, escritura atómica |
| `tool/internal/engram/binary_test.go` | release falsa con `httptest`, `HOME` temporal |
| `tool/internal/{engram,cli,tui}/main_test.go` | default-deny de red y de `HOME` en los tests |

### Decisiones

| Decisión | Razón |
|---|---|
| El binario va **primero** en el plan | Es el motor. Instalar el plugin antes deja una ventana con hooks que no pueden correr. Por eso `StateReady` sin binario ahora sí produce plan. |
| Se instala el build de `runtime.GOOS`/`GOARCH` | Es el mismo entorno donde se resolvió `claude`. Un engram de Linux no le sirve a un Claude Code nativo de Windows. |
| `go install` antes que el asset | Es la recomendación de upstream para Windows: Defender y ESET marcan sus binarios prebuilt sin firmar como falso positivo. Además reusa `Runner`/`RunStream` sin tocar nada. |
| Se reusa `MarketplaceTag` para el binario | Plugin y binario no pueden derivar. Los assets llevan el tag sin la `v`. |
| Solo `amd64` y `arm64` | Es lo único que publica el release. Otro `GOARCH` da plan vacío con motivo, nunca una URL adivinada. |
| Checksum obligatorio | Un asset que no coincide, o que no está en el manifiesto, no se instala. |
| Temp + `os.Rename` | Una descarga interrumpida no deja un ejecutable truncado en el PATH. |
| **deal-kit no toca el PATH** | Misma clase de mutación global que ya se rechaza con el marketplace en conflicto. Desde §15 entrega además el comando exacto para agregarlo (`tui.PathHint`): negarse a ejecutarlo no es razón para que el usuario tenga que deducirlo. |
| `Verified()` exige el binario | Un `StateReady` sin binario no es una instalación que funcione. |
| Presupuesto: se nombra al que lo consumió, no se reparte | Un budget por paso mal elegido convierte un install lento pero sano en una falla. |

### Defecto encontrado corriendo el instalador de verdad

`claude plugin install` **ya habilita** el plugin. Planear `plugin enable`
después hacía que toda instalación desde cero terminara en
`Plugin "engram@engram" is already enabled at user scope` y saliera distinto de
cero, sobre un install que había funcionado.

`planFor` ya no planea `StepEnable` en `StateMarketplaceMissing` ni en
`StatePluginMissing`. `StatePluginDisabled` es el único estado donde habilitar
es trabajo propio.

La suite nunca lo agarró porque el `claude` simulado era infiel: su `install`
dejaba el plugin deshabilitado y su `enable` aceptaba cualquier cosa. Los dos
fakes ahora instalan-y-habilitan y rechazan un `enable` redundante con el
mensaje real y exit distinto de cero. `TestNoPlanBothInstallsAndEnables` fija la
regla. **Lección: un fake que acepta una secuencia imposible no es una
simplificación, es un punto ciego.**

### Trampas nuevas

- **La suite descargaba el release real y pisaba el `engram` del desarrollador.**
  `PlanFor` antepone un `StepBinaryDownload` a cualquier `Status` sin
  `EngramPath`, y ese paso es la única mutación que no pasa por `Runner`.
  Verificado: `stat -c %Y ~/.local/bin/engram` cambiaba al correr `go test`, y
  un test tardaba 94.54s. Arreglo: `releaseBase` arranca en un centinela que no
  resuelve cuando `testing.Testing()` es true, más `TestMain` en los tres
  paquetes redirigiendo `HOME`/`LOCALAPPDATA`. Se eligió el centinela sobre un
  `TestMain` por paquete porque `releaseBase` no está exportada. `internal/engram`
  pasó de 94.54s a 0.08s.
- **La descarga era una terminal muda**, la fix #1 de la sección 10
  reintroducida. `downloadTo` ahora escribe progreso al mismo `live`, en líneas
  enteras y no con `\r`: `live` puede ser un pipe.
- **La advertencia de Windows salía en todos los sistemas** en la TUI, y tres
  goldens la habían fijado. `var hostGOOS` para que los snapshots sean
  deterministas entre plataformas.
- **Un binario preexistente en el destino se pisaba sin avisar.** `Download.Replaces`,
  resuelto al armar el plan: la pantalla de confirmación solo puede ser
  consentimiento informado si ya lo sabe.

### Riesgo aceptado: el checksum verifica transporte, no procedencia

`checksums.txt` se baja del **mismo release** que el asset. Detecta un asset
truncado o un intermediario, pero no a quien controle el release. Es el mismo
riesgo del tag mutable y se cierra con lo mismo que no existe: una firma con una
clave que no viva en el release.

### Verificación

`gofmt -l .` limpio · `go vet ./...` limpio · `go test ./... -count=1` verde ·
goldens regenerados · hermeticidad probada a nivel máquina (`stat` del binario
real sin cambios tras una corrida completa).

Se corrió el instalador **de verdad** contra un `HOME` vacío: descarga con
progreso, checksum verificado, `engram 1.20.0` ejecutable en el destino, plugin
instalado y habilitado, `mcpServers` presente en `claude plugin list --json`.
Ahí apareció el defecto del `enable`.

---

## 12. Persona de comunicación (`config/persona.md`)

Regla de tono siempre activa, instalada en los tres perfiles como
`.claude/persona.md`. Es el primer artefacto `type: config` del kit, y estrena el
directorio `config/`.

| Archivo | Qué hace |
|---|---|
| `config/persona.md` | el texto de la persona (inglés, como el resto de los artefactos) |
| `kit.yaml` | artefacto `general/persona` + entrada en los tres `profiles`, después de `general/tdd` |
| `README.md` | sección "The communication persona" (reescrita en §13) |

### Decisiones

| Decisión | Razón |
|---|---|
| `config`, no `skill` | Una skill se carga solo cuando el modelo juzga que su `description` matchea la tarea. Una regla de tono que vale para **toda** respuesta no puede ser condicional. `general/conventions`, `general/security` y `general/tdd` sí describen una condición ("antes de escribir código"); esta no tiene ninguna. |
| Archivo propio + `@import`, no escribir `CLAUDE.md` | Un artefacto `config` copia su `src` tal cual a `a.Dest`. Apuntar `dest` a `CLAUDE.md` pisaría lo que el proyecto ya tenga ahí. |
| ~~La línea `@.claude/persona.md` la agrega una persona, una vez~~ | **Revertida en §13.** El paso manual se olvidaba y el fallo era silencioso: el archivo instalado, la persona nunca cargada, y `status` en `ok`. Ahora la agrega `ensure_line`. |
| **Sin frontmatter** | `CheckFrontmatterName` se invoca solo desde `repo_manifest_test.go:59,68`, dentro de un `switch a.Type` con casos `skill`, `agent` y `command`. `config` cae en el default: no se chequea. Un frontmatter decorativo sería ruido que nada valida. |
| `applies_to` solo en `kit.yaml` | La herramienta lo lee de `manifest.go:30` y nunca del archivo. Que 2 de 9 skills lo repitan en su frontmatter es drift, no convención. |

### Verificación

`go test ./internal/kit/ -count=1` ok · `gofmt -l .` limpio · `go vet ./...`
limpio · `go test ./... -count=1` 12/12. Goldens de la TUI sin diff (el artefacto
entra en el grupo "Team conventions", que ya existía).

Binario real (`/tmp/deal-kit`, `--kit-dir` al working tree) contra tres proyectos
scratch fuera del repo: `crm-deal-web` instalado de verdad — `.claude/persona.md`
idéntico byte a byte a `config/persona.md`, registrado en `deal-kit.lock`, y
`status` lo reporta `general/persona ok`. `crm-deal-orders-service` y
`crm-deal-mobile` verificados con `--dry-run`: el plan incluye
`crear .claude/persona.md` en ambos.

### Tag

`kit-v*` únicamente: no hay cambios bajo `tool/`. **Superado por §14**, que
agrega la capacidad `ensure_line` en `tool/` y por lo tanto necesita los dos.

---

## 13. Defecto: deal-kit no reconocía su propio marketplace (`fix/engram-marketplace-identity`)

Confirmado en una máquina Windows real. Un solo bug con tres síntomas.

`claude plugin marketplace list --json` devuelve **dos formas distintas** según
cómo se agregó el marketplace:

```json
{ "name": "claude-plugins-official", "source": "github", "repo": "anthropics/claude-plugins-official", "installLocation": "..." }
{ "name": "engram", "source": "git", "url": "https://github.com/Gentleman-Programming/engram.git", "ref": "v1.20.0", "installLocation": "..." }
```

`MarketplaceAddArgs` agrega **por URL**, o sea que la forma que produce el
propio deal-kit es la segunda: `source: "git"`, con `url` y `ref` y **sin
`repo`**. `Detect` comparaba identidad usando solo `repo`, así que
`sameRepo("", "Gentleman-Programming/engram")` daba falso y el estado salía
`StateMarketplaceConflict`: **deal-kit no reconocía su propia instalación**.

Consecuencias observadas: la pantalla decía `Ya hay un marketplace llamado
engram que apunta a —` (el `—` era el `repo` vacío), el plan quedaba vacío, no
se instalaba ni el plugin ni el binario, `engram` no quedaba en el PATH y Claude
Code reportaba `Failed to reconnect to plugin:engram:engram`.

Era invisible en Linux solo porque esa máquina tenía el marketplace agregado con
el atajo `owner/name`, que sí llena `repo`.

### Qué se corrigió

| Qué | Cómo quedó |
|---|---|
| Identidad | `marketplace.identity()` devuelve el `owner/name` a comparar y el texto a mostrar. Sale de `repo` si está; si no, de `url` vía `repoFromURL`. |
| Normalización de URLs | `repoFromURL` lee URL con esquema y la forma scp de ssh (`git@github.com:owner/name.git`), descarta userinfo, puerto, `?query` y `#fragment` (el `#ref` que agrega `MarketplaceAddArgs`), la barra final y el `.git`. Compara host contra `MarketplaceHost` sin distinguir mayúsculas. |
| Lo que no se puede leer no matchea | Una URL que no se reduce a exactamente dos segmentos, o que apunta a otro host, devuelve `""` y **no** matchea: desconocido sigue siendo desconocido, igual que con el JSON ilegible. Nuestro `owner/name` en `evil.example.com` es otro repositorio y sigue siendo conflicto. |
| El conflicto real sigue siendo conflicto | Un marketplace `engram` que apunta a otro lado sigue dando `StateMarketplaceConflict`, plan vacío y cero mutaciones. El bug era un falso positivo, no una excusa para dejar de chequear. |
| `Status.FoundRepo` honesto | Se llena con lo que identificó al marketplace: el `owner/name` cuando se pudo leer, y la URL cruda cuando no. La pantalla de conflicto nombra la cosa real en vez de un `—`. |
| `MarketplaceHost` | Constante nueva, fijada contra `MarketplaceURL` por `TestTheMarketplaceURLResolvesToTheMarketplaceRepo`: nada más ataba esas dos constantes, y editar una sin la otra reproduce este mismo defecto. |

### Decisión: el `ref` se **reporta**, no es un estado ni se re-apunta

El JSON trae el ref, así que ahora `Status.FoundRef` lo guarda y
`Status.RefMismatch()` dice si el marketplace registrado está en otro tag que
`MarketplaceTag`.

**No es un `State` nuevo.** Los estados son una escalera de cuánto avanzó la
instalación, y un marketplace en otro tag no está ni más arriba ni más abajo: es
el repositorio correcto, así que no es conflicto, y el plugin se instala igual
desde ahí. Volverlo estado obligaría a `PlanFor` a contestarlo y las dos
respuestas posibles son malas: un plan vacío se negaría a instalar un plugin que
instala perfecto (el mismo síntoma que este defecto), y un plan que lo re-apunta
tendría que hacer `marketplace remove` de algo que registró el usuario. Igual que
con el marketplace ajeno: se reporta y no se toca.

`FoundRef` se llena **solo** cuando el marketplace es el nuestro; el pinning de
un tercero no es asunto de deal-kit. Y el atajo `owner/name` no trae ref: ref
desconocido no es drift.

Dónde se ve: el campo `marketplace` de la pantalla (`v1.20.0  (fijado) ·
registrado en v1.19.0`), una advertencia en prosa que dice que hay que quitarlo y
volver a agregarlo a mano, y una línea en `renderEngram`.

### Tests, y cómo se verificaron al revés

Cada uno se corrió con la corrección sacada a mano. Mensaje sin la corrección:

| Test | Sin la corrección |
|---|---|
| `TestTheURLShapeIsRecognisedAsOurOwnMarketplace` (el JSON real de arriba, textual como fixture `urlMarketplace`) | `State = 3, want StateReady` (3 = `StateMarketplaceConflict`) |
| `TestTheShorthandShapeIsStillRecognised` | (no regresiona: fija que la forma `github` sigue andando y que un ref desconocido no es drift) |
| `TestAForeignMarketplaceIsAConflictInEveryShape` | con identidad solo por `repo`: `FoundRepo = "", want "someone-else/engram"` · sin chequeo de host: `State = 6, want StateMarketplaceConflict` (6 = `StateReady`) · con `FoundRepo` = solo lo que matcheó: `FoundRepo = "", want "engram"` |
| `TestAMarketplaceAtAnotherRefIsReportedAndStillInstallable` | `State = 3, want StatePluginMissing`; y sin guardar el ref: `FoundRef = "", RefMismatch = false, want "v1.19.0" and true` |
| `TestRepoFromURLReadsOnlyWhatItCanVerify` | sin chequeo de host: `repoFromURL("https://gitlab.com/Gentleman-Programming/engram.git") = "Gentleman-Programming/engram", want ""` |
| `TestTheMarketplaceURLResolvesToTheMarketplaceRepo` | (fija `MarketplaceURL` ↔ `MarketplaceHost` ↔ `MarketplaceRepo`) |
| `tui.TestAMarketplaceAtAnotherRefIsNamedOnTheScreen` | `the screen never names the registered ref` |

---


## 14. `ensure_line`: garantizar una línea en un archivo que el kit no posee

El artefacto `general/persona` instalaba `.claude/persona.md`, pero Claude Code
no lo carga hasta que el `CLAUDE.md` del proyecto tiene `@.claude/persona.md`.
Esa línea la agregaba una persona a mano, una vez por repo (§12). **Cuando se
olvidaba, no había síntoma**: el archivo estaba instalado, la persona nunca
cargaba, `status` decía `ok`. Un fallo silencioso no es un paso manual, es un
bug con documentación.

`ensure_line` es la capacidad nueva: garantizar que **una** línea esté presente
en un archivo que el kit **no** posee, sin reescribir el resto.

```yaml
- { id: general/persona, type: config, ..., dest: ".claude/persona.md",
    ensure_line: { file: "CLAUDE.md", line: "@.claude/persona.md" } }
```

| Situación | Resultado |
|---|---|
| el archivo no existe | se crea con la línea |
| existe y ya tiene la línea | no-op, `status` → `ok` |
| existe y le falta la línea | se agrega al final, byte por byte intacto lo demás |

### El problema de diseño: qué estado se trackea

El lockfile guarda un `hash` por archivo instalado, y `status` reporta drift
comparando. **Ese modelo no sirve acá.** `CLAUDE.md` lo edita el equipo todo el
día por razones que no tienen nada que ver con el kit: hashearlo haría que
`status` dijera "cambiado" en **cada** corrida, y un status que siempre grita
entrena a la gente a no leerlo. Es peor que no reportar nada.

Se verificó modelándolo mal a propósito (mutación M5: registrar `CLAUDE.md`
como `OwnedFile` con hash). Resultado: el primer edit del equipo bloquea el
sync entero con `1 archivo(s) requieren atención antes de aplicar`.

Entonces el estado trackeado es **presencia, no contenido**:

| Archivo | Se registra en | Lleva hash | `status` pregunta |
|---|---|---|---|
| `.claude/persona.md` | `files:` | sí | ¿cambió el contenido? |
| `CLAUDE.md` | `lines:` | **no** | ¿sigue estando la línea? |

`lockfile.EnsuredLine{Path, Line}` es el registro nuevo. Se guarda igual —
aunque la respuesta se recalcula del archivo en cada plan — para que
`deal-kit.lock` siga mostrando qué le hizo el CLI al proyecto: una mutación sin
registro es una mutación no auditable.

Etiqueta propia en `status`: **`FALTA IMPORT`**, no `DESACTUALIZADO`. Contesta
la pregunta útil ("¿está el import?") y no la engañosa ("¿cambió el archivo?").

### Dónde vive cada cosa

| Archivo | Qué hace |
|---|---|
| `tool/internal/plan/ensureline.go` | nuevo: `ensureLineAction` (decide) y `appendLine` (escribe) |
| `tool/internal/plan/plan.go` | `Kind` nueva `AppendLine`, campo `Action.Line`, mapa `ensured`, rama en `Apply`, `recordedIDs()` |
| `tool/internal/plan/summary.go` | `DirSummary.Lines`: sin esto el árbol final sumaba 11 y el pie decía 12 |
| `tool/internal/lockfile/lockfile.go` | `Installed.Lines []EnsuredLine`, sin hash y con el porqué en el comentario |
| `tool/internal/kit/kit.go` + `manifest.go` | `ensure_line: {file, line}` en `kit.yaml`, validado |
| `tool/internal/cli/render.go` | `agregar línea`, `FALTA IMPORT`, `kindW` 12 → 13 |
| `tool/internal/tui/view.go` | glifo, la línea en la fila, contador propio en `summary()` |

### Decisiones

| Decisión | Razón |
|---|---|
| La línea sale de `kit.yaml`, no está hardcodeada | Es dato del artefacto, no de la herramienta. |
| Una sola línea literal, un solo archivo | No es un motor de templates. Cualquier cosa más rica convierte al kit en segundo autor de un archivo que no posee, que es justo lo que las reglas de propiedad prohíben. Un valor con `\n` se rechaza al parsear. |
| `AppendLine` **nunca** puede quedar `Blocked` | Todo otro destino bloquea cuando el proyecto es dueño del archivo, porque escribirlo destruye trabajo. Acá que el proyecto sea dueño es el caso normal: se agrega una línea y no se reescribe nada, así que no hay nada que perder ni que rechazar. |
| Match por línea con `TrimSpace`, no `Contains` | Un import indentado, o con CRLF, ya está: agregar otro sería la herramienta discutiéndole al archivo. Y `@.claude/persona.md.bak` **contiene** la línea sin **ser** la línea. |
| Se re-chequea la presencia dentro de `appendLine` | `Apply` tiene que converger aunque el archivo cambie entre planear y escribir. Un import duplicado es el único modo de falla plausible de esta feature. |
| Se preserva el modo del archivo | Es del proyecto; el CLI es un invitado. |
| El `file` pasa por `paths.Resolve` | Mismo guard de traversal que cualquier otro destino. Hay test con `../outside`. |
| **Quitar la línea queda fuera de alcance** | El código existente borra archivos de forma uniforme, pero acá no hay archivo que borrar. Si el artefacto se desinstala o queda huérfano, `lock.Remove(id)` se lleva el registro y **la línea queda en `CLAUDE.md`**. Es lo honesto: editar un archivo ajeno para sacarle algo es otra decisión, y bastante más peligrosa que agregarlo. |

### Tests, cada uno mapeado a su comportamiento

Todos verificados al revés, mutando la corrección afuera:

| Mutación | Tests que caen |
|---|---|
| M1 `Build` no planea el `ensure_line` | 13 tests en `plan` y `cli` |
| M2 `appendLine` no separa con `\n` | `TestEnsureLineDoesNotJoinTheProjectsLastLine` (`"no trailing newline@.claude/persona.md"`) |
| M3 `hasLine` usa `strings.Contains` | `TestEnsureLineDoesNotMatchASubstring` (`kind = "unchanged", want "append-line"`) |
| M4 `appendLine` pisa en vez de agregar | `TestEnsureLineAppendsWithoutTouchingWhatIsAlreadyThere` + 4 más |
| **M5 `CLAUDE.md` con hash en `files:`** | `TestEditingTheRestOfTheFileIsNotReportedAsDrift` (`blocked = [...]`), `TestEnsureLineIsRecordedAsAPresenceNotAHash`, `TestASecondInitLeavesClaudeMDAloneAndReportsOK` (`1 archivo(s) requieren atención`) |
| M6 sin etiqueta `FALTA IMPORT` | `TestStatusReportsAMissingImportAndInitRestoresIt` |
| M7 `AppendLine` fuera de `Changes()` | `TestDryRunShowsTheImportAndWritesNothing` |
| M8 sin validación de `ensure_line` | `TestParseManifestRejectsAnIncompleteEnsureLine` (4 subcasos) |

### Verificación

`gofmt -l .` limpio · `go vet ./...` limpio · `go test ./... -count=1` 12/12 ·
goldens de la TUI regenerados sin diff (ninguna fixture existente está en otro
ref) · el chequeo de hermeticidad sobre `~/.local/bin/engram` sigue dando
`HERMETIC`.

### Tag

`v*`: los cambios son de `tool/`.

---

### Verificación (§14)

goldens de la TUI regenerados **sin diff** (ningún fixture de la TUI declara
`ensure_line`; lo cubren dos tests directos en `internal/tui/ensureline_test.go`)
· hermeticidad `HERMETIC` (`stat` de `~/.local/bin/engram` sin cambios).

Binario real (`/tmp/deal-kit`, `--kit-dir` al working tree) contra proyectos
scratch en `/tmp/scratch/`, los cuatro casos:

1. Proyecto sin `CLAUDE.md` → `init` lo crea con la línea sola.
2. `CLAUDE.md` con contenido y **sin salto de línea final** → línea agregada;
   `head -c <tamaño original>` del resultado hashea idéntico al original, o sea
   ningún byte previo se movió. `--dry-run` antes: sha256 sin cambios.
3. `init` de nuevo → `ya está actualizado`, sha256 igual, `status` → `ok`,
   `grep -c` del import → `1`.
4. Se borra la línea → `status` dice `general/persona  FALTA IMPORT  CLAUDE.md`
   y `init` la repone al final, conservando lo que el equipo escribió después.
   Editar el **resto** del archivo (caso 4a) sigue dando `ok`.

### Tag

`v*` **y** `kit-v*`: hay cambios bajo `tool/` (la capacidad) y en `kit.yaml`
(el artefacto que la usa). **El `v*` primero**: un binario anterior a este
cambio ignora `ensure_line` en silencio — el campo no existe en su
`rawArtifact`, así que instala la persona y nunca agrega el import, que es
exactamente el bug que esto arregla.

---

## 15. La pantalla de Engram: una instrucción falsa y tres veces el PATH

Salida real de una máquina Windows. Dos problemas distintos.

### El defecto: `engram setup claude-code` no queda pendiente

La pantalla y `renderEngram` decían *"Después queda pendiente `engram setup
claude-code`, que registra el servidor MCP"*. Es falso. El plugin trae su propio
`.mcp.json`; instalarlo y habilitarlo **es** lo que registra el servidor MCP.
Verificado instalando contra un `HOME` vacío: `claude plugin list --json`
devolvió `"mcpServers": {"engram": {"command": "engram", "args":
["mcp","--tools=agent"]}}`. Upstream lo lista en `docs/PLUGINS.md` como
**alternativa** al install por marketplace, no como paso siguiente.

Consecuencia: mandaba a cada usuario a ejecutar un comando que cambia permisos y
archivos globales, para nada. La fila de §10 que lo registraba como exclusión
deliberada quedó corregida en el mismo estilo que la fila de identidad del
marketplace en §13.

### El ruido: el PATH dicho tres veces y arreglado ninguna

`engram no está en el PATH` aparecía en la fila de la tabla, en una nota debajo
de `destino` y otra vez en el bloque de advertencias, y lo más parecido a una
instrucción era *"agregarlo a mano"*.

Ahora es **un** bloque de acción, último antes de las teclas, con el comando de
la plataforma del host y el directorio real que resolvió el plan:

```
   Falta: engram no está en el PATH — el servidor MCP no va a arrancar.

     setx PATH "%PATH%;C:\Users\...\AppData\Local\Programs\engram"

   Después reiniciar Claude Code.
```

En POSIX es `export PATH="$PATH:<dir>"` más dónde va esa línea (`~/.zshrc`,
`~/.bashrc`). **No se edita un rc concreto ni se dice que se editó**: deal-kit no
sabe qué shell corre y no nombra un archivo que no miró. Sigue sin tocar el PATH.

### Decisiones

| Decisión | Razón |
|---|---|
| `tui.PathHint(goos, dir)` exportada | La pantalla y `renderEngram` tienen que decir lo mismo en la misma máquina. Ya divergieron una vez con la advertencia de Windows (§11). Un solo helper hace imposible que contesten distinto. |
| `internal/cli` estrena su propio `hostGOOS` | `render.go` gateaba con `runtime.GOOS` directo y no se podía testear la otra plataforma. Ahora espeja `tui.hostGOOS` y hay un test por plataforma de los dos lados. |
| El bloque del PATH va **último** | Es lo único de la pantalla que el install no puede hacer por el usuario. Arriba quedan las advertencias, que no deben competir con él. |
| Se sacó la sección "Qué se instala" | Prosa estática que nunca cambia, con su propio título y dos líneas en blanco. Se fusionó en una oración del párrafo de arriba. |
| Se sacó "El plan lo instala primero, antes de tocar el plugin" | El orden de la lista de comandos ya lo muestra. |
| Se sacó la nota de "reemplaza el archivo que ya está ahí" | `downloadNote` ya lo dice en la línea del comando. Se dice una vez. |
| El destino ya en el PATH no pide nada | Si el binario aterriza en un directorio que la shell ya busca, no hay acción pendiente; inventar una entrena a saltear el bloque. |
| Sin `Download` queda una línea, no un bloque | `go install` escribe en `GOBIN` y un plan bloqueado no tiene directorio: no hay comando honesto que dar, pero la consecuencia vale una línea. |

### Riesgo conocido: `setx PATH "%PATH%;..."`

Es el comando corto y copiable, y es el que se entrega. Tiene dos defectos
conocidos de Windows que **no** son de deal-kit: `%PATH%` en `cmd.exe` es la
mezcla de PATH de sistema y de usuario, así que `setx` copia el de sistema
dentro del de usuario; y `setx` trunca a 1024 caracteres. La alternativa segura
es una línea de PowerShell con `[Environment]::SetEnvironmentVariable`, que a 46
columnas se corta por el medio y deja de ser copiable. Se eligió el comando
corto; si alguna vez molesta, la decisión vive en un solo lugar (`PathHint`).

### Tests

| Qué fija | Test |
|---|---|
| La instrucción falsa no vuelve (4 estados) | `tui.TestTheScreenNeverTellsTheUserToRunEngramSetup` |
| Lo mismo en la salida no interactiva | `cli.TestTheEngramOutputNeverTellsTheUserToRunEngramSetup` |
| El comando por plataforma, como unidad | `tui.TestPathHintIsTheCommandForItsPlatform` |
| El comando en la pantalla, por plataforma | `tui.TestThePathCommandIsTheOneForTheHostPlatform` (además exige que "agregarlo a mano" y "El plan lo instala primero" no vuelvan) |
| El comando en `renderEngram`, por plataforma | `cli.TestThePathCommandInTheOutputMatchesTheHostPlatform` |
| Un destino ya en el PATH no pide nada | `tui.TestAnAlreadyReachableDestinationAsksForNothing` |
| La pantalla que ve un Windows | golden `engram-binary-missing-windows` (`tui.TestViewEngramMissingBinaryOnWindows`) |

Los cinco primeros se verificaron al revés reintroduciendo la frase falsa, la
nota de "a mano" y un `goos` equivocado en `PathHint`: los seis fallan.

El golden de Windows sigue mostrando un `destino` POSIX: `hostGOOS` gobierna solo
el texto de la vista, mientras que `engram.PlanFor` deriva el directorio de
`runtime.GOOS`. Lo que fija el snapshot es la forma de la pantalla y qué frases
recibe un host Windows, no la ruta.

### Verificación

`gofmt -l .` limpio · `go vet ./...` limpio · `go test ./... -count=1` 12/12 ·
goldens regenerados (los cuatro de Engram cambiaron, el resto sin diff) ·
hermeticidad `HERMETIC`.

### Tag

`v*`: los cambios son de `tool/`.

---

## 16. `frontend/ux-review`: lente de revisión UX/UI para web y móvil

Skill nueva: ocho principios de UX aplicados al código de una pantalla, con
hallazgos anclados a `file:line`, un diff concreto y una de cuatro severidades.
Alcance cerrado en ocho principios — Hick, Miller, espacio en blanco, KISS,
minimalismo, Don't Make Me Think, divulgación progresiva y jerarquía visual.
**Accesibilidad queda explícitamente fuera de esta versión** (decisión del
usuario), y la skill lo dice para que nadie la reintroduzca por inercia.

| Archivo | Qué hace |
|---|---|
| `skills/frontend/ux-review/SKILL.md` | la skill (inglés, como el resto) |
| `kit.yaml` | artefacto `frontend/ux-review` + entrada en los perfiles `web` y `mobile` |
| `README.md` | fila de `skills/` corregida (decía "PR workflow", eliminada en `03af755`) y sección "The UX/UI review lens" |

### El prefijo del id es convención, no restricción

Verificado en `tool/internal/kit/manifest.go`: **nada relaciona el prefijo del id
con `applies_to`.** Las únicas validaciones son id único y no vacío, `src`
presente, `type` conocido, la regla de `dest` por tipo, que cada valor de
`applies_to` nombre un `project_type` declarado, y que cada perfil solo incluya
artefactos que soportan ese tipo. El prefijo se usa en dos lugares y ninguno
restringe nada: `InstallName()` lo aplana para el nombre de la skill instalada, y
`a.Group` cae al primer segmento del id **solo si** el artefacto no declara
`group`.

Por eso el id es `frontend/ux-review` con `applies_to: [web, mobile]`: el prefijo
describe lo que el artefacto cubre. `general/` habría mentido — hoy significa los
tres tipos —, y `web/` habría escondido que móvil también lo instala.

### Decisiones

| Decisión | Razón |
|---|---|
| `group: "Frontend"` | `buildGroups` (`internal/tui/tree.go:32`) arma grupos **solo con artefactos que no son `skill`**, así que el `group` de una skill es inerte en la TUI. Se declara igual para que `kit.yaml` se lea coherente, y "Web" sería falso para un artefacto que también instala móvil. |
| Sin `applies_to` en el frontmatter | La herramienta lo lee de `kit.yaml` (`manifest.go`) y nunca del archivo. Que 2 de 9 skills lo repitan es drift (§12). |
| Cuatro severidades, no aceptar/rechazar | Un umbral duro produce falsos positivos — un `DataTable` de 20 columnas es correcto, y un formulario plano de 200 líneas no mejora partido en tres pestañas — y un revisor que rechaza trabajo correcto deja de leerse. |
| Los números son gatillos, no veredictos | Está dicho en una sección propia y repetido en "What NOT to do": "más de ~7 elementos" obliga a **justificar**, no a rechazar. Un lector que obedece un número sin pensar es el modo de falla. |
| No repite otras skills | Tabla "Not in this review" que apunta a `general-conventions` (TypeScript estricto, Zod), `web-ui` (catálogo en vez de markup propio) y `web-architecture` / `mobile-architecture` (dónde va cada archivo). KISS queda acotado a complejidad de UI justamente para no pisar arquitectura. |

### De dónde salen las dos ideas prestadas

Se revisaron dos skills públicas: `wonjyou/design-audit` (solo imágenes, 5
estrellas, sin licencia) y `keysjoao/laws-of-ux-skills` (MIT, sobre código, 3
estrellas). **Ninguna es adoptable**: inmaduras y ajenas a este design system. Se
tomaron dos ideas: las cuatro severidades, y el hallazgo anclado a `file:line`
con diff en vez de prosa — que además es como reportan las lentes de revisión de
este repo.

### Verificación

`go test ./internal/kit/ -count=1` ok · `gofmt -l .` limpio · `go vet ./...`
limpio · `go test ./... -count=1` 12/12 · goldens de la TUI regenerados **sin
diff** (el `group` de una skill no llega a `buildGroups`).

Binario real (`/tmp/deal-kit-ux`, `--kit-dir` al working tree) contra proyectos
scratch fuera del repo:

- `crm-deal-web`: `init --yes --no-deps` instaló
  `.claude/skills/frontend-ux-review/SKILL.md`, **idéntico byte a byte** al
  fuente (`diff` vacío), registrado con hash en `deal-kit.lock`, y `status`
  reporta `frontend/ux-review  ok`.
- `crm-deal-mobile`: mismo resultado, `status` → `ok`.
- `crm-deal-orders-service`: `init --dry-run` **no** menciona el artefacto
  (`grep -c ux-review` → 0), que es lo que `applies_to: [web, mobile]` promete.

### Tag

`kit-v*` únicamente: no hay cambios bajo `tool/`.

---

## 16. El ui-kit ahora se compila (`ci: typecheck del ui-kit`)

### Lo que estaba pasando

Los 70 archivos `.ts`/`.tsx` de `ui-kit/` **nunca se habían compilado**. No había
`tsconfig.json` ni `package.json` en ningún lado del repo, y `.github/workflows/ci.yml`
solo corría los pasos de Go (`go vet`, `go test`, `go build`) más `shellcheck`. El único
lugar donde esos componentes veían un compilador era el proyecto que los instalaba, es
decir: después de publicar el tag.

### Qué se agregó

| Archivo | Para qué |
|---|---|
| `ui-kit/package.json` | `"private": true`, sin build ni publish. Un solo script: `typecheck` → `tsc --noEmit`. Las `devDependencies` se derivan de los bloques `npm:` de `kit.yaml`, más `react`, `react-dom`, `@types/*` y `typescript`. |
| `ui-kit/tsconfig.json` | `strict` (lo que exige la skill `general-conventions`), más `noUnusedLocals`, `noUnusedParameters` y `noFallthroughCasesInSwitch`. `noEmit`, `jsx: react-jsx`, y `paths` mapeando `@/*` a la raíz de `ui-kit/` para que resuelvan los imports internos. |
| `ui-kit/package-lock.json` | Versionado, para que CI corra `npm ci` y sea reproducible. |
| Job `ui-kit` en `ci.yml` | `actions/setup-node@v4` con caché de npm, `npm ci`, `npm run typecheck`. Mismo estilo que el job `tool`. |

`.gitignore` ya tenía `node_modules/` sin anclar, así que cubre `ui-kit/node_modules`
sin tocar nada.

### Los dos errores que aparecieron

Ambos de nivel lint, ninguno un bug de comportamiento:

- `components/ui/scroll-area.tsx:3` — `import * as React from "react"` sin usar
  (`TS6133`). Con `jsx: react-jsx` el import ya no hace falta. Se borró la línea.
- `components/data-table/DataTable.tsx:503` — `function SortableDataRow<T>` declaraba
  un genérico `T` que el cuerpo nunca usa (`TS6133`). Se borró el `<T>`. El único
  call site (línea 938) no lo pasaba, así que no cambia nada.

### `react` no está en ningún bloque `npm:` de `kit.yaml`

Es el único paquete que el ui-kit importa y el manifiesto no declara (39 imports).
**Se dejó así a propósito**: el `project_type` `web` ya es React + Vite, así que React
es la base del proyecto, no algo que instalar componente por componente. Declararlo en
los 57 bloques `npm:` sería ruido y no cambiaría lo que se instala. En `package.json`
sí está, porque `tsc` necesita sus tipos.

### Verificación

```
cd ui-kit && npm install          # 147 paquetes, 0 vulnerabilidades
npx tsc --noEmit                  # 2 errores → arreglados → limpio
cd tool && gofmt -l .             # sin salida
go vet ./... && go test ./... -count=1   # todo ok
go test ./internal/kit/ -count=1  # ok — kit.yaml no se tocó
```

### Tag

`kit-v*`: cambia `ui-kit/`. `ci.yml` no está bajo `tool/` y no dispara `v*`.

---

## 17. Defecto: la convergencia se reportaba como conflicto y congelaba el update

`classify()` en `tool/internal/plan/plan.go` chequeaba en este orden: no existe →
`Create`; no está en el lock → `Blocked`; **`current != recorded` → `Blocked`**;
`current == srcHash` → `Unchanged`; si no → `Overwrite`.

El chequeo del hash del lock corría **antes** que el de igualdad de contenido. Cuando
alguien arregla un archivo del kit en su proyecto y ese mismo arreglo después sube
upstream, el archivo en disco queda **byte a byte igual** al que el kit escribiría, pero
el lock sigue guardando el hash de la versión vieja. Resultado: `Blocked`, con el motivo
`editado localmente desde que deal-kit lo escribió`.

**Eso no es un conflicto, es convergencia.** No hay nada que sobreescribir y nada que
perder: los bytes en disco ya son los bytes que el kit quiere.

No es un caso de borde. `skills/web/ui/SKILL.md` le dice explícitamente al equipo que
arregle el archivo del kit localmente y suba el cambio upstream, así que este estado es
el **final normal** de ese flujo.

### Era irrecuperable

- No hay flag `--force` ni "adopt" (los flags están en `tool/cmd/deal-kit/main.go:50-57`).
- `Plan.Apply` se niega a correr mientras haya **cualquier** cosa bloqueada, así que un
  solo archivo convergido congelaba el update entero, incluidos artefactos que no tenían
  nada que ver.

Caso real que lo motivó: `C:\SoftwareDevelopment\frontend-crm` (lock en `kit-v0.8.0`)
tenía `src/shared/ui/scroll-area.tsx` y `src/shared/ui/data-table/DataTable.tsx`
bloqueados, ambos idénticos a `kit-v0.9.0` módulo CRLF y la reescritura de imports.

### El arreglo

`current == srcHash → Unchanged` pasa **arriba** de `current != recorded → Blocked`. La
igualdad de contenido le gana a la contabilidad del lock.

### El lock se autocura, con una condición

Cada acción no bloqueada se registra con `Hash: act.hash` (que es `srcHash`) en
`plan.go:109-111`, y `Apply` reescribe la entrada con ese hash. O sea que el hash viejo
se corrige solo — **pero solo si `Apply` llega a correr**. `internal/cli/cli.go:298-310`
corta antes cuando `len(p.Changes()) == 0`, así que un run donde lo único "distinto" es
el hash rancio imprime `ya está actualizado` y no reescribe el lock. Es inocuo: el plan
ya dice `Unchanged` y `status` dice `ok`, así que el estado es estable y no bloquea. En
el caso real —un update de kit que sí trae otros cambios— `Apply` corre y el hash queda
corregido. Verificado con el binario real, ver abajo.

### Alcance: el branch `!owned` no se tocó

Un archivo que deal-kit **nunca** escribió sigue siendo `Blocked` aunque su contenido
coincida. Es del proyecto, y esa política es deliberada (§4.2: eso es lo que resolvería
un `deal-kit adopt`, que sigue pendiente). `TestBlockedWhenAnUnmanagedFileIsInTheWay` lo
sigue fijando.

### `planRemovals` **no** tiene el mismo problema

`planRemovals` (`plan.go:158-190`) compara `current != old.Hash` y bloquea con
`reasonRemovedEdited`. Ahí **no hay con qué converger**: el artefacto dejó de producir
ese archivo, así que no existe un `srcHash` contra el cual comparar. La única referencia
posible es lo que deal-kit escribió la última vez, que es exactamente lo que ya compara.
No se cambió, y no hay reordenamiento análogo que hacer.

### Tests

| Test | Sin la corrección |
|---|---|
| `plan.TestAConvergedFileIsUnchangedEvenWhenTheLockIsStale` | `kind = "blocked" (editado localmente desde que deal-kit lo escribió), want unchanged` |
| `plan.TestApplyRewritesTheStaleHashOfAConvergedFile` | `1 archivo(s) requieren atención antes de aplicar` |

### Verificación

`gofmt -l .` sin salida · `go vet ./...` limpio · `go test ./... -count=1` 12/12 ·
goldens de la TUI regenerados **sin diff** (el cambio es de clasificación, no de
renderizado, y ningún golden fija un archivo convergido).

Binario real contra un proyecto scratch (`/tmp/conv-proj`, perfil `web`, `--kit-dir` al
working tree) con el hash de `src/shared/lib/utils.ts` corrompido a mano en el lock para
reproducir la convergencia:

```
deal-kit-old status → ui-kit/base  MODIFICADO  src/shared/lib/utils.ts
deal-kit     status → ui-kit/base  ok
```

Y con otro cambio en el mismo run (para que `Apply` corra), el lock volvió al hash
correcto solo.

### Tag

`v*` únicamente: el cambio vive entero bajo `tool/`.

---

## 18. `Co-Authored-By` en `general-conventions` (`feat/kit-conventions-coauthor`)

Regla nueva en `skills/general/conventions/SKILL.md`, **justo debajo** del bloque de
Conventional Commits (antes de "## Zod is the single source of truth"):

```
An AI agent ends its commits with a `Co-Authored-By` trailer, so the history shows
what wrote the change:

    Co-Authored-By: Claude <noreply@anthropic.com>
```

Genérica (no específica de Claude en el texto), un solo ejemplo.

### Convención agregada por el dueño, NO por el briefing del coordinador

La presentación de agosto 2026 (§8) no menciona atribución de commits. Esta regla la
decidió el dueño del repo a propósito. Es la excepción explícita a "solo lo que la
presentación afirma" (§5), documentada acá para que nadie la borre pensando que es drift.

### Test

`tool/internal/kit/repo_conventions_test.go` (nuevo, paquete `kit`, junto a
`repo_manifest_test.go`): lee el `SKILL.md` real y afirma que

- contiene el trailer `Co-Authored-By:` (con sus dos puntos), y
- ese trailer aparece **después** del heading `**Conventional Commits**` del cuerpo
  (no de la mención en el frontmatter `description:`), fijando el emplazamiento que
  pidió el dueño.

Cambio de producción que lo hace fallar: quitar la guía de `SKILL.md`. Verificado al
revés — sin la regla, `FAIL: ... does not carry the Co-Authored-By trailer rule for AI
commits`.

### Verificación

`gofmt -l .` limpio · `go vet ./...` limpio · `go test ./... -count=1` verde ·
goldens de la TUI **sin diff** (es un cambio de cuerpo de skill; la TUI renderiza
nombres de artefactos, no cuerpos).

Binario real (`/tmp/deal-kit`, `--kit-dir` al working tree) contra
`/tmp/coauthor-proj` (perfil `web`, `--type web --yes --no-deps`):
`.claude/skills/general-conventions/SKILL.md` **idéntico byte a byte** al fuente
(`diff` vacío), la regla presente en la copia instalada, `status` →
`general/conventions  ok`.

### Tag

`kit-v*` obligado: cambia `skills/`, que es lo que los proyectos pinean.

`v*` es discutible. Por la letra del decision gate ("anything under `tool/`" → `v*`)
el test nuevo `repo_conventions_test.go` lo dispara; por el propósito del namespace
("builds binaries") no, porque un archivo de test no cambia el binario que se publica.
**Decisión del dueño**: cortar solo `kit-v*`, o los dos por prolijidad del gate.

---

## 19. `repo_skills_test.go`: la prosa de las skills ahora se valida contra el código

`kit.yaml` está protegido por `repo_manifest_test.go` desde el principio. La prosa de
`skills/` no tenía **nada** equivalente, y derivó dos veces:

1. El catálogo original nombraba tres exports que no existen: `PortalContainer`,
   `Chart`, `Resizable` (§6).
2. `skills/web/ui/SKILL.md` afirmaba que el catálogo corre sobre Radix. Es falso: 26
   archivos importan `@base-ui/react`, 4 usan Radix como primitivo real y 4 solo le
   sacan el `Slot`. Corregido en `ce7764b`.

La primera Hard Rule de la skill `deal-kit-maintenance` ("nunca documentar un símbolo,
path o export sin greppearlo en el fuente") existe por esto, pero era una regla que solo
se cumplía si el que escribía se acordaba. Ahora hay un test.

### Qué hace

`tool/internal/kit/repo_skills_test.go` →
`TestSkillsOnlyNameRealKitExports`. Lee los `SKILL.md` **reales** del repo y falla
cuando uno nombra un componente que ningún fuente del `ui-kit` exporta.

La lista de exports **se deriva del código en cada corrida** — `export { ... }` y
`export function/const/type/interface/...` sobre `ui-kit/**/*.ts(x)`, salteando
`node_modules`. Hardcodearla recrearía exactamente el problema que el test evita.

Va en `internal/kit` por precedente, no por la tabla "New code goes in" de la skill: esa
tabla cubre código de producto (decide un sync → `plan`, lee/escribe el proyecto →
`cli`, renderiza → `tui`). Esto no es ninguna de las tres: es validación de contenido del
repo, y el único test de esa clase que ya existe —`repo_manifest_test.go`— vive acá, con
la misma forma de ubicar la raíz (`filepath.Join("..", "..", "..")`).

### Alcance deliberado: dos regiones estructuradas, no toda la prosa

Solo se extraen símbolos de:

1. las filas de una tabla cuyo header es exactamente `| Need | Use |` (el catálogo), y
2. la sección titulada `Critical composition rules`.

Y dentro de esas regiones, solo un code span que sea **entero** una palabra PascalCase
(`^[A-Z][A-Za-z0-9]*$`).

**El criterio es señal sobre ruido, no cobertura.** Los backticks de estas skills
envuelven muchísimo que no es un export: paths (`shared/ui/button.tsx`), clases CSS
(`bg-primary`, `size-10`), paquetes npm (`@base-ui/react`), props (`selectable?`),
comandos (`deal-kit add`), elementos HTML (`<select>`), tokens de Tailwind y fragmentos
de código. Un test que los marque se silencia o se borra en una semana. La regla estricta
los descarta a todos por construcción: ninguno es una sola palabra PascalCase.

Medido: **125 símbolos chequeados, 0 falsos positivos, allowlist vacía.** `nonKitSymbols`
existe declarada y vacía, con el comentario de que crecer más de un puñado de entradas
significa que la regla de extracción quedó demasiado ancha y hay que angostarla, no
rellenar la lista.

Las dos regiones se detectan por **estructura, no por nombre de archivo**. Por eso las
skills de móvil no producen fallas: `crm-deal-mobile` usa React Native Paper, esos
componentes no están en este repo, y ninguna skill de móvil tiene tabla `| Need | Use |`
ni sección de composition rules. Verificado: se recorren los 10 `SKILL.md` y solo
`web/ui` aporta símbolos.

### Qué NO agarra

Hay que ser honesto con esto, porque el test da una sensación de cobertura mayor que la
que tiene:

- **La clase de error de Radix (el defecto #2 de arriba) NO la agarraría.** "Está
  construido sobre Radix" es una afirmación en prosa sobre de qué librería depende un
  componente. `Accordion` existe y se exporta; el símbolo está bien y la afirmación
  está mal. Un test de símbolos no puede verlo.
- Nada fuera de las dos regiones: el catálogo de props del `DataTable`, los ejemplos
  `tsx`, la sección `What NOT to do`, y toda la prosa del resto de las skills.
- Los identificadores camelCase (`defineColumns`, `useIsMobile`, `cn`) y los que llevan
  guión bajo (`DEFAULT_DATA_TABLE_LABELS`): son exports reales, pero admitirlos abriría
  la puerta a las props y a los nombres de hooks inventados en cualquier ejemplo. Se
  eligió perder un error real antes que producir uno falso.
- Que un componente exista **no** prueba que la fila del catálogo lo describa bien.

Dos guardas contra el fallo silencioso, porque un extractor que deja de matchear haría
pasar el test encontrando cero: falla si parsea menos de 100 exports del `ui-kit`, y
falla si no extrae ni un símbolo de ningún `SKILL.md`.

### CI

**Ya corre, sin cablear nada.** El job `tool` de `.github/workflows/ci.yml` hace
`actions/checkout` del repo completo y corre `go test ./...` desde `tool/`; el
`working-directory` solo afecta a los `run`, no al checkout. Los paths relativos
(`../../../skills`, `../../../ui-kit`) resuelven igual que en local. No necesita
`ui-kit/node_modules` (que en CI solo existe en el otro job): el walk lo saltea.

### Verificación

`gofmt -l .` sin salida · `go vet ./...` limpio · `go test ./... -count=1` 12/12 ·
goldens de la TUI sin tocar (no hay cambio de renderizado).

Probado al revés, sembrando los dos errores históricos y corriendo el test:

```
skills/web/ui/SKILL.md:125: the component catalog table names "PortalContainer",
    which no ui-kit source exports.
skills/web/ui/SKILL.md:136: the Critical composition rules section names "Resizable",
    which no ui-kit source exports.
```

Cada mensaje nombra el archivo, la línea, la región, el símbolo, dónde se esperaba
encontrarlo (`ui-kit/components/ui/*.ts(x)`, …) y qué hacer si el nombre es legítimo.
La siembra se revirtió; `git diff` de `skills/` quedó vacío.

### Tag

`v*` únicamente: el cambio vive entero bajo `tool/`. No cambia contenido del kit.

## 19. Corrección: la regla de Co-Authored-By estaba invertida

El PR #32 (`feat(kit): require a Co-Authored-By trailer for AI commits`) mergeó a
`main` lo contrario de lo pedido. El pedido era que un agente **no** firme sus
commits con atribución de IA; lo que entró fue un párrafo en
`skills/general/conventions/SKILL.md` indicando que sí lo haga, más
`tool/internal/kit/repo_conventions_test.go` asertando su presencia. O sea que el
CI pasó a fallar si alguien borraba la regla equivocada: quedó blindada al revés.

La convención correcta ya estaba escrita en el `CLAUDE.md` global del dueño del
repo: *"Never add 'Co-Authored-By' or AI attribution to commits."*

| Qué se corrigió | Cómo |
|---|---|
| El párrafo del skill | Ahora prohíbe el trailer y cualquier otra atribución de IA, con el motivo: el historial registra quién es responsable del cambio, y eso siempre es una persona |
| El test | `TestTeamConventionsForbidAIAttribution` asierta la prohibición, su ubicación después de Conventional Commits, y que el trailer aparezca a lo sumo una vez — como contraejemplo marcado `← never` |

El comentario del test viejo se auto-justificaba llamando a la regla *"the
owner-added rule"* y *"a deliberate repo-owner addition"*, y citaba una sección
de este archivo que sobre ese tema nunca existió. Vale como recordatorio de la
primera Hard Rule del skill de mantenimiento: una afirmación sin fuente
verificada es una invención, también cuando la escribe un test.

**Causa raíz:** dos sesiones trabajando sobre el mismo working tree. Los archivos
de una aparecían bajo los pies de la otra. Para trabajo en paralelo, un
`git worktree` por sesión.

**Verificación:** el test falla si se revierte la prohibición
(`no longer forbids AI attribution in commits`) y pasa con ella. Suite completa
en verde.

**Tag:** `kit-v*` por el cambio en `skills/`, `v*` por el cambio en `tool/`.

## 20. Regresión del PR #31: un Ctrl+C se reportaba como presupuesto agotado

El PR #31 cambió `budgetErr` para que la rama `prev == nil` —que antes devolvía
el error tal cual— sintetice siempre *"no quedó presupuesto … no terminó a
tiempo"*. En el mismo PR se agregó el guard de `DeadlineExceeded`, pero **solo en
el sitio mid-step** (`engram.go:741`). El chequeo del tope del loop
(`engram.go:724`) quedó envolviendo sin condición.

`internal/cli/interactive.go:250` arma un solo contexto con `signal.NotifyContext`
(SIGINT/SIGTERM) **y** `context.WithTimeout`. O sea que un Ctrl+C entre pasos
llegaba al chequeo del tope del loop y el usuario leía que un paso *"no terminó a
tiempo"* cuando ese paso nunca había arrancado. El comentario del sitio hermano
ya decía textualmente lo contrario: *"a Ctrl+C cancels the same shared context
and must not be reported as an exhausted budget"*.

`TestACancelledContextRunsNothing` no lo cazó porque solo asertaba
`errors.Is(out.Err, context.Canceled)`, y eso sigue siendo cierto: `budgetErr`
envuelve con `%w`. El mensaje mentía, el unwrap no.

| Qué se corrigió | Cómo |
|---|---|
| `engram.go:724` | mismo guard que el sitio mid-step: solo un deadline es presupuesto agotado |
| `TestAnExhaustedBudgetNamesTheStepThatSpentIt` | simulaba el deadline con un `cancel()`. Ahora usa `context.WithTimeout` real y el nuevo helper `starveAfterMutation`, que espera `ctx.Done()` en vez de competir con el timer — determinista. Su aserción final ahora espera `DeadlineExceeded`, que es lo que un presupuesto agotado realmente es |
| Test nuevo | `TestACancellationBetweenStepsIsNotReportedAsABudgetFailure`, hermano del que ya cubría el sitio mid-step |

**Lección:** un test que aserta solo el `errors.Is` no protege el mensaje, y acá
el mensaje era el producto. Y simular un deadline con un `cancel()` hace que el
test pase por un camino que ningún usuario recorre.

**Verificación:** el test nuevo falla sin el fix y pasa con él; suite completa en
verde.

**Tag:** `v*`.

## 21. `general/smoke-run`: la entrega tiene que arrancar (`feat/kit-smoke-run`)

En la demo el agente terminó el trabajo, corrió los tests y todo pasó. Cuando el
usuario intentó levantar la app, no arrancó. El kit no tenía **nada** que cubriera
eso: `general-tdd` cierra en verde y ahí se terminaba la evidencia.

La clase de fallo es exactamente la que ningún unit test ve, porque vive entre las
piezas: un provider que el módulo nunca registró, una env var que ningún test lee,
un import circular, el schema Zod de `config/` que solo valida al bootear, un alias
que el bundler resuelve y el test runner shimea. Suite verde, `main` muerto en la
primera línea.

### Qué se agregó

| Archivo | Qué |
|---|---|
| `skills/general/smoke-run/SKILL.md` | la skill, reestructurada al contrato de `skill-creator`: Activation Contract, Hard Rules, Decision Gates, Execution Steps, Output Contract, References |
| `kit.yaml` | artefacto `general/smoke-run` + entrada en los tres `profiles`, después de `general/tdd` |
| `skills/general/tdd/SKILL.md` | una línea en `What NOT to do` que apunta acá. No repite la skill: la nombra |
| `README.md` | sección "The boot gate" + fila de la tabla `skills/` |

### El gate

1. Levantar las **dependencias de contenedor** que el repo declara y esperar a que
estén *healthy* · 2. buildear · 3. levantar **en background**, en un puerto libre,
con el log a archivo · 4. **pollear el puerto** hasta que responda o expire, y probar
una vez por HTTP · 5. matar el árbol de procesos, **verificar que el puerto quedó libre**
y frenar **solo** los servicios que arrancó el propio gate — el paso 5 corre también
cuando fallan el 2, el 3 o el 4.

### Decisiones, con su razón

| Decisión | Razón |
|---|---|
| Skill sola, sin config always-on | Se ofreció anclarlo en un `config/` con `ensure_line` (como la persona) porque una skill carga solo si el modelo juzga que la `description` aplica — el mismo mecanismo que falló en la demo. El dueño del kit eligió skill sola. **Riesgo asumido y conocido: si el gate se vuelve a saltear, esta es la palanca que queda por mover.** La `description` se escribió agresiva a propósito ("antes de decir que algo está done, finished, working, ready to review or ready to merge") |
| La skill no hardcodea ningún comando | Los tres repos **no existen todavía** (§8). Afirmar que existe `start:dev` sería inventar una convención. La fuente de verdad es `package.json` → `scripts`, y la línea de ready se lee del log, no se adivina |
| Puerto libre, nunca el default del proyecto | El server del propio dev suele estar en el default: el bind falla y el fallo se le achaca al cambio. Y un check que se muere a la mitad deja el puerto del equipo secuestrado |
| `--strictPort` en Vite | Sin él Vite se corre calladito al siguiente puerto libre y el probe pega contra nada |
| `taskkill /T` en Windows | Matar `npm.cmd` **no** mata el `node` que tiene el puerto. Por eso el paso 4 verifica el puerto en vez de confiar en el exit code del kill |
| Móvil no tiene probe | Sin device no hay puerto que consultar. El bundle (`expo export`) sí caza la clase de fallo que importa. Si algo necesita emulador y no lo había, se reporta sin verificar, no se redondea a verificado |

### Reestructura al contrato de `skill-creator`

La skill se reescribió con las secciones en el orden del contrato (Activation
Contract, Hard Rules, Decision Gates, Execution Steps, Output Contract,
References) y la sección "What NOT to do" se plegó dentro de Hard Rules. Esa pasada
también creó `assets/` y `references/`, que la pasada siguiente eliminó — ver abajo.

La `description` bajó de **454 a 227 caracteres**, con las trigger words primero
(`done, finished, working, ready to review, ready to merge, before opening a PR`).
**Razón:** una `description` inflada diluye justamente las palabras por las que la
skill se activa, y la activación es el mecanismo que falló en la demo. El cuerpo
pasó de ~1593 a ~1030 tokens estimados; no bajó a los ~700 del contrato porque el
gate sumó un paso entero (Docker) y tres reglas nuevas de evidencia, y amputar
reglas para llegar al número era peor que pasarse. Referencia interna:
`general-tdd` está en ~890.

**El frontmatter NO sigue a `skill-creator`.** Ninguna de las 11 skills del kit
lleva `license` ni `metadata`, y `name` tiene que seguir siendo
`general-smoke-run` porque `tool/internal/kit` (`CheckFrontmatterName`) lo valida
contra el nombre de instalación de `kit.yaml`. La convención del kit gana sobre el
contrato global, a propósito.

### Paso nuevo: dependencias de contenedor

El gate ahora levanta lo que el repo declara (`docker-compose.yml`,
`compose.yaml`, `docker-compose.*.yml`) **antes** de buildear y bootear. Misma
disciplina que ya aplicaba a `package.json` → `scripts`: se lee lo que hay, no se
inventa ni el archivo ni los nombres de servicio.

| Decisión | Razón |
|---|---|
| Esperar **healthy**, no "started" (`docker compose up -d --wait`) | Un contenedor arriba pero que todavía no acepta conexiones mata la app en el boot, y el fallo se le achaca al cambio |
| Teardown **solo de lo que arrancó el gate** | Espeja la disciplina del puerto. Si el stack ya estaba arriba, se deja arriba: matarle la base de datos al dev es peor que el problema que este gate resuelve. Se chequea primero, se registra lo que se arrancó, se frena solo eso |
| Docker ausente o stack que no levanta | Boot **UNVERIFIED** nombrando la dependencia que faltó. Nunca se redondea a verificado |

### Incidente: un modelo débil confirmó un boot que nunca corrió

Ayer un modelo clase Sonnet corrió el espíritu de este gate y reportó "funciona"
**sin haber verificado que la app arrancara**. La prosa no lo frenó. La mitigación
es estructural, no retórica: el Output Contract exige una línea con campos que son
imposibles de producir sin haber corrido los comandos — el comando exacto, la línea
de ready **citada textual del log**, el status code HTTP del probe, los servicios de
compose arrancados (o `none`) y la confirmación de que el puerto quedó libre y el
stack se bajó.

Dos Hard Rules lo cierran: **cualquier campo faltante es UNVERIFIED, no done**
(ausencia de evidencia observada nunca es un pase), y **está prohibido reportar un
status code, una línea de ready o un estado de salud que no se haya leído de la
salida real**.

### Verificación

`gofmt -l .` en la raíz del repo: sin salida.

`go vet ./...` y `go test ./... -count=1` **tienen que correr dentro de `tool/`**:
la raíz del repo no es un módulo Go y ahí fallan con
`directory prefix . does not contain main module`. Dentro de `tool/`: vet limpio,
**11 paquetes ok** (`internal/execenv` no tiene archivos de test). La línea previa
de esta sección decía `go test ./... -count=1` "12/12" desde la raíz: era
incorrecta en las dos cosas.

Binario real contra los tres tipos (`--dry-run --offline --kit-dir`): los tres
resuelven `SKILL.md` bajo `.claude/skills/general-smoke-run/`. El CLI copia
subdirectorios de skill recursivamente (`tool/internal/plan/plan.go`, `filePairs`
usa `filepath.WalkDir`), así que `assets/` y `references/` se instalan solos.

**Tag:** `kit-v*` — el cambio es contenido del kit, no toca `tool/`.

### Pasada de simplificación (la que dejó la skill como está)

La versión reestructurada se había ido a ~1030 tokens con once Hard Rules. El dueño
del kit cortó: *"nos estamos complicando mucho, es simplemente definir que al
terminar algo grande pruebe él mismo que está funcionando todo para que lo solucione
en la misma iteración"*. Se recortó a eso.

| Cambio | Razón |
|---|---|
| Disparador por **camino de arranque**, no por tamaño | "Cuando sea gigante" es un criterio que el modelo tiene que adivinar. La demo no falló por grande: falló porque el cambio tocó el boot. Una línea en un `@Module` mata el arranque; quinientas dentro del cuerpo de un service no. El Activation Contract ahora lista la clase: registro de módulos/DI, env vars y schema de config, dependencias, entrypoint, config de build y aliases, migraciones y compose |
| **Polling al puerto** en vez de grep de la ready line | La ready line era un string que había que adivinar: si se erraba, el wait caía al timeout y el probe pegaba contra un server que todavía arrancaba. El polling no adivina nada. La ready line se sigue **citando** del log como evidencia en el reporte, pero ya no se **espera** por ella |
| Once Hard Rules a siete | Se plegaron las redundantes |
| Se descartó shippear un `boot-gate.mjs` ejecutable | Se propuso un script Node sin dependencias (detección + overrides + polling) porque el exit code saca al modelo de la decisión. Se descartó: los tres repos objetivo todavía no existen (§8), un ejecutable es código del kit y necesita fixtures y tests de detección, y si el gate corre poco el mantenimiento no se amortiza. Se conservó la única idea gratis del script: el polling |

**Conflicto de puertos con el entorno de desarrollo del equipo: ya resuelto por
diseño.** La app arranca en puerto libre, nunca el default, y compose levanta solo
el delta y frena solo eso. Lo que **sí** puede chocar y el puerto libre no cubre es
el **build**: `npm run build` escribe `dist/`, y con un `nest start --watch`
corriendo los dos escriben el mismo directorio. Hipótesis sobre stacks estándar, sin
verificar contra los repos reales — pero es el riesgo a mirar, no los puertos.

### Dos defectos corregidos en las recetas

Ambos scripts tenían el teardown como bloque final, pero `npm run build || exit 1`
sale antes: **un build fallido dejaba la stack de compose levantada**, contradiciendo
la propia regla de la skill de que el paso 5 corre siempre. Corregido con
`trap teardown EXIT` en POSIX y `try/finally` en PowerShell. `sh -n` valida la
sintaxis del POSIX; el PowerShell no se ejecutó (no hay Windows acá).

Las recetas siguen siendo plantillas con placeholders, no scripts corridos: no hay
app Node en este repo contra la que ejecutarlas. Su comportamiento en runtime está
**sin verificar por construcción**, igual que las recetas que reemplazaron.

### Colapso a un solo archivo (estado final)

`assets/` y `references/` se **eliminaron**. Razón del dueño del kit: *"veo que
dentro de smoke run hay assets y references, muy cargado todo"*.

Y la evidencia lo respalda: de las 11 skills del kit, **9 son un solo `SKILL.md`**.
La única otra con material de apoyo es `general/tdd`, y es **un archivo hermano
plano** (`writing-good-tests.md`), no subdirectorios. `smoke-run` con dos carpetas
era el único outlier de estructura del repo.

Lo que se perdió al borrar, dicho explícitamente: las recetas ejecutables POSIX y
PowerShell, y `rationale.md` con el fundamento largo de cada regla. Las reglas
sobreviven en Hard Rules; el *por qué* extendido de cada una, no. Si vuelve a hacer
falta, el patrón correcto para este repo es un archivo hermano plano al lado del
`SKILL.md`, como hace `tdd` — no una carpeta.

El Activation Contract también se angostó. Ya no dispara en "todo cambio
sustancial" sino en **cambios que el equipo no vería romperse en su propia
pantalla**: inicialización o scaffold de un proyecto, wiring de módulos y DI, env
vars y schema de config, dependencias, entrypoint, config de build y aliases,
migraciones, compose. *"En caso de ser cosas chicas eso lo podemos ver nosotros en
tiempo real, no es problema."*

**Estado final:** `SKILL.md` único, `description` en 217 caracteres, cuerpo en ~738
tokens estimados (`general-tdd` está en ~890).
