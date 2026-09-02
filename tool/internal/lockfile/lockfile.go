// Package lockfile reads and writes deal-kit.lock, the record of what the CLI
// owns inside a consumer project. The CLI must never write to or delete a path
// that is not recorded here.
package lockfile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Name is the lockfile's filename at the root of a consumer project.
const Name = "deal-kit.lock"

// File is the on-disk deal-kit.lock.
type File struct {
	KitVersion  string            `yaml:"kit_version"`
	ProjectType string            `yaml:"project_type"`
	Roots       map[string]string `yaml:"roots"`
	Artifacts   []Installed       `yaml:"artifacts"`
}

// Installed records one installed artifact and every file it owns.
type Installed struct {
	ID    string      `yaml:"id"`
	Files []OwnedFile `yaml:"files"`
}

// OwnedFile binds a path to the hash the CLI wrote. A mismatch on the next
// sync means a human edited the file: the CLI reports and refuses to overwrite
// rather than clobbering the change.
type OwnedFile struct {
	Path string `yaml:"path"` // slash-separated, relative to the project root
	Hash string `yaml:"hash"` // sha256 of the content as written by the CLI
}

// Load reads the lockfile at the root of projectDir. A project with no
// lockfile yet returns a zero File and false, not an error.
func Load(projectDir string) (*File, bool, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, Name))
	if errors.Is(err, fs.ErrNotExist) {
		return &File{Roots: map[string]string{}}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, false, fmt.Errorf("%s: %w", Name, err)
	}
	if f.Roots == nil {
		f.Roots = map[string]string{}
	}
	return &f, true, nil
}

// Save writes the lockfile, sorted so that repeated runs produce identical
// output and a diff only ever shows a real change.
func (f *File) Save(projectDir string) error {
	f.sort()
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	header := []byte("# Managed by deal-kit. Records every file the CLI owns in this project.\n" +
		"# Do not edit by hand: the hashes are how deal-kit detects local changes.\n")
	return os.WriteFile(filepath.Join(projectDir, Name), append(header, data...), 0o644)
}

func (f *File) sort() {
	sort.Slice(f.Artifacts, func(i, j int) bool { return f.Artifacts[i].ID < f.Artifacts[j].ID })
	for i := range f.Artifacts {
		files := f.Artifacts[i].Files
		sort.Slice(files, func(a, b int) bool { return files[a].Path < files[b].Path })
	}
}

// Artifact returns the record for an installed artifact.
func (f *File) Artifact(id string) (Installed, bool) {
	for _, a := range f.Artifacts {
		if a.ID == id {
			return a, true
		}
	}
	return Installed{}, false
}

// Set inserts or replaces the record for an artifact.
func (f *File) Set(in Installed) {
	for i, a := range f.Artifacts {
		if a.ID == in.ID {
			f.Artifacts[i] = in
			return
		}
	}
	f.Artifacts = append(f.Artifacts, in)
}

// Remove drops an artifact's record.
func (f *File) Remove(id string) {
	for i, a := range f.Artifacts {
		if a.ID == id {
			f.Artifacts = append(f.Artifacts[:i], f.Artifacts[i+1:]...)
			return
		}
	}
}

// Owns reports whether a project-relative path is recorded in the lockfile.
// Every destructive operation must consult this first.
func (f *File) Owns(path string) bool {
	for _, a := range f.Artifacts {
		for _, file := range a.Files {
			if file.Path == path {
				return true
			}
		}
	}
	return false
}

// RecordedHash returns the hash the CLI wrote for a path.
func (f *File) RecordedHash(path string) (string, bool) {
	for _, a := range f.Artifacts {
		for _, file := range a.Files {
			if file.Path == path {
				return file.Hash, true
			}
		}
	}
	return "", false
}

// Hash is the content hash used throughout: sha256, hex-encoded.
func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// HashFile hashes a file on disk. A missing file returns ("", false).
func HashFile(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return Hash(data), true, nil
}
