package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

type zipSource struct {
	path  string
	zr    *zip.ReadCloser
	files []FileInfo
	idx   map[string]*zip.File
}

func openZIP(p string) (*zipSource, error) {
	zr, err := zip.OpenReader(p)
	if err != nil {
		return nil, err
	}
	z := &zipSource{path: p, zr: zr, idx: map[string]*zip.File{}}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		n := normalize(f.Name)
		z.idx[strings.ToLower(n)] = f
		z.files = append(z.files, FileInfo{Path: n, Size: int64(f.UncompressedSize64)})
	}
	sort.Slice(z.files, func(i, j int) bool { return z.files[i].Path < z.files[j].Path })
	return z, nil
}
func (z *zipSource) Name() string         { return filepath.Base(z.path) }
func (z *zipSource) Kind() Kind           { return KindZIP }
func (z *zipSource) List() []FileInfo     { return append([]FileInfo(nil), z.files...) }
func (z *zipSource) Exists(p string) bool { _, ok := z.idx[strings.ToLower(normalize(p))]; return ok }
func (z *zipSource) Read(p string) ([]byte, error) {
	f := z.idx[strings.ToLower(normalize(p))]
	if f == nil {
		return nil, fmt.Errorf("file not found: %s", p)
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
func (z *zipSource) Close() error { return z.zr.Close() }
