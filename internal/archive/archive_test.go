package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZIPSource(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "x.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	w, _ := z.Create("def/vehicle/test.sii")
	w.Write([]byte("abc"))
	z.Close()
	f.Close()
	s, err := Open(p, OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Kind() != KindZIP {
		t.Fatal(s.Kind())
	}
	if !s.Exists("/def/vehicle/test.sii") {
		t.Fatal("missing")
	}
	b, _ := s.Read("def/vehicle/test.sii")
	if string(b) != "abc" {
		t.Fatal(string(b))
	}
}
