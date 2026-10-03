package pit

import (
	"regexp"
	"strconv"
	"strings"

	"ets2omsi/internal/pixtext"
)

// VariantFilter describes the visibility state encoded in a PIT Variant section.
type VariantFilter struct {
	Name         string          `json:"name"`
	VisibleParts map[string]bool `json:"visible_parts"`
	Found        bool            `json:"found"`
}

var intRE = regexp.MustCompile(`-?\d+`)

func Variant(file, name string) (VariantFilter, error) {
	secs, err := pixtext.ParseFile(file)
	if err != nil {
		return VariantFilter{}, err
	}
	return VariantFromSections(secs, name), nil
}

func VariantFromSections(secs []*pixtext.Section, name string) VariantFilter {
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		want = "default"
	}
	out := VariantFilter{Name: name, VisibleParts: map[string]bool{}}
	for _, s := range secs {
		if !strings.EqualFold(s.Name, "Variant") {
			continue
		}
		sn := strings.TrimSpace(pixtext.First(s, "Name"))
		if !strings.EqualFold(sn, want) {
			continue
		}
		out.Name, out.Found = sn, true
		for _, p := range pixtext.Children(s, "Part") {
			pn := strings.ToLower(strings.TrimSpace(pixtext.First(p, "Name")))
			if pn == "" {
				continue
			}
			visible := true
			for _, a := range pixtext.Children(p, "Attribute") {
				if !strings.EqualFold(strings.TrimSpace(pixtext.First(a, "Tag")), "visible") {
					continue
				}
				visible = parseInt(pixtext.First(a, "Value"), 1) != 0
			}
			out.VisibleParts[pn] = visible
		}
		return out
	}
	// Some old wheel models call their two variants left/right without a default.
	// A missing variant is not an error: the caller can choose a safer fallback.
	return out
}

func AvailableVariants(file string) ([]string, error) {
	secs, err := pixtext.ParseFile(file)
	if err != nil {
		return nil, err
	}
	out := []string{}
	seen := map[string]bool{}
	for _, s := range secs {
		if !strings.EqualFold(s.Name, "Variant") {
			continue
		}
		n := strings.TrimSpace(pixtext.First(s, "Name"))
		if n == "" {
			continue
		}
		k := strings.ToLower(n)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, n)
	}
	return out, nil
}

func parseInt(s string, d int) int {
	m := intRE.FindString(s)
	if m == "" {
		return d
	}
	n, err := strconv.Atoi(m)
	if err != nil {
		return d
	}
	return n
}
