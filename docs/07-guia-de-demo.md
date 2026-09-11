# Guía de demo

**En resumen:** esta página es para quien presenta. Trae el mensaje central,
los argumentos por público, un guion de ~10-15 minutos con comandos exactos
—verificados contra el binario real salvo donde se marca lo contrario— y las
preguntas más probables con respuesta corta.

## Mensaje central, en tres frases

1. CRM DEAL son tres repos distintos que necesitan las mismas convenciones; sin
   una herramienta que las sincronice, divergen en semanas.
2. `deal` instala y mantiene ese contenido con un comando, detecta cuándo algo
   se editó a mano y nunca lo pisa sin avisar.
3. El mismo contenido que instala es lo que Claude Code lee para trabajar
   dentro de las convenciones del equipo — no es solo una plantilla de
   archivos, es el contexto del agente.

## Argumentos de valor por público

### Para developers

| Beneficio | Cómo se ve |
|---|---|
| Un comando deja el repo con convenciones, skills y componentes | `deal init` o `deal install`, sin copiar nada a mano |
| El agente ya sabe dónde va cada archivo | Las skills de arquitectura (`backend-architecture`, `web-architecture`, `mobile-architecture`) están instaladas y cargan solo cuando la tarea las necesita |
| Un componente UI no diverge entre proyectos | Se instala desde una única fuente; si se arregla algo localmente, sube upstream y el próximo `update` lo trae a todos |
| Drift detectado, no silencioso | `deal status` marca cualquier archivo editado a mano antes de que un `update` lo pise |

### Para coordinación técnica / leads

| Beneficio | Cómo se ve |
|---|---|
| Consistencia entre repos | Las mismas convenciones (TDD, seguridad, persistencia, conventional commits + Jira) en los tres tipos de proyecto, sin depender de que cada dev las recuerde |
| Onboarding | Un dev nuevo corre un comando y tiene el mismo contexto que el resto del equipo, incluido lo que el agente de IA sabe |
| Gobernanza versionada | El contenido del kit se tagea (`kit-v*`); cada proyecto fija una versión en `deal-kit.lock` y avanza cuando el equipo decide, no automáticamente |
| Trazabilidad por lockfile | `deal-kit.lock` es auditable: qué artefacto, qué archivo, qué hash — no hay "no sé de dónde salió este componente" |
| Seguridad codificada, no solo documentada | La skill `general-security` y el agente `backend-review-security` bajan la revisión de arquitectura de seguridad a reglas que el agente aplica y audita, no a un PDF que nadie relee |

## Guion de demo (~10-15 minutos)

Usar un directorio de scratch, **no** un repo real. La variable `DEAL_KIT_DIR`
apunta al checkout local del kit y equivale a pasar `--kit-dir` en cada
comando: así la demo no depende de la red ni de un tag ya publicado. El costo
es que `status` muestra el kit como `(sin fijar)`; sin la variable, `deal` baja
el último `kit-v*` de GitHub y lo fija en `deal-kit.lock`.

### 1. Preparar el proyecto (1 min)

```sh
# fish
set -x DEAL_KIT_DIR <ruta-al-checkout-de-deal-dev-kit>
# bash / zsh
export DEAL_KIT_DIR=<ruta-al-checkout-de-deal-dev-kit>

mkdir -p /tmp/demo-crm-deal-web && cd /tmp/demo-crm-deal-web
git init -q
```

### 2. Mostrar el plan sin aplicar nada (2 min)

```sh
deal init --type web --dry-run
```

Verificado con el binario real: imprime proyecto, tipo detectado, raíces, el
perfil `web`, y un plan de 12 artefactos / 15 archivos, terminando en
`--dry-run: no se escribió nada`. Buen momento para señalar: "esto no tocó el
disco todavía".

### 3. Aplicar (2 min)

```sh
deal init --type web --yes --no-deps
```

Verificado con el binario real: escribe los 15 archivos, imprime un árbol
resumido por directorio y termina en `listo: 15 archivo(s)`. `--no-deps` evita
depender de la red para instalar `clsx`/`tailwind-merge` durante la demo; sin
él, instala con el package manager detectado.

### 4. Mostrar el estado (1 min)

```sh
deal status```

Verificado: los 12 artefactos en `ok`.

### 4b. Instalar todo lo que aplica (1 min)

```sh
deal install --yes --no-deps
deal install --yes --no-deps     # segunda vez
```

Verificado con el binario real: sobre el proyecto ya inicializado, la primera
corrida agrega 67 archivos, casi todos componentes de UI en `src/shared/ui`, y
el proyecto queda con 71 artefactos y 82 archivos. La segunda responde
`ya está actualizado` sin escribir nada. Es el momento de mostrar que los
comandos son convergentes: repetirlos es seguro. Requiere `deal` v0.12.0 o
posterior, que es la primera versión con `install`.

### 5. Simular un edit local y mostrar que el kit lo detecta (3 min)

```sh
echo "// nota local del equipo" >> .claude/persona.md
deal status```

