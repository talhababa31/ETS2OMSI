package scanner

import (
	"archive/zip"
	"ets2omsi/internal/archive"
	"os"
	"path/filepath"
	"testing"
)

func TestUnboundVehicle(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "u.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	w, _ := z.Create("def/vehicle/ai/lonely.sii")
	w.Write([]byte("SiiNunit\n{\ntraffic_vehicle : traffic.lonely {\n name: \"Lonely\"\n}\n}"))
	z.Close()
	f.Close()
	s, _ := archive.Open(p, archive.OpenOptions{})
	defer s.Close()
	r, _ := Scan(s)
	if r.Stats.UnboundCount != 1 {
		t.Fatalf("%+v", r.Stats)
	}
}
