// Command deal-kit installs and updates shared development assets (skills,
// conventions, UI components) from the deal-dev-kit repository into a project.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/cli"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
)

// version is injected at build time via -ldflags.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, cli.ErrAborted) {
			fmt.Fprintln(os.Stderr, "cancelado")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if help, showVersion := isHelpOrVersion(args); help || showVersion {
		if help {
			usage()
		} else {
			fmt.Println(version)
		}
		return nil
	}

	cmd, env, fs, flags, err := parseArgs(args)
	if err != nil {
		return err
	}
	return cmd.run(env, fs, flags)
}

// parseArgs resolves the subcommand and parses its flags into an Env, without
// running anything. Kept separate from run so the flag-to-Env wiring — the
// one place a flag either reaches its Env field or silently does not, e.g.
// --dry-run failing to reach env.DryRun and letting a "preview" turn into a
// real write — is testable without executing a real command.
func parseArgs(args []string) (command, cli.Env, *flag.FlagSet, *commandFlags, error) {
	// No subcommand opens the interactive browser.
	name, rest := extractCommand(args)
	cmd, ok := lookup(name)
	if !ok {
		usage()
		return command{}, cli.Env{}, nil, nil, fmt.Errorf("comando desconocido %q", name)
	}

	fs := flag.NewFlagSet(invoked()+" "+name, flag.ContinueOnError)
	var (
		kitDir  = fs.String("kit-dir", os.Getenv("DEAL_KIT_DIR"), "usar un checkout local del kit en lugar de descargarlo")
		repo    = fs.String("repo", envOr("DEAL_KIT_REPO", kit.DefaultRepo), "repositorio del kit desde donde descargar")
		ref     = fs.String("ref", os.Getenv("DEAL_KIT_REF"), "tag, branch o SHA del kit (por defecto: el tag kit-v* más nuevo)")
		offline = fs.Bool("offline", false, "usar el kit en caché sin contactar el remoto")
		yes     = fs.Bool("yes", false, "aplicar sin confirmación")
		dryRun  = fs.Bool("dry-run", false, "imprimir el plan y detenerse")
		noDeps  = fs.Bool("no-deps", false, "no ejecutar el package manager")
		here    = fs.Bool("here", false, "usar el directorio actual como raíz del proyecto")

		flags commandFlags
	)
	fs.StringVar(&flags.typeOverride, "type", "", "forzar el tipo de proyecto en lugar de detectarlo")
	fs.BoolVar(&flags.check, "check", false, "self-update: informar la última versión sin instalarla")

	if err := fs.Parse(permute(fs, rest)); err != nil {
		return command{}, cli.Env{}, nil, nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return command{}, cli.Env{}, nil, nil, err
	}

	env := cli.Env{
		Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin,
		Cwd: cwd, Version: version,
		KitDir: *kitDir, Repo: *repo, Ref: *ref, Offline: *offline,
		ReleaseRepo: os.Getenv("DEAL_KIT_RELEASE_REPO"),
		AssumeYes:   *yes, DryRun: *dryRun, NoDeps: *noDeps, Here: *here,
	}
	return cmd, env, fs, &flags, nil
}

// envOr returns the environment variable, or a fallback when it is unset.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// invoked is the name the user actually typed, taken from argv[0] rather than
// hardcoded. The published binary is `deal`, but a developer building from
// source gets `deal-kit` from the package directory, and someone may rename it
// again; usage that names a command the reader cannot run is worse than no
// usage. Falls back to the published name when argv[0] is unusable.
func invoked() string {
	if len(os.Args) == 0 || os.Args[0] == "" {
		return "deal"
	}
	// Cut on both separators rather than filepath.Base: that one only knows
	// the separator of the platform it was built for, and an argv[0] carrying
	// the other one — a Windows path seen through WSL interop, say — would
	// come back as the whole path instead of the command name.
	name := os.Args[0]
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return "deal"
	}
	return strings.TrimSuffix(name, ".exe")
}

func usage() {
	fmt.Fprintf(os.Stderr, "%s %s\n\nUso:\n  %s <comando> [opciones]\n\nComandos:\n", invoked(), version, invoked())
	for _, c := range visibleCommands() {
		label := c.name
		if c.name == "browse" {
			label = "(none)"
		}
		if c.name == "add" {
			label = "add <id>..."
		}
		if c.name == "new" {
			label = "new <dir>"
		}
		fmt.Fprintf(os.Stderr, "  %-13s %s\n", label, c.summary)
	}
	fmt.Fprint(os.Stderr, `
Opciones:
  --repo       Repositorio del kit (o definir DEAL_KIT_REPO)
  --ref        Tag, branch o SHA del kit (o definir DEAL_KIT_REF)
  --offline    Usar el kit en caché sin contactar el remoto
  --kit-dir    Usar un checkout local del kit en lugar de descargarlo (o DEAL_KIT_DIR)
  --type       Forzar el tipo de proyecto en lugar de detectarlo
  --here       Usar el directorio actual como raíz del proyecto
  --dry-run    Imprimir el plan sin escribir nada
  --yes        Aplicar sin confirmación
  --no-deps    Omitir la instalación de dependencias
  --check      Con self-update, sólo informar qué hay disponible
`)
}
