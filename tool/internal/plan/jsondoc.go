package plan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// This file is the ordered-JSON document `ensure_json` merges into. Go's
// encoding/json decodes an object into a map, and a map has no order, so
// re-encoding a file through map[string]any would alphabetise every key in
// it. For a file the kit does not own that is unacceptable: the project would
// see its whole .claude/settings.json rewritten to add two keys, and a diff
// that large hides the one change deal-kit actually made.
//
// So an object is decoded as its key order plus its values, and encoded back
// in that same order, with keys the kit adds appended at the end. Numbers are
// kept as json.Number, i.e. as the exact literal the file carried: re-encoding
// 1 as 1.0 would be a silent edit too.

// jsonObject is a JSON object that remembers the order its keys appeared in.
type jsonObject struct {
	keys []string
	vals map[string]any
}

func newJSONObject() *jsonObject {
	return &jsonObject{vals: map[string]any{}}
}

// get returns the value stored under key.
func (o *jsonObject) get(key string) (any, bool) {
	v, ok := o.vals[key]
	return v, ok
}

// set replaces the value under key, keeping its position, or appends the key
// at the end when it is new.
func (o *jsonObject) set(key string, v any) {
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = v
}

var (
	errNotAnObject   = errors.New("el contenido no es un objeto JSON")
	errTrailingBytes = errors.New("hay contenido después del objeto JSON")
)

// decodeJSONObject parses data as a single JSON object, preserving key order.
func decodeJSONObject(data []byte) (*jsonObject, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errNotAnObject
	}
	obj, err := decodeObjectBody(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errTrailingBytes
	}
	return obj, nil
}

// decodeJSONValue parses data as a single JSON value of any shape. It is how a
// value declared in kit.yaml is normalised: the declared value round-trips
// through JSON so it is compared against the file with the same Go types the
// file itself decodes to, rather than the ones YAML happened to produce.
func decodeJSONValue(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errTrailingBytes
	}
	return v, nil
}

// decodeObjectBody reads members until the closing brace, which it consumes.
func decodeObjectBody(dec *json.Decoder) (*jsonObject, error) {
	obj := newJSONObject()
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("clave JSON inesperada %v", tok)
		}
		v, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		obj.set(key, v)
	}
	if _, err := dec.Token(); err != nil { // the closing '}'
		return nil, err
	}
	return obj, nil
}

// decodeValue reads one value: an object, an array, or a scalar.
func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return tok, nil // string, bool, nil, or json.Number
	}
	switch d {
	case '{':
		return decodeObjectBody(dec)
	case '[':
		arr := []any{}
		for dec.More() {
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		if _, err := dec.Token(); err != nil { // the closing ']'
			return nil, err
		}
		return arr, nil
	}
	return nil, fmt.Errorf("delimitador JSON inesperado %v", d)
}

// jsonIndent is the indentation ensure_json writes. Two spaces is what Claude
// Code itself writes into .claude/settings.json, and matching it keeps a merge
// from reindenting a file the kit does not own.
const jsonIndent = "  "

// encodeJSONObject renders the object with a trailing newline, in the key
// order it carries.
func encodeJSONObject(o *jsonObject) ([]byte, error) {
	var b bytes.Buffer
	if err := writeValue(&b, o, ""); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func writeValue(b *bytes.Buffer, v any, pad string) error {
	switch t := v.(type) {
	case *jsonObject:
		if len(t.keys) == 0 {
			b.WriteString("{}")
			return nil
		}
		inner := pad + jsonIndent
		b.WriteString("{\n")
		for i, k := range t.keys {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(inner)
			key, err := json.Marshal(k)
			if err != nil {
				return err
			}
			b.Write(key)
			b.WriteString(": ")
			if err := writeValue(b, t.vals[k], inner); err != nil {
				return err
			}
		}
		b.WriteString("\n" + pad + "}")
		return nil
	case []any:
		if len(t) == 0 {
			b.WriteString("[]")
			return nil
		}
		inner := pad + jsonIndent
		b.WriteString("[\n")
		for i, item := range t {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(inner)
			if err := writeValue(b, item, inner); err != nil {
				return err
			}
		}
		b.WriteString("\n" + pad + "]")
		return nil
	default:
		enc, err := json.Marshal(v)
		if err != nil {
			return err
		}
		b.Write(enc)
		return nil
	}
}

// jsonEqual compares two decoded JSON values. Both sides come from the same
// decoder, so a number is a json.Number on both: 1 and 1.0 are treated as
// different values, which is the conservative answer — the CLI would have to
// rewrite the literal to make them equal, and rewriting is exactly what it
// refuses to do here.
func jsonEqual(a, b any) bool {
	switch x := a.(type) {
	case *jsonObject:
		y, ok := b.(*jsonObject)
		if !ok || len(x.keys) != len(y.keys) {
			return false
		}
		for _, k := range x.keys {
			v, ok := y.get(k)
			if !ok || !jsonEqual(x.vals[k], v) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !jsonEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

// jsonPath renders a key path for a human: ["attribution","commit"] reads as
// "attribution.commit".
func jsonPath(path []string) string { return strings.Join(path, ".") }
