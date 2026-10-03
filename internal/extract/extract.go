package extract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ets2omsi/internal/archive"
	"ets2omsi/internal/graph"
	"ets2omsi/internal/scanner"
)

type Result struct {
	VehicleID string   `json:"vehicle_id"`
	Name      string   `json:"name"`
	Output    string   `json:"output"`
	Files     []string `json:"files"`
}

func Vehicle(packagePath string, v scanner.Vehicle, outputRoot string) (Result, error) {
	src, err := archive.Open(packagePath, archive.OpenOptions{})
	if err != nil {
		return Result{}, err
	}
	defer src.Close()
	if outputRoot == "" {
		outputRoot = "extracted"
	}
	name := safe(v.DisplayName)
	if name == "" {
		name = safe(v.ID)
	}
	dst := uniqueDir(outputRoot, "ETS2_"+name)
	if err := os.MkdirAll(filepath.Join(dst, "source"), 0755); err != nil {
		return Result{}, err
	}

	logicalToActual := map[string]string{}
	list := src.List()
	for _, f := range list {
		n := graph.Normalize(f.Path)
		logicalToActual[strings.ToLower(n)] = f.Path
	}
	resolve := func(logical string) (string, bool) {
		logical = graph.Normalize(logical)
		if a, ok := logicalToActual[strings.ToLower(logical)]; ok {
			return a, true
		}
		// Nested package root fallback: select a unique file whose path ends in the
		// requested game-root path.
		want := strings.ToLower(strings.TrimPrefix(logical, "/"))
		hit := ""
		for _, f := range list {
			got := strings.ToLower(strings.TrimPrefix(strings.ReplaceAll(f.Path, "\\", "/"), "/"))
			if got == want || strings.HasSuffix(got, "/"+want) {
				if hit != "" {
					return "", false
				}
				hit = f.Path
			}
		}
		return hit, hit != ""
	}

	selected := map[string]bool{}
	add := func(p string) {
		p = graph.Normalize(p)
		if p != "/" && p != "" {
			selected[strings.ToLower(p)] = true
		}
	}
	add(v.RootDefinition)
	for _, p := range v.Chassis {
		add(p)
	}
	for _, p := range v.Models {
		add(p)
	}
	for _, p := range v.WheelModels {
		add(p)
	}
	for _, n := range v.Dependencies {
		if n.Exists {
			add(n.Path)
		}
	}

	// PMD siblings are part of the same concrete model package. Include geometry,
	// collision and animation siblings without pulling unrelated traffic vehicles.
	for _, m := range append(append([]string{}, v.Models...), v.WheelModels...) {
		ext := filepath.Ext(m)
		stem := strings.TrimSuffix(m, ext)
		for _, e := range []string{".pmd", ".pmg", ".pmc", ".pma"} {
			add(stem + e)
		}
		dir := strings.TrimSuffix(graph.Normalize(filepath.Dir(strings.ReplaceAll(m, "\\", "/"))), "/") + "/"
		for _, f := range list {
			lp := strings.ToLower(graph.Normalize(f.Path))
			if strings.HasPrefix(lp, strings.ToLower(dir)) {
				// Keep model-local assets only; avoid recursively swallowing other
				// vehicle directories from large packs.
				rel := strings.TrimPrefix(lp, strings.ToLower(dir))
				if !strings.Contains(rel, "/") {
					add(f.Path)
				}
			}
		}
	}

	keys := make([]string, 0, len(selected))
	for k := range selected {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	copied := []string{}
	for _, k := range keys {
		actual, ok := resolve(k)
		if !ok {
			continue
		}
		b, er := src.Read(actual)
		if er != nil {
			continue
		}
		rel := strings.TrimPrefix(graph.Normalize(k), "/")
		out := filepath.Join(dst, "source", filepath.FromSlash(rel))
		if er = os.MkdirAll(filepath.Dir(out), 0755); er != nil {
			return Result{}, er
		}
		if er = os.WriteFile(out, b, 0644); er != nil {
			return Result{}, er
		}
		copied = append(copied, filepath.ToSlash(filepath.Join("source", rel)))
	}
	manifest := struct {
		Vehicle scanner.Vehicle `json:"vehicle"`
		Files   []string        `json:"files"`
		Note    string          `json:"note"`
	}{Vehicle: v, Files: copied, Note: "SCS-only vehicle snapshot. Storage-wide and unrelated vehicle assets are intentionally excluded."}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	_ = os.WriteFile(filepath.Join(dst, "vehicle_manifest.json"), mb, 0644)
	copied = append(copied, "vehicle_manifest.json")
	return Result{VehicleID: v.ID, Name: v.DisplayName, Output: dst, Files: copied}, nil
}

func safe(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == ' ' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return strings.Trim(strings.ReplaceAll(b.String(), " ", "_"), "_ .")
}
func uniqueDir(root, base string) string {
	_ = os.MkdirAll(root, 0755)
	p := filepath.Join(root, base)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return p
	}
	for i := 2; i < 10000; i++ {
		q := filepath.Join(root, fmt.Sprintf("%s_%d", base, i))
		if _, err := os.Stat(q); os.IsNotExist(err) {
			return q
		}
	}
	return filepath.Join(root, base+"_new")
}
