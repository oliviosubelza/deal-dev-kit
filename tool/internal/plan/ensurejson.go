package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/paths"
)

// ensureJSONAction decides what an artifact's `ensure_json` needs, without
// touching the project. It is `ensure_line` for a file whose format is JSON:
// the project owns the file, so only the declared leaf keys are considered
// and every other key in it is none of the CLI's business.
//
//	file absent                     -> MergeJSON, and Apply creates it with the keys
//	every declared key already set  -> Unchanged
//	some declared key missing       -> MergeJSON, merged in beside the rest
//	a declared key holds another value -> Blocked
//
// That last case is where it deviates from ensureLineAction, which can never
// block. A line is either present or absent, so appending it destroys nothing.
// A key is different: a value already there was put there by the project on
// purpose, and replacing it would silently reverse a decision somebody made —
// the very thing every other Blocked in this package exists to prevent. There
// is no honest merge of two different answers to the same question, so a
// human decides.
//
// A file that is not valid JSON blocks for the same reason: the CLI cannot
// merge into what it cannot parse, and rewriting it whole would lose the file.
func ensureJSONAction(in Input, a kit.Artifact) (Action, []lockfile.EnsuredJSON, bool, error) {
	if a.EnsureJSON == nil {
		return Action{}, nil, false, nil
	}
	dest, err := paths.Resolve(a.EnsureJSON.File, in.Roots)
	if err != nil {
		return Action{}, nil, false, err
	}

	leaves, err := jsonLeaves(a.EnsureJSON.Values)
	if err != nil {
		return Action{}, nil, false, err
	}

	act := Action{ArtifactID: a.ID, Path: dest, Keys: leafKeys(leaves), jsonLeaves: leaves}

	data, err := os.ReadFile(filepath.Join(in.ProjectDir, filepath.FromSlash(dest)))
	if errors.Is(err, fs.ErrNotExist) {
		act.Kind = MergeJSON
		act.Reason = reasonJSONFileMissing
		return act, records(dest, leaves), true, nil
	}
	if err != nil {
		return Action{}, nil, false, err
	}

	doc, err := decodeJSONObject(data)
	if err != nil {
		act.Kind = Blocked
		act.Reason = reasonJSONUnparseable
		return act, nil, true, nil
	}

	missing := 0
	for _, leaf := range leaves {
		current, found, ok := lookupJSON(doc, leaf.path)
		switch {
		case !ok:
			// Something on the way to the key is a string, a number or an
			// array, so the key cannot exist without replacing it.
			act.Kind = Blocked
			act.Reason = fmt.Sprintf(reasonJSONNotAnObject, jsonPath(leaf.path))
			return act, nil, true, nil
		case !found:
			missing++
		case !jsonEqual(current, leaf.value):
			act.Kind = Blocked
			act.Reason = fmt.Sprintf(reasonJSONConflict, jsonPath(leaf.path))
			return act, nil, true, nil
		}
	}

	if missing == 0 {
		act.Kind = Unchanged
	} else {
		act.Kind = MergeJSON
		act.Reason = reasonJSONKeysMissing
	}
	return act, records(dest, leaves), true, nil
}

const (
	reasonJSONKeysMissing  = "faltan claves de configuración"
	reasonJSONFileMissing  = "el archivo no existe todavía"
	reasonJSONUnparseable  = "el archivo existe pero no es JSON válido; deal-kit no lo reescribe"
	reasonJSONConflict     = "«%s» ya tiene otro valor; lo decidió el proyecto y deal-kit no lo pisa"
	reasonJSONNotAnObject  = "«%s» no se puede fijar: una clave intermedia no es un objeto"
	errJSONApplyConflictFm = "«%s» cambió de valor entre planear y escribir; deal-kit no lo pisa"
)

// jsonLeaf is one declared key path and the value it must hold.
type jsonLeaf struct {
	path  []string
	value any
}

