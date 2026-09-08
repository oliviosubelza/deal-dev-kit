package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/selfupdate"
)

func TestVersionsMatch(t *testing.T) {
	tests := []struct {
		name, current, tag string
		want               bool
	}{
		{"exact match", "1.2.3", "1.2.3", true},
		{"tag carries the leading v", "1.2.3", "v1.2.3", true},
		{"current already carries a v", "v1.2.3", "v1.2.3", true},
		{"genuinely different versions", "1.2.3", "1.2.4", false},
		{"different versions, tag has v", "1.2.3", "v1.2.4", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionsMatch(tt.current, tt.tag); got != tt.want {
				t.Errorf("versionsMatch(%q, %q) = %v, want %v", tt.current, tt.tag, got, tt.want)
			}
		})
	}
}

// fakeSelfupdateServer serves one release the way GitHub and GoReleaser do,
// and points newSelfupdateClient at it for the duration of the test — the
// same seam-swap pattern the engram entry points in interactive.go already
// use.
func fakeSelfupdateServer(t *testing.T, tag string, binary []byte) {
	t.Helper()
	sum := sha256.Sum256(binary)
	sums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), selfupdate.AssetName())

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/kit/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q}`, tag)
	})
	mux.HandleFunc("/acme/kit/releases/download/"+tag+"/"+selfupdate.AssetName(), func(w http.ResponseWriter, _ *http.Request) {
		w.Write(binary)
	})
	mux.HandleFunc("/acme/kit/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, sums)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prev := newSelfupdateClient
	newSelfupdateClient = func(string) *selfupdate.Client {
		c := selfupdate.New("acme/kit")
		c.BaseURL, c.DownloadURL = srv.URL, srv.URL
		return c
	}
	t.Cleanup(func() { newSelfupdateClient = prev })
}

// replaceSpy stands in for the genuine file-replace machinery. os.Executable
// inside a `go test` run resolves to the compiled test binary, and actually
// renaming that mid-run would be catastrophic, not a simplification — so no
// test in this file ever lets the real selfupdate.Replace run.
type replaceSpy struct {
	called bool
	path   string
	binary []byte
}

func spyOnReplace(t *testing.T) *replaceSpy {
	t.Helper()
	spy := &replaceSpy{}
	prev := replaceBinary
	replaceBinary = func(path string, binary []byte) error {
		spy.called, spy.path, spy.binary = true, path, binary
		return nil
	}
	t.Cleanup(func() { replaceBinary = prev })
	return spy
}

func TestSelfUpdateRefusesADevBuildWithoutConfirmation(t *testing.T) {
	// "dev" is what a local build reports (Env.Version left empty); silently
	// replacing it with a public release is exactly the failure this guard
	// exists to prevent.
	fakeSelfupdateServer(t, "v9.9.9", []byte("new-binary"))
	spy := spyOnReplace(t)

	var out bytes.Buffer
	err := SelfUpdate(Env{Stdout: &out, ReleaseRepo: "acme/kit"}, false)
	if err == nil {
		t.Fatal("expected an error for a dev build without --yes")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error does not mention the way out: %q", err)
	}
	if spy.called {
		t.Error("a dev build was replaced despite the missing confirmation")
	}
}

func TestSelfUpdateAssumeYesUpdatesADevBuild(t *testing.T) {
	fakeSelfupdateServer(t, "v9.9.9", []byte("new-binary"))
	spy := spyOnReplace(t)

	var out bytes.Buffer
	err := SelfUpdate(Env{Stdout: &out, ReleaseRepo: "acme/kit", AssumeYes: true}, false)
	if err != nil {
		t.Fatalf("SelfUpdate() error = %v", err)
	}
	if !spy.called {
		t.Fatal("--yes did not let a dev build update")
	}
	if string(spy.binary) != "new-binary" {
		t.Errorf("replaced with %q, want the fetched release binary", spy.binary)
	}
}

func TestSelfUpdateSkipsAnAlreadyCurrentVersion(t *testing.T) {
	fakeSelfupdateServer(t, "v1.2.3", []byte("new-binary"))
	spy := spyOnReplace(t)

	var out bytes.Buffer
	err := SelfUpdate(Env{Stdout: &out, ReleaseRepo: "acme/kit", Version: "1.2.3"}, false)
	if err != nil {
		t.Fatalf("SelfUpdate() error = %v", err)
	}
	if !strings.Contains(out.String(), "ya está actualizado") {
		t.Errorf("output does not say it is current:\n%s", out.String())
	}
	if spy.called {
		t.Error("an already-current version was replaced anyway")
	}
}

func TestSelfUpdateCheckOnlyReportsWithoutInstalling(t *testing.T) {
	fakeSelfupdateServer(t, "v2.0.0", []byte("new-binary"))
	spy := spyOnReplace(t)

	var out bytes.Buffer
	err := SelfUpdate(Env{Stdout: &out, ReleaseRepo: "acme/kit", Version: "1.0.0"}, true)
	if err != nil {
		t.Fatalf("SelfUpdate() error = %v", err)
	}
	if !strings.Contains(out.String(), "v2.0.0") {
		t.Errorf("--check does not name the available version:\n%s", out.String())
	}
	if spy.called {
		t.Error("--check installed the update instead of only reporting it")
	}
}

func TestSelfUpdateFetchesAndReplacesOnAGenuineUpdate(t *testing.T) {
	fakeSelfupdateServer(t, "v2.0.0", []byte("new-binary-content"))
	spy := spyOnReplace(t)

	var out bytes.Buffer
	err := SelfUpdate(Env{Stdout: &out, ReleaseRepo: "acme/kit", Version: "1.0.0"}, false)
	if err != nil {
		t.Fatalf("SelfUpdate() error = %v", err)
	}
	if !spy.called {
		t.Fatal("a genuine update never reached the replace step")
	}
	if string(spy.binary) != "new-binary-content" {
		t.Errorf("replaced with %q, want the fetched release binary", spy.binary)
	}
	if spy.path == "" {
		t.Error("replaceBinary was called with no path to replace")
	}
	if !strings.Contains(out.String(), "1.0.0") || !strings.Contains(out.String(), "2.0.0") {
		t.Errorf("output does not report the version transition:\n%s", out.String())
	}
}
