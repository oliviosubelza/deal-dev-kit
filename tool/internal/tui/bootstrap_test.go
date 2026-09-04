package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

func bootstrapConfig() BootstrapConfig {
	return BootstrapConfig{
		Dir:        "/projects/nuevo-servicio",
		Markers:    []string{"package.json", ".git", "deal-kit.lock"},
		Types:      []string{"backend", "mobile", "web"},
		CLIVersion: "0.1.7",
	}
}

func sendBootstrap(t *testing.T, m BootstrapModel, keys ...string) BootstrapModel {
	t.Helper()
	var model tea.Model = m
	for _, k := range keys {
		next, cmd := model.Update(key(k))
		model = next
		for cmd != nil {
			msg := cmd()
			if _, isQuit := msg.(tea.QuitMsg); isQuit || msg == nil {
				break
			}
			next, cmd = model.Update(msg)
			model = next
		}
	}
	return model.(BootstrapModel)
}

func TestBootstrapStartsUnconfirmed(t *testing.T) {
	m := NewBootstrap(bootstrapConfig())
	res := m.Result()
	if res.Confirmed {
		t.Error("a fresh screen reports a confirmed directory before the user answered")
	}
	if res.ProjectType != "" {
		t.Errorf("ProjectType = %q, want empty", res.ProjectType)
	}
}

func TestBootstrapCancelLeavesNothingConfirmed(t *testing.T) {
	// Cancel is the whole point of the screen: --here is opt-in because a
	// mistyped cd must not grow a source tree, and this is its equivalent.
	for _, keys := range [][]string{{"esc"}, {"q"}, {"ctrl+c"}, {"down", "enter"}} {
		m := sendBootstrap(t, NewBootstrap(bootstrapConfig()), keys...)
		if res := m.Result(); res.Confirmed {
			t.Errorf("keys %v confirmed the directory", keys)
		}
	}
}

func TestBootstrapConfirmMovesToTheTypeChoice(t *testing.T) {
	m := sendBootstrap(t, NewBootstrap(bootstrapConfig()), "enter")
	if m.step != bootstrapType {
		t.Fatalf("step = %v, want the type choice", m.step)
	}
	if m.Result().Confirmed {
		t.Error("confirming the directory already reported a result; the type is still unchosen")
	}
	view := ansi.ReplaceAllString(m.View(), "")
	for _, want := range []string{"backend", "mobile", "web"} {
		if !strings.Contains(view, want) {
			t.Errorf("type choice does not offer %q\n%s", want, view)
		}
	}
}

func TestBootstrapTypeOptionsComeFromTheManifestTypes(t *testing.T) {
	// A different manifest must produce different options: a hardcoded
	// backend/web/mobile list here is the second list that drifts.
	cfg := bootstrapConfig()
	cfg.Types = []string{"api", "desktop"}
	m := sendBootstrap(t, NewBootstrap(cfg), "enter")
	view := ansi.ReplaceAllString(m.View(), "")
	for _, want := range []string{"api", "desktop"} {
		if !strings.Contains(view, want) {
			t.Errorf("type choice does not offer %q\n%s", want, view)
		}
	}
	for _, absent := range []string{"backend", "mobile", "web"} {
		if strings.Contains(view, absent) {
			t.Errorf("type choice offers %q, which this manifest does not declare\n%s", absent, view)
		}
	}
}

func TestBootstrapChoosingATypeReportsIt(t *testing.T) {
	cases := []struct {
		keys []string
		want string
	}{
		{[]string{"enter", "enter"}, "backend"},
		{[]string{"enter", "down", "enter"}, "mobile"},
		{[]string{"enter", "down", "down", "enter"}, "web"},
	}
	for _, tt := range cases {
		m := sendBootstrap(t, NewBootstrap(bootstrapConfig()), tt.keys...)
		res := m.Result()
		if !res.Confirmed {
			t.Errorf("keys %v did not confirm", tt.keys)
		}
		if res.ProjectType != tt.want {
			t.Errorf("keys %v chose %q, want %q", tt.keys, res.ProjectType, tt.want)
		}
	}
}

func TestBootstrapPreselectsTheTypeGivenOnTheCommandLine(t *testing.T) {
	cfg := bootstrapConfig()
	cfg.Selected = "web"
	m := sendBootstrap(t, NewBootstrap(cfg), "enter", "enter")
	if got := m.Result().ProjectType; got != "web" {
		t.Errorf("ProjectType = %q, want web (--type web was passed)", got)
	}
}

func TestBootstrapBackFromTheTypeChoiceReturnsToTheQuestion(t *testing.T) {
	m := sendBootstrap(t, NewBootstrap(bootstrapConfig()), "enter", "esc")
	if m.step != bootstrapConfirm {
		t.Fatalf("step = %v, want the confirm question", m.step)
	}
	if m.Result().Confirmed {
		t.Error("stepping back still reported a confirmed directory")
	}
}

func TestBootstrapNamesTheDirectoryAndWhatWasMissing(t *testing.T) {
	view := ansi.ReplaceAllString(NewBootstrap(bootstrapConfig()).View(), "")
	for _, want := range []string{"/projects/nuevo-servicio", "package.json", ".git", "deal-kit.lock"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not mention %q\n%s", want, view)
		}
	}
}

func TestViewBootstrapQuestion(t *testing.T) {
	assertGolden(t, "bootstrap", ansi.ReplaceAllString(NewBootstrap(bootstrapConfig()).View(), ""))
}

func TestViewBootstrapTypeChoice(t *testing.T) {
	m := sendBootstrap(t, NewBootstrap(bootstrapConfig()), "enter")
	assertGolden(t, "bootstrap-type", ansi.ReplaceAllString(m.View(), ""))
}

func TestBootstrapLinesFitEveryPanelWidth(t *testing.T) {
	// A line wider than the content width wraps onto a second one, and the
	// panel then pads that remnant to full width so it looks deliberate. The
	// only way to see it is to measure the lines before they are rendered —
	// with lipgloss.Width, never len: the styles emit ANSI that len counts.
	//
	// Width 0 is what a pty with no size reports, which is where the real
	// binary showed every sentence broken mid-word.
	for _, w := range []int{0, 40, 46, 60, 80, 200} {
		for _, step := range []bootstrapStep{bootstrapConfirm, bootstrapType} {
			m := NewBootstrap(bootstrapConfig())
			m.width = w
			if step == bootstrapType {
				m = sendBootstrap(t, m, "enter")
			}
			for _, line := range m.lines() {
				if got := lipgloss.Width(line); got > m.content() {
					t.Errorf("width %d, step %v: line is %d wide, content is %d:\n%q",
						w, step, got, m.content(), ansi.ReplaceAllString(line, ""))
				}
			}
		}
	}
}
