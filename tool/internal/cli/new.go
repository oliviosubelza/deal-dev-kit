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
	// Empty means no single non-interactive command exists, and the user is
	// told what to run instead of being handed a half-automated wizard.
	Args   []string
	Manual string
}

func generatorFor(pt kit.ProjectType, pm string) generator {
	switch pt {
	case kit.Web:
		return generator{Args: []string{pm, "create", "vite@latest", "{dir}", "--template", "react-ts"}}
	case kit.Backend:
		return generator{Manual: pm + " dlx @nestjs/cli new {dir} --package-manager " + pm}
	case kit.Mobile:
		return generator{Manual: pm + " create expo-app {dir} --template blank-typescript"}
	}
	return generator{}
}

// New scaffolds a project with its official generator, then installs the kit
// into it.
func New(e Env, dir, typeOverride string) error {
	if dir == "" {
		return errors.New("name the directory to create: deal-kit new crm-deal-web")
	}
	target, err := filepath.Abs(filepath.Join(e.Cwd, dir))
	if err != nil {
		return err
	}
	if entries, err := os.ReadDir(target); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s already exists and is not empty", target)
	}

	ck, err := e.kit()
	if err != nil {
		return err
	}
	m, err := kit.LoadManifest(ck.Dir)
	if err != nil {
		return fmt.Errorf("reading the kit: %w", err)
	}

	pt := kit.ProjectType(typeOverride)
	if typeOverride == "" {
		var ok bool
		if pt, ok = m.MatchProjectType(filepath.Base(target)); !ok {
			return fmt.Errorf("could not tell what kind of project %q is; pass --type (one of: %s)",
				filepath.Base(target), strings.Join(projectTypeNames(m), ", "))
		}
	} else if _, ok := m.ProjectTypes[pt]; !ok {
		return fmt.Errorf("unknown project type %q (known: %s)", typeOverride, strings.Join(projectTypeNames(m), ", "))
	}

	// Check the toolchain before creating anything: a missing pnpm should be
	// reported here, not as a generator failing with a directory half made.
	report := doctor.Check(doctor.ForWeb())
	renderDoctor(e.Stdout, report)
	if !report.OK() {
		return fmt.Errorf("install the missing tool(s) and run again")
	}
	pm, ok := report.PackageManager()
	if !ok {
		return errors.New("no package manager found: install pnpm (recommended) or npm")
	}

	gen := generatorFor(pt, pm)
	if len(gen.Args) == 0 {
		return fmt.Errorf("no non-interactive generator for %s projects.\n"+
			"  Create it with the official tool, then run deal-kit inside it:\n"+
			"    %s", pt, strings.ReplaceAll(gen.Manual, "{dir}", dir))
	}

	// Pass the directory as the user typed it, not the absolute path:
	// create-vite joins its argument onto the working directory even when it
	// is already absolute, which produced /tmp/tmp/crm-deal-web.
	args := make([]string, len(gen.Args))
	for i, a := range gen.Args {
		args[i] = strings.ReplaceAll(a, "{dir}", dir)
	}
	fmt.Fprintf(e.Stdout, "\n  creating a %s project with:\n    %s\n\n", pt, strings.Join(args, " "))

	if !e.AssumeYes {
		ok, err := confirm(e, "run it?")
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
		return fmt.Errorf("%s failed: %w", args[0], err)
	}

	fmt.Fprintf(e.Stdout, "\n  installing the kit into %s\n\n", target)
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
		return fmt.Errorf("%d required tool(s) missing", len(report.Missing()))
	}
	return nil
}
