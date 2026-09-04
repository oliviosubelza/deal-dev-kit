package cli

import (
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
)

// Every project type must have a command deal-kit can actually run. Printing
// one for the user to copy was based on the belief that NestJS and Expo have
// no single-command generator; both do. None of the three is silent on a
// terminal — see generatorFor — but New gives them the real stdin, so their
// own prompts are answerable, exactly as create-vite already was.
func TestGeneratorForEveryProjectTypeIsRunnable(t *testing.T) {
	cases := []struct {
		pt       kit.ProjectType
		contains []string
	}{
		{kit.Web, []string{"create", "vite@latest", "--template", "react-ts"}},
		{kit.Backend, []string{"dlx", "@nestjs/cli", "new", "--package-manager", "pnpm", "--skip-install"}},
		{kit.Mobile, []string{"create", "expo-app", "--template", "blank-typescript", "--no-install"}},
	}
	for _, tt := range cases {
		t.Run(string(tt.pt), func(t *testing.T) {
			gen := generatorFor(tt.pt, "pnpm")
			if len(gen.Args) == 0 {
				t.Fatalf("%s has no runnable command, only the manual hint %q", tt.pt, gen.Manual)
			}
			if gen.Args[0] != "pnpm" {
				t.Errorf("Args[0] = %q, want the detected package manager", gen.Args[0])
			}
			line := strings.Join(gen.Args, " ")
			for _, want := range tt.contains {
				if !strings.Contains(line, want) {
					t.Errorf("%s command %q is missing %q", tt.pt, line, want)
				}
			}
			// {dir} is what New substitutes; without it the generator would
			// scaffold into whatever name it invents.
			if !strings.Contains(line, "{dir}") {
				t.Errorf("%s command %q never names the target directory", tt.pt, line)
			}
		})
	}
}

func TestGeneratorForAnUnknownTypeHasNoCommand(t *testing.T) {
	if gen := generatorFor(kit.ProjectType("desktop"), "pnpm"); len(gen.Args) != 0 {
		t.Errorf("an undeclared project type got a command: %v", gen.Args)
	}
}
