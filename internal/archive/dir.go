package archive

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type dirSource struct {
	root    string
	name    string
	files   []FileInfo
	idx     map[string]string
	cleanup bool
}

func openDir(root string, cleanup bool) (*dirSource, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	d := &dirSource{root: abs, name: filepath.Base(abs), idx: map[string]string{}, cleanup: cleanup}
	err = filepath.WalkDir(abs, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(abs, p)
		if err != nil {
			return err
		}
		st, err := e.Info()
		if err != nil {
			return err
		}
		n := normalize(rel)
		d.files = append(d.files, FileInfo{Path: n, Size: st.Size()})
		d.idx[strings.ToLower(n)] = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(d.files, func(i, j int) bool { return d.files[i].Path < d.files[j].Path })
	return d, nil
}
func (d *dirSource) Name() string     { return d.name }
func (d *dirSource) Kind() Kind       { return KindDirectory }
func (d *dirSource) List() []FileInfo { return append([]FileInfo(nil), d.files...) }
func (d *dirSource) Read(p string) ([]byte, error) {
	return os.ReadFile(d.idx[strings.ToLower(normalize(p))])
}
func (d *dirSource) Exists(p string) bool { _, ok := d.idx[strings.ToLower(normalize(p))]; return ok }
func (d *dirSource) Close() error {
	if d.cleanup {
		return os.RemoveAll(d.root)
	}
	return nil
}
