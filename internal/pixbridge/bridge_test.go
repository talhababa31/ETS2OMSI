package pixbridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindMissingDoesNotPanic(t *testing.T) { _ = ToolStatus("/definitely/not/here") }

func TestCacheKeyChangesWithModel(t *testing.T) {
	d := t.TempDir()
	exe := filepath.Join(d, "converter_pix.exe")
	if err := os.WriteFile(exe, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	m := filepath.Join(d, "mod.scs")
	if err := os.WriteFile(m, []byte("m"), 0644); err != nil {
		t.Fatal(err)
	}
	a := cacheKey(exe, []string{m}, "/vehicle/a.pmd")
	b := cacheKey(exe, []string{m}, "/vehicle/b.pmd")
	if a == b || len(a) != 64 {
		t.Fatalf("bad cache keys %q %q", a, b)
	}
}
