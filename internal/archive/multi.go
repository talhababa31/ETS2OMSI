package archive

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Mount describes one archive/directory in the virtual ETS2 filesystem.
// Higher Priority wins when several mounts contain the same logical path.
type Mount struct {
	Label    string `json:"label"`
	Path     string `json:"path"`
	Priority int    `json:"priority"`
	Source   Source `json:"-"`
}

type Origin struct {
	Label    string `json:"label"`
	Path     string `json:"path"`
	Kind     Kind   `json:"kind"`
	Priority int    `json:"priority"`
}

type multiEntry struct {
	file  FileInfo
	mount Mount
}

type MultiSource struct {
	name   string
	mounts []Mount // high -> low priority
	files  []FileInfo
	idx    map[string]multiEntry
}

func OpenMulti(name string, mounts []Mount) (*MultiSource, error) {
	if len(mounts) == 0 {
		return nil, fmt.Errorf("archive: no mounts")
	}
	mm := append([]Mount(nil), mounts...)
	sort.SliceStable(mm, func(i, j int) bool { return mm[i].Priority > mm[j].Priority })
	m := &MultiSource{name: name, mounts: mm, idx: map[string]multiEntry{}}
	for _, mt := range mm {
		if mt.Source == nil {
			continue
		}
		for _, fi := range mt.Source.List() {
			n := normalize(fi.Path)
			k := strings.ToLower(n)
			if _, exists := m.idx[k]; exists {
				continue
			}
			m.idx[k] = multiEntry{file: FileInfo{Path: n, Size: fi.Size}, mount: mt}
		}
	}
	for _, e := range m.idx {
		m.files = append(m.files, e.file)
	}
	sort.Slice(m.files, func(i, j int) bool { return m.files[i].Path < m.files[j].Path })
	if m.name == "" {
		m.name = "ETS2 virtual filesystem"
	}
	return m, nil
}

func (m *MultiSource) Name() string         { return m.name }
func (m *MultiSource) Kind() Kind           { return Kind("multi") }
func (m *MultiSource) List() []FileInfo     { return append([]FileInfo(nil), m.files...) }
func (m *MultiSource) Exists(p string) bool { _, ok := m.idx[strings.ToLower(normalize(p))]; return ok }
func (m *MultiSource) Read(p string) ([]byte, error) {
	e, ok := m.idx[strings.ToLower(normalize(p))]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", p)
	}
	return e.mount.Source.Read(e.file.Path)
}
func (m *MultiSource) Origin(p string) (Origin, bool) {
	e, ok := m.idx[strings.ToLower(normalize(p))]
	if !ok {
		return Origin{}, false
	}
	return Origin{Label: e.mount.Label, Path: e.mount.Path, Kind: e.mount.Source.Kind(), Priority: e.mount.Priority}, true
}
func (m *MultiSource) Mounts() []Mount {
	out := make([]Mount, len(m.mounts))
	copy(out, m.mounts)
	return out
}
func (m *MultiSource) Close() error {
	var first error
	seen := map[Source]bool{}
	for _, mt := range m.mounts {
		if mt.Source == nil || seen[mt.Source] {
			continue
		}
		seen[mt.Source] = true
		if err := mt.Source.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// OpenMounts opens all supplied paths and creates one priority virtual filesystem.
// The caller should place the selected mod at the highest priority.
func OpenMounts(name string, specs []Mount, opts OpenOptions) (*MultiSource, error) {
	opened := make([]Mount, 0, len(specs))
	for _, s := range specs {
		if strings.TrimSpace(s.Path) == "" {
			continue
		}
		src, err := Open(s.Path, opts)
		if err != nil {
			for _, x := range opened {
				_ = x.Source.Close()
			}
			return nil, fmt.Errorf("open mount %s (%s): %w", s.Label, filepath.Base(s.Path), err)
		}
		s.Source = src
		if s.Label == "" {
			s.Label = filepath.Base(s.Path)
		}
		opened = append(opened, s)
	}
	if len(opened) == 0 {
		return nil, fmt.Errorf("archive: no usable mounts")
	}
	return OpenMulti(name, opened)
}