// jsonLeaves flattens a declared nested map into its leaves, so the unit of
// comparison is one key and not one object. Declaring `attribution` as a whole
// object and comparing it whole would mean a project that added a third key
// under it conflicts with the kit, when in fact it agrees about both keys the
// kit cares about.
//
// The values are normalised through JSON so they compare against a file's
// decoded values with the same Go types on both sides.
func jsonLeaves(values map[string]any) ([]jsonLeaf, error) {
	var out []jsonLeaf
	var walk func(prefix []string, m map[string]any) error
	walk = func(prefix []string, m map[string]any) error {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		// Sorted so a plan, a lockfile and a rendered row are identical on
		// every run: Go map order is not.
		sort.Strings(keys)
		for _, k := range keys {
			path := append(append([]string{}, prefix...), k)
			if nested, ok := m[k].(map[string]any); ok && len(nested) > 0 {
				if err := walk(path, nested); err != nil {
					return err
				}
				continue
			}
			encoded, err := json.Marshal(m[k])
			if err != nil {
				return err
			}
			value, err := decodeJSONValue(encoded)
			if err != nil {
				return err
			}
			out = append(out, jsonLeaf{path: path, value: value})
		}
		return nil
	}
	if err := walk(nil, values); err != nil {
		return nil, err
	}
	return out, nil
}

func leafKeys(leaves []jsonLeaf) []string {
	out := make([]string, len(leaves))
	for i, l := range leaves {
		out[i] = jsonPath(l.path)
	}
	return out
}

// records is what the lockfile stores for the leaves: the key and the value
// the CLI guaranteed, so deal-kit.lock shows what it did to a file it does not
// own.
func records(dest string, leaves []jsonLeaf) []lockfile.EnsuredJSON {
	out := make([]lockfile.EnsuredJSON, 0, len(leaves))
	for _, l := range leaves {
		encoded, err := json.Marshal(l.value)
		if err != nil {
			// Unreachable: every value came out of the JSON decoder.
			continue
		}
		out = append(out, lockfile.EnsuredJSON{
			Path: dest, Key: jsonPath(l.path), Value: string(encoded),
		})
	}
	return out
}

// lookupJSON walks a key path. ok is false when an intermediate key exists but
// is not an object, which is a conflict rather than a missing key.
func lookupJSON(doc *jsonObject, path []string) (value any, found, ok bool) {
	cur := doc
	for i, k := range path {
		v, exists := cur.get(k)
		if !exists {
			return nil, false, true
		}
		if i == len(path)-1 {
			return v, true, true
		}
		nested, isObj := v.(*jsonObject)
		if !isObj {
			return nil, false, false
		}
		cur = nested
	}
	return nil, false, true
}

// mergeJSON guarantees every declared leaf is set in the file at abs, leaving
// every other key exactly where and as it was.
//
// Like appendLine, it re-checks rather than trusting the planned Kind: Apply
// must converge even if the file changed between planning and writing, and
// here the stakes are higher — a value that appeared in the meantime is a
// decision the project made after the plan was computed, so writing over it is
// refused with an error instead of silently winning the race.
func mergeJSON(abs string, leaves []jsonLeaf) error {
	perm := fs.FileMode(0o644)
	doc := newJSONObject()

	data, err := os.ReadFile(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if doc, err = decodeJSONObject(data); err != nil {
			return fmt.Errorf("%s: %w", abs, err)
		}
		// Keep whatever mode the project gave the file: this is its file, and
		// the CLI is a guest in it.
		if info, err := os.Stat(abs); err == nil {
			perm = info.Mode().Perm()
		}
	}

	for _, leaf := range leaves {
		if err := setJSON(doc, leaf); err != nil {
			return err
		}
	}

	out, err := encodeJSONObject(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(abs, out, perm)
}

// setJSON writes one leaf, creating the objects on the way to it. A key that
// already holds a different value, or an intermediate key that is not an
// object, stops the whole merge.
func setJSON(doc *jsonObject, leaf jsonLeaf) error {
	cur := doc
	for i, k := range leaf.path {
		if i == len(leaf.path)-1 {
			if existing, ok := cur.get(k); ok && !jsonEqual(existing, leaf.value) {
				return fmt.Errorf(errJSONApplyConflictFm, jsonPath(leaf.path))
			}
			cur.set(k, leaf.value)
			return nil
		}
		v, ok := cur.get(k)
		if !ok {
			next := newJSONObject()
			cur.set(k, next)
			cur = next
			continue
		}
		nested, isObj := v.(*jsonObject)
		if !isObj {
			return fmt.Errorf(errJSONApplyConflictFm, jsonPath(leaf.path[:i+1]))
		}
		cur = nested
	}
	return nil
}