Verificado: `general/persona` pasa a `MODIFICADO .claude/persona.md`, el resto
sigue en `ok`. Después:

```sh
deal update --dry-run
```

Verificado: el plan lista `.claude/persona.md` bajo "requiere atención" con el
motivo `editado localmente desde que deal-kit lo escribió`, y el comando
termina con código de salida distinto de cero y el mensaje
`deal-kit no sobrescribe estos archivos. Llevar el cambio al kit, o revertir el
archivo localmente, y volver a ejecutar.` — sin escribir nada. Es el momento
central de la demo: el kit nunca destruye un cambio local en silencio.

### 6. Mostrar el navegador interactivo (2 min) — *(verificar antes de la demo)*

```sh
cd /tmp/demo-crm-deal-web
deal```

El navegador necesita una terminal real, así que conviene ensayarlo antes.
**Fuera** de una terminal el comando falla con el mensaje
`` no es una terminal: usar `deal-kit init`, `add` o `status` con --yes para uso no interactivo ``,
lo que confirma que dentro de una terminal se abre la rama interactiva. Recorrer: Menú →
Componentes de UI (mostrar que están agrupados y plegados) → Estado del
proyecto → salir con `q` sin aplicar nada.

### 7. Mostrar una skill y cómo la usa Claude Code (2 min)

```sh
cat .claude/skills/general-conventions/SKILL.md | head -20
```

Explicar: esto no es documentación pasiva — Claude Code lo carga
automáticamente cuando la tarea matchea la `description` del frontmatter, sin
que nadie tenga que pegarlo en el prompt.

### 8. Opcional: pantalla de Engram (1-2 min) — *(verificar antes de la demo)*

Desde el navegador interactivo, entrar a "Engram para Claude Code" y mostrar
el estado detectado (instalado / no instalado) sin confirmar nada con `y`. No ejecutar un install real sin avisar: escribe
en la configuración global (`~/.claude`) de la máquina donde se corra. Ver [`06-engram.md`](06-engram.md).

## Preguntas probables

| Pregunta | Respuesta corta |
|---|---|
| ¿Por qué no un paquete npm? | GitHub Packages exige un token por dev y por CI; un registro privado se paga. Copiar el fuente también evita que Tailwind v4 no escanee `node_modules`. |
| ¿Qué pasa si edito un componente? | El kit lo detecta (`status` lo marca) y se niega a pisarlo. El flujo esperado es subir el arreglo al kit; el próximo `update` lo trae a todos los proyectos. |
| ¿Y si pierdo el lockfile? | Hoy no hay salida automática: el CLI ve archivos que no escribió y se niega a tocarlos. Es la funcionalidad `deal adopt`, que está **pendiente** (no implementada). |
| ¿Funciona en Windows? | Sí, con instalador propio (`install.ps1`) y binario nativo. `deal-kit new` en Windows y el propio `install.ps1` solo fueron verificados parcialmente — ver estado honesto abajo. |
| ¿Cómo se actualiza? | `deal update` mueve el pin del contenido del kit; `deal self-update` reemplaza el binario del CLI. Son independientes — ver [`05-versionado-y-distribucion.md`](05-versionado-y-distribucion.md). |
| ¿Esto reemplaza la documentación de arquitectura? | No. Documenta lo que el coordinador y los leads ya definieron; donde algo no está definido (alias de imports, dónde van los tests), el kit no lo inventa. |

## Estado honesto

Verificado contra `HANDOFF.md` §4 (pendientes actuales, no resueltos):

| Ítem | Estado |
|---|---|
| `deal adopt` (registrar archivos ya existentes sin reescribirlos) | Pendiente — no implementado |
| Cobertura de tests de `internal/cli` | 37.9%, la más baja del proyecto; `new.go` y `selfupdate.go` siguen sin tests automatizados |
| `doctor` usa `ForWeb()` para los tres tipos de proyecto | Correcto hoy (los tres necesitan Node), pero mentirá cuando mobile pida `eas` o backend pida `docker` — falta `ForBackend()`/`ForMobile()` |
| Compartir schemas Zod entre web y mobile | Sin resolver: hoy se copian a mano fuera del kit; es candidato a artefacto futuro |
| `deal scaffold module/feature` | Propuesto y pospuesto a propósito — no se generan árboles de carpetas vacías |
| `deal lint` | Propuesto y pospuesto a propósito — sin chequeo por porcentaje, solo reglas falsables una por una |
| `deal-kit new` en Windows | Solo probado en WSL |
| `install.ps1` | Verificado por el dueño del proyecto; no reproducido en esta sesión (sin PowerShell disponible) |

Lo que **sí** está publicado y probado de punta a punta contra el binario
real, según `HANDOFF.md` §3 y verificado de nuevo para esta guía: los
instaladores Linux/macOS/Windows con verificación SHA-256, `self-update`,
fetch por git con caché, y los 61 componentes con resolución transitiva y
reescritura de imports.
