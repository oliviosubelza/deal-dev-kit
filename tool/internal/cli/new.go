package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/doctor"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
)

// generator is the official command that scaffolds one project type.
//
// deal-kit runs these rather than reimplementing them: the org standard fixes
// one stack per project type, so there is no choice to expose, and owning a
// generator's own UX and breakage buys nothing.
type generator struct {
	// Args is the command line, with {dir} replaced by the target directory.
	// Empty means the project type has no generator here at all, and the user
	// is told what to run instead of being handed a half-automated wizard.
	Args   []string
	Manual string
}

// generatorFor returns the official generator for a project type.
//
// All three are commands deal-kit can run. An earlier version printed the
// NestJS and Expo ones for the user to copy, on the belief that neither had a
// non-interactive form; both scaffold from a single command line.
//
// What none of the three is, measured on a pty on node v20.20.2 / pnpm
// 10.30.2, is silent: create-vite asks which linter, @nestjs/cli asks about
// @nestjs/observe, create-expo-app asks for an SDK version. With stdin closed
// each takes its default and exits 0, which is why they read as unattended in
// a pipe. New hands the generator the real stdin, so on a terminal the user
// answers the generator's own questions — its UX, not one reimplemented here.
//
// Every one of them skips its own dependency install. The kit install runs
// right after, and when it has npm dependencies to add `pnpm add` resolves the
// whole package.json in the same pass, so installing here first only pays for
// a full resolve twice — before the kit install can report a problem, and
// before the user has seen what deal-kit is about to write. create-vite never
// installs anyway, so skipping is also what makes the three types match.
func generatorFor(pt kit.ProjectType, pm string) generator {
	switch pt {
	case kit.Web:
		return generator{Args: []string{pm, "create", "vite@latest", "{dir}", "--template", "react-ts"}}
	case kit.Backend:
		// --skip-git as well: nest new runs git init, which would make the new
		// directory a repository the user did not ask for.
		return generator{Args: []string{pm, "dlx", "@nestjs/cli", "new", "{dir}",
			"--package-manager", pm, "--skip-install", "--skip-git"}}
	case kit.Mobile:
		return generator{Args: []string{pm, "create", "expo-app", "{dir}",
			"--template", "blank-typescript", "--no-install"}}
	}
	return generator{}
}

// New scaffolds a project with its official generator, then installs the kit
// into it.
func New(e Env, dir, typeOverride string) error {
	if dir == "" {
		return errors.New("indicar el directorio a crear: deal-kit new crm-deal-web")
	}
	target, err := filepath.Abs(filepath.Join(e.Cwd, dir))
	if err != nil {
		return err
	}
	if entries, err := os.ReadDir(target); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s ya existe y no está vacío", target)
	}

	ck, err := e.kit()
	if err != nil {
		return err
	}
	m, err := kit.LoadManifest(ck.Dir)
	if err != nil {
		return fmt.Errorf("no se pudo leer el kit: %w", err)
	}

	pt := kit.ProjectType(typeOverride)
	if typeOverride == "" {
		var ok bool
		if pt, ok = m.MatchProjectType(filepath.Base(target)); !ok {
			return fmt.Errorf("no se pudo determinar qué tipo de proyecto es %q; pasar --type (uno de: %s)",
				filepath.Base(target), strings.Join(projectTypeNames(m), ", "))
		}
	} else if _, ok := m.ProjectTypes[pt]; !ok {
		return fmt.Errorf("tipo de proyecto desconocido %q (conocidos: %s)", typeOverride, strings.Join(projectTypeNames(m), ", "))
	}

	// Check the toolchain before creating anything: a missing pnpm should be
	// reported here, not as a generator failing with a directory half made.
	report := doctor.Check(doctor.ForWeb())
	renderDoctor(e.Stdout, report)
	if !report.OK() {
		return fmt.Errorf("instalar la(s) herramienta(s) faltante(s) y volver a ejecutar")
	}
	pm, ok := report.PackageManager()
	if !ok {
		return errors.New("no se encontró ningún package manager: instalar pnpm (recomendado) o npm")
	}

	gen := generatorFor(pt, pm)
	if len(gen.Args) == 0 {
		return fmt.Errorf("no hay generador no interactivo para proyectos %s.\n"+
			"  Crearlo con la herramienta oficial y luego ejecutar deal-kit dentro:\n"+
			"    %s", pt, strings.ReplaceAll(gen.Manual, "{dir}", dir))
	}

	// Pass the directory as the user typed it, not the absolute path:
	// create-vite joins its argument onto the working directory even when it
	// is already absolute, which produced /tmp/tmp/crm-deal-web.
	args := make([]string, len(gen.Args))
	for i, a := range gen.Args {
		args[i] = strings.ReplaceAll(a, "{dir}", dir)
	}
	fmt.Fprintf(e.Stdout, "\n  creando un proyecto %s con:\n    %s\n\n", pt, strings.Join(args, " "))

	if !e.AssumeYes {
		ok, err := confirm(e, "¿ejecutarlo?")
		if err != nil {
			return err
		}
		if !ok {
			return ErrAborted
		}
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = e.Cwd
	cmd.Stdout, cmd.Stderr, cmd.Stdin = e.Stdout, e.Stderr, e.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s falló: %w", args[0], err)
	}

	fmt.Fprintf(e.Stdout, "\n  instalando el kit en %s\n\n", target)
	// Init runs against the new directory, which now has its own markers.
	inner := e
	inner.Cwd = target
	return Init(inner, string(pt))
}

// Doctor reports the state of the local toolchain.
func Doctor(e Env) error {
	report := doctor.Check(doctor.ForWeb())
	renderDoctor(e.Stdout, report)
	if !report.OK() {
		return fmt.Errorf("faltan %d herramienta(s) requerida(s)", len(report.Missing()))
	}
	return nil
}
