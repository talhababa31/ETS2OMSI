package sii

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrEncoded = errors.New("SII/SUI is not plain UTF-8 text")

type Field struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	IsArray bool   `json:"is_array,omitempty"`
}
type Unit struct {
	Type   string  `json:"type"`
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
}
type Document struct {
	Path     string   `json:"path"`
	Includes []string `json:"includes"`
	Units    []Unit   `json:"units"`
	Warnings []string `json:"warnings,omitempty"`
}

var incRE = regexp.MustCompile(`(?i)^\s*@include\s+(?:"([^"]+)"|'([^']+)'|([^\s]+))`)
var unitRE = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*:\s*([^\s\{]+)\s*\{(.*)$`)
var unitHeaderRE = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*:\s*([^\s\{]+)\s*$`)

// Accept key, key[], and key[0] forms. Values intentionally remain conservative: quoted strings or one token.
var fieldRE = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)(\[(?:\d*)\])?\s*:\s*(?:"([^"\\]*(?:\\.[^"\\]*)*)"|'([^'\\]*(?:\\.[^'\\]*)*)'|([^\s\}]+))`)

func Parse(file string, data []byte) (Document, error) {
	d := Document{Path: file}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return d, ErrEncoded
	}
	// Real mods occasionally use C-style block comments around whole groups of definitions.
	data = stripBlockComments(data)
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var cur *Unit
	var pending *Unit // legacy/official style may put the opening brace on the next line
	depth := 0
	for sc.Scan() {
		raw := stripLineComment(sc.Text())
		trim := strings.TrimSpace(raw)
		if trim == "" {
			continue
		}

		// A large amount of real ETS2 data uses:
		//   traffic_vehicle_data : traffic.foo
		//   {
		// instead of placing the opening brace on the unit-header line.  The old
		// alpha parser silently skipped those files, which made valid Jazzycat-era
		// packages look as if they contained zero units.
		if pending != nil && cur == nil {
			if strings.HasPrefix(trim, "{") {
				cur = pending
				pending = nil
				depth = braceDelta(trim)
				appendFields(cur, strings.TrimPrefix(trim, "{"))
				if depth <= 0 {
					d.Units = append(d.Units, *cur)
					cur = nil
					depth = 0
				}
				continue
			}
			// If the next meaningful line was not an opening brace, the header was
			// malformed. Drop it and let the current line be parsed normally.
			d.Warnings = append(d.Warnings, fmt.Sprintf("unit %s missing opening brace", pending.Name))
			pending = nil
		}

		if m := incRE.FindStringSubmatch(trim); len(m) > 0 && cur == nil {
			ref := firstNonEmpty(m[1], m[2], m[3])
			if ref != "" {
				d.Includes = append(d.Includes, Resolve(file, ref))
			}
			continue
		}
		if cur == nil {
			if strings.EqualFold(trim, "SiiNunit") || trim == "{" || trim == "}" {
				continue
			}
			if m := unitRE.FindStringSubmatch(trim); len(m) > 0 {
				u := Unit{Type: m[1], Name: strings.Trim(m[2], `"'`)}
				cur = &u
				depth = 1
				tail := m[3]
				appendFields(cur, tail)
				depth += braceDelta(tail)
				if depth <= 0 {
					d.Units = append(d.Units, *cur)
					cur = nil
					depth = 0
				}
				continue
			}
			if m := unitHeaderRE.FindStringSubmatch(trim); len(m) > 0 {
				u := Unit{Type: m[1], Name: strings.Trim(m[2], `"'`)}
				pending = &u
			}
			continue
		}
		appendFields(cur, trim)
		depth += braceDelta(trim)
		if depth <= 0 {
			d.Units = append(d.Units, *cur)
			cur = nil
			depth = 0
		}
	}
	if err := sc.Err(); err != nil {
		return d, err
	}
	if pending != nil {
		d.Warnings = append(d.Warnings, fmt.Sprintf("unit %s missing opening brace", pending.Name))
	}
	if cur != nil {
		d.Warnings = append(d.Warnings, fmt.Sprintf("unterminated unit %s", cur.Name))
	}
	d.Includes = uniqueStrings(d.Includes)
	return d, nil
}

