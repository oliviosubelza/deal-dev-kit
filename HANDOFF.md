# deal-dev-kit — estado y pendientes

Documento de traspaso. No versionado (está en `.gitignore`).
Última actualización: 2026-09-03.

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

---

## 2. URGENTE: `kit-v0.2.0` está publicado con 6 skills en TODO

Se tageó antes de commitear. Hoy el tag contiene:

```
ok     web/ui (287 líneas)
TODO   backend/connections, backend/architecture, web/architecture,
TODO   mobile/architecture, mobile/offline, general/conventions
```

Las 7 están escritas y con tests verdes en el working tree. Falta:

```bash
git add -A && git commit -m "feat(skills): write the seven skills from the architecture briefing"
git push
git tag -d kit-v0.2.0 && git push origin :kit-v0.2.0
git tag kit-v0.2.0 && git push origin kit-v0.2.0
```

Retagear es seguro: nadie pineó `kit-v0.2.0` todavía.

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
| `plan` | 90.7% |
| `lockfile` | 88.9% |
| `doctor` | 88.2% |
| `kit` | 85.0% |
| `tui` | 78.6% |
| `selfupdate` | 73.2% |
| `pm` | 70.3% |
| **`cli`** | **3.6%** ← deuda |

---

## 4. Pendientes

### 4.1 Deuda de tests: `internal/cli` en 3.6%

`new.go`, `selfupdate.go` y buena parte de `cli.go` no tienen tests. Es justo el código
que **ejecuta comandos externos y escribe en directorios del usuario**. Solo probado a
mano. Es lo que atacaría primero.

Lo que sí tiene tests: `ProjectRoot` (9 casos, en `cli_test.go`).

### 4.2 `deal-kit adopt`

Un proyecto que pierde el `deal-kit.lock`, o que ya tiene componentes copiados a mano
del `ui-kit` viejo, no tiene salida: el CLI ve archivos que no escribió y se niega a
tocarlos (correcto), pero no hay forma de decirle "estos son tuyos, registralos".

Debería: comparar el archivo local con el del kit, y si son idénticos, registrarlo en el
lockfile sin reescribir nada. Escenario garantizado cuando migren proyectos existentes.

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
"Seguridad de la Arquitectura"— que **no están en la máquina**. Se verificaron los 25
PDFs de Downloads, los 5 `.docx` y la carpeta `documentation/`.
