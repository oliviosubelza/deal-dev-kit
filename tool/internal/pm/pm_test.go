package pm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name   string
		files  map[string]string
		want   Manager
		wantOK bool
	}{
		{
			name:  "pnpm lockfile",
			files: map[string]string{"pnpm-lock.yaml": ""},
			want:  PNPM, wantOK: true,
		},
		{
			name:  "npm lockfile",
			files: map[string]string{"package-lock.json": "{}"},
			want:  NPM, wantOK: true,
		},
		{
			name:  "yarn lockfile",
			files: map[string]string{"yarn.lock": ""},
			want:  Yarn, wantOK: true,
		},
		{
			name:  "bun lockfile",
			files: map[string]string{"bun.lockb": ""},
			want:  Bun, wantOK: true,
		},
		{
			name: "lockfile wins over packageManager field",
			files: map[string]string{
				"pnpm-lock.yaml": "",
				"package.json":   `{"packageManager":"npm@10.0.0"}`,
			},
			want: PNPM, wantOK: true,
		},
		{
			name:  "packageManager field when there is no lockfile",
			files: map[string]string{"package.json": `{"packageManager":"pnpm@9.1.0"}`},
			want:  PNPM, wantOK: true,
		},
		{
			name:   "package.json without packageManager",
			files:  map[string]string{"package.json": `{"name":"x"}`},
			wantOK: false,
		},
		{
			name:   "unknown packageManager value",
			files:  map[string]string{"package.json": `{"packageManager":"deno@1"}`},
			wantOK: false,
		},
		{
			name:   "malformed package.json",
			files:  map[string]string{"package.json": `{not json`},
			wantOK: false,
		},
		{
			name:   "empty directory",
			files:  map[string]string{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, ok := Detect(dir)
			if ok != tt.wantOK || (tt.wantOK && got != tt.want) {
				t.Errorf("Detect() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestInstallArgs(t *testing.T) {
	deps := map[string]string{
		"clsx":           "^2.1.1",
		"tailwind-merge": "^3.5.0",
	}
	tests := []struct {
		manager Manager
		want    string
	}{
		{PNPM, "pnpm add clsx@^2.1.1 tailwind-merge@^3.5.0"},
		{Yarn, "yarn add clsx@^2.1.1 tailwind-merge@^3.5.0"},
		{Bun, "bun add clsx@^2.1.1 tailwind-merge@^3.5.0"},
		{NPM, "npm install --save clsx@^2.1.1 tailwind-merge@^3.5.0"},
	}
	for _, tt := range tests {
		t.Run(string(tt.manager), func(t *testing.T) {
			if got := strings.Join(InstallArgs(tt.manager, deps), " "); got != tt.want {
				t.Errorf("InstallArgs = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstallArgsIsDeterministic(t *testing.T) {
	deps := map[string]string{"z": "1", "a": "2", "m": "3"}
	first := strings.Join(InstallArgs(PNPM, deps), " ")
	for i := 0; i < 20; i++ {
		if got := strings.Join(InstallArgs(PNPM, deps), " "); got != first {
			t.Fatalf("map iteration leaked into the command: %q vs %q", got, first)
		}
	}
	if !strings.Contains(first, "a@2 m@3 z@1") {
		t.Errorf("dependencies are not sorted: %q", first)
	}
}

func TestInstallWithNoDepsDoesNothing(t *testing.T) {
	if err := Install(t.TempDir(), PNPM, nil, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
