# deal-dev-kit — estado y pendientes

Documento de traspaso. No versionado (está en `.gitignore`).
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
