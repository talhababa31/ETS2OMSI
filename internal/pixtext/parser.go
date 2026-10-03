package pixtext

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Row struct {
	Index  int
	Values []string
}
type Section struct {
	Name     string
	Props    map[string][]string
	Rows     []Row
	Children []*Section
}

var sectionRE = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*\{\s*$`)
var propRE = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(.*?)\s*$`)
var rowRE = regexp.MustCompile(`^(-?\d+)\s*\((.*)\)\s*$`)

func ParseFile(p string) ([]*Section, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return Parse(string(b))
}
func Parse(text string) ([]*Section, error) {
	roots := []*Section{}
	stack := []*Section{}
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(stripComment(sc.Text()))
		if line == "" {
			continue
		}
		if m := sectionRE.FindStringSubmatch(line); m != nil {
			sec := &Section{Name: m[1], Props: map[string][]string{}}
			if len(stack) == 0 {
				roots = append(roots, sec)
			} else {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, sec)
			}
			stack = append(stack, sec)
			continue
		}
		if line == "}" || line == "};" {
			if len(stack) == 0 {
				return nil, fmt.Errorf("pix line %d: unexpected }", lineNo)
			}
			stack = stack[:len(stack)-1]
			continue
		}
		if len(stack) == 0 {
			continue
		}
		cur := stack[len(stack)-1]
		if m := propRE.FindStringSubmatch(line); m != nil {
			cur.Props[m[1]] = append(cur.Props[m[1]], strings.TrimSpace(m[2]))
			continue
		}
		if m := rowRE.FindStringSubmatch(line); m != nil {
			var idx int
			fmt.Sscanf(m[1], "%d", &idx)
			cur.Rows = append(cur.Rows, Row{Index: idx, Values: splitValues(m[2])})
			continue
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(stack) != 0 {
		return roots, fmt.Errorf("pix: %d unclosed section(s)", len(stack))
	}
	return roots, nil
}
func stripComment(s string) string {
	if i := strings.Index(s, "//"); i >= 0 {
		return s[:i]
	}
	return s
}
func splitValues(s string) []string {
	out := []string{}
	var b strings.Builder
	quote := rune(0)
	for _, r := range s {
		if quote != 0 {
			b.WriteRune(r)
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			b.WriteRune(r)
			continue
		}
		if r == ' ' || r == '\t' || r == ',' || r == ';' {
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}
func First(s *Section, key string) string {
	if s == nil {
		return ""
	}
	for k, v := range s.Props {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return unq(v[0])
		}
	}
	return ""
}
func Children(s *Section, name string) []*Section {
	out := []*Section{}
	for _, c := range s.Children {
		if strings.EqualFold(c.Name, name) {
			out = append(out, c)
		}
	}
	return out
}
func unq(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}
