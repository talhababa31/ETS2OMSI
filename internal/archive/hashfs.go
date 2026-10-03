package archive

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func openHashFS(input string, opts OpenOptions) (Source, error) {
	helper, kind := findHelper(opts.HelperPaths)
	if helper == "" {
		return nil, fmt.Errorf("%w. Run tools\\setup_scs_packer.ps1 or set ETS2OMSI_SCS_HELPER", ErrHelperMissing)
	}
	rootBase := opts.TempRoot
	if rootBase == "" {
		rootBase = os.TempDir()
	}
	tmp, err := os.MkdirTemp(rootBase, "ets2omsi-scs-")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var cmd *exec.Cmd
	if kind == "scs_tool" {
		cmd = exec.CommandContext(ctx, helper, "extract", input, tmp)
	} else {
		cmd = exec.CommandContext(ctx, helper, "extract", input, "-root", tmp)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tmp)
		return nil, fmt.Errorf("%s extraction failed: %w\n%s", kind, err, string(out))
	}
	d, err := openDir(tmp, true)
	if err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	d.name = filepath.Base(input)
	return &hashWrapped{dirSource: d}, nil
}

type hashWrapped struct{ *dirSource }

func (h *hashWrapped) Kind() Kind { return KindHashFS }

func findHelper(extra []string) (string, string) {
	candidates := append([]string{}, extra...)
	if e := os.Getenv("ETS2OMSI_SCS_HELPER"); e != "" {
		candidates = append(candidates, e)
	}
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	names := []string{"scs_tool.exe", "scs_packer.exe"}
	if runtime.GOOS != "windows" {
		names = []string{"scs_tool", "scs_packer"}
	}
	for _, base := range []string{filepath.Join(exeDir, "tools"), "tools", "."} {
		for _, n := range names {
			candidates = append(candidates, filepath.Join(base, n))
		}
	}
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			candidates = append(candidates, p)
		}
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		a, _ := filepath.Abs(c)
		if seen[strings.ToLower(a)] {
			continue
		}
		seen[strings.ToLower(a)] = true
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			bn := strings.ToLower(filepath.Base(c))
			if strings.Contains(bn, "scs_tool") {
				return c, "scs_tool"
			}
			if strings.Contains(bn, "scs_packer") || strings.Contains(bn, "scs_extractor") {
				return c, "scs_packer"
			}
		}
	}
	return "", ""
}

type HelperStatus struct {
	Found bool   `json:"found"`
	Path  string `json:"path,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

func ToolStatus(extra []string) HelperStatus {
	p, k := findHelper(extra)
	return HelperStatus{Found: p != "", Path: p, Kind: k}
}
