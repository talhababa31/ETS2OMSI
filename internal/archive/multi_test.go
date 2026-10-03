package archive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultiPriority(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if err := os.MkdirAll(filepath.Join(a, "def"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b, "def"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(a, "def", "x.sii"), []byte("base"), 0644)
	os.WriteFile(filepath.Join(b, "def", "x.sii"), []byte("mod"), 0644)
	sa, _ := openDir(a, false)
	sb, _ := openDir(b, false)
	m, err := OpenMulti("x", []Mount{{Label: "base", Priority: 10, Source: sa}, {Label: "mod", Priority: 100, Source: sb}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := m.Read("/def/x.sii")
	if string(got) != "mod" {
		t.Fatalf("got %q", got)
	}
	o, ok := m.Origin("/def/x.sii")
	if !ok || o.Label != "mod" {
		t.Fatalf("origin=%+v", o)
	}
}
