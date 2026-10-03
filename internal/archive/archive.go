package archive

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Kind string

const (
	KindDirectory Kind = "directory"
	KindZIP       Kind = "zip"
	KindHashFS    Kind = "hashfs"
)

type FileInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type Source interface {
	Name() string
	Kind() Kind
	List() []FileInfo
	Read(path string) ([]byte, error)
	Exists(path string) bool
	Close() error
}

type OpenOptions struct {
	HelperPaths []string
	TempRoot    string
}

var ErrHelperMissing = errors.New("HashFS archive detected but no SCS extraction helper is available")

func Open(input string, opts OpenOptions) (Source, error) {
	st, err := os.Stat(input)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return openDir(input, false)
	}
	f, err := os.Open(input)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	head := make([]byte, 8)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if len(head) >= 2 && bytes.Equal(head[:2], []byte{'P', 'K'}) {
		return openZIP(input)
	}
	if len(head) >= 4 && bytes.Equal(head[:4], []byte{'S', 'C', 'S', '#'}) {
		return openHashFS(input, opts)
	}
	// Some mods use .scs extension but are regular ZIPs with unusual preamble. archive/zip can still test them.
	if strings.EqualFold(filepath.Ext(input), ".scs") || strings.EqualFold(filepath.Ext(input), ".zip") {
		if z, zerr := openZIP(input); zerr == nil {
			return z, nil
		}
	}
	return nil, fmt.Errorf("unsupported package format: %s", input)
}

func normalize(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s == "" || s == "." {
			continue
		}
		if s == ".." {
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
			continue
		}
		out = append(out, s)
	}
	return "/" + strings.Join(out, "/")
}