func appendFields(u *Unit, line string) {
	for _, m := range fieldRE.FindAllStringSubmatch(line, -1) {
		v := firstNonEmpty(m[3], m[4], m[5])
		u.Fields = append(u.Fields, Field{Key: m[1], IsArray: m[2] != "", Value: unquote(v)})
	}
}

func stripBlockComments(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString := byte(0)
	esc := false
	inComment := false
	for i := 0; i < len(in); i++ {
		c := in[i]
		if inComment {
			if c == '*' && i+1 < len(in) && in[i+1] == '/' {
				inComment = false
				i++
				continue
			}
			if c == '\n' || c == '\r' {
				out = append(out, c)
			}
			continue
		}
		if inString != 0 {
			out = append(out, c)
			if c == '\\' && !esc {
				esc = true
				continue
			}
			if c == inString && !esc {
				inString = 0
			}
			esc = false
			continue
		}
		if c == '"' || c == '\'' {
			inString = c
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(in) && in[i+1] == '*' {
			inComment = true
			i++
			continue
		}
		out = append(out, c)
	}
	return out
}

func stripLineComment(s string) string {
	in := byte(0)
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if in != 0 {
			if c == '\\' && !esc {
				esc = true
				continue
			}
			if c == in && !esc {
				in = 0
			}
			esc = false
			continue
		}
		if c == '"' || c == '\'' {
			in = c
			continue
		}
		if c == '#' {
			return s[:i]
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			return s[:i]
		}
	}
	return s
}

func braceDelta(s string) int {
	delta := 0
	in := byte(0)
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if in != 0 {
			if c == '\\' && !esc {
				esc = true
				continue
			}
			if c == in && !esc {
				in = 0
			}
			esc = false
			continue
		}
		if c == '"' || c == '\'' {
			in = c
			continue
		}
		if c == '{' {
			delta++
		}
		if c == '}' {
			delta--
		}
	}
	return delta
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
		v = v[1 : len(v)-1]
	}
	v = strings.ReplaceAll(v, `\"`, `"`)
	v = strings.ReplaceAll(v, `\'`, `'`)
	return v
}

func Resolve(current, ref string) string {
	ref = strings.ReplaceAll(strings.TrimSpace(strings.Trim(ref, `"'`)), "\\", "/")
	if strings.HasPrefix(ref, "/") {
		return clean(ref)
	}
	return clean(path.Join(path.Dir(current), ref))
}
func clean(p string) string {
	p = path.Clean("/" + strings.TrimPrefix(p, "/"))
	if p == "." {
		return "/"
	}
	return p
}

func UnitValues(u Unit, key string) []string {
	out := []string{}
	for _, f := range u.Fields {
		if strings.EqualFold(f.Key, key) {
			out = append(out, f.Value)
		}
	}
	return out
}
func First(u Unit, key string) string {
	v := UnitValues(u, key)
	if len(v) > 0 {
		return v[0]
	}
	return ""
}

// Absolute or relative asset-ish path ending in a known SCS/texture extension.
var pathRE = regexp.MustCompile(`(?i)(/?(?:[A-Za-z0-9_@+.,()\-]+/)*[A-Za-z0-9_@+.,()\-]+\.(?:sii|sui|pmd|pmg|pmc|pma|pim|pit|pic|mat|tobj|dds|png|tga|jpg|jpeg))`)

func PathsInValue(v string) []string { return PathsInValueAt("/", v) }
func PathsInValueAt(current, v string) []string {
	m := pathRE.FindAllString(v, -1)
	out := make([]string, 0, len(m))
	seen := map[string]bool{}
	for _, p := range m {
		if strings.HasPrefix(p, "/") {
			p = clean(p)
		} else {
			p = Resolve(current, p)
		}
		if !seen[strings.ToLower(p)] {
			seen[strings.ToLower(p)] = true
			out = append(out, p)
		}
	}
	return out
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		k := strings.ToLower(s)
		if !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	return out
}
