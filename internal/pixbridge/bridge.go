package pixbridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const DownloadURL = "https://raw.githubusercontent.com/mwl4/ConverterPIX/master/bin/win_x64/converter_pix.exe"
const ProjectURL = "https://github.com/mwl4/ConverterPIX"

type Status struct {
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Message string `json:"message,omitempty"`
}
type Result struct {
	ModelPath  string `json:"model_path"`
	WorkDir    string `json:"work_dir"`
	PIM        string `json:"pim"`
	PIT        string `json:"pit,omitempty"`
	Output     string `json:"output,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

func Find(extra string) string {
	candidates := []string{}
	if strings.TrimSpace(extra) != "" {
		candidates = append(candidates, extra)
	}
	if v := strings.TrimSpace(os.Getenv("ETS2OMSI_CONVERTER_PIX")); v != "" {
		candidates = append(candidates, v)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "tools", "converter_pix.exe"))
	}
	candidates = append(candidates, filepath.Join("tools", "converter_pix.exe"), "converter_pix.exe")
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			a, _ := filepath.Abs(c)
			return a
		}
	}
	if p, err := exec.LookPath("converter_pix"); err == nil {
		return p
	}
	if p, err := exec.LookPath("converter_pix.exe"); err == nil {
		return p
	}
	return ""
}
func ToolStatus(extra string) Status {
	p := Find(extra)
	if p == "" {
		return Status{Message: "ConverterPIX not installed"}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Status{Path: p, Message: err.Error()}
	}
	h := sha256.Sum256(b)
	return Status{Found: true, Path: p, SHA256: hex.EncodeToString(h[:]), Message: "Ready"}
}

func Download(ctx context.Context, dest string) (Status, error) {
	if runtime.GOOS != "windows" && strings.HasSuffix(strings.ToLower(dest), ".exe") { /* building packages on non-Windows is okay */
	}
	if dest == "" {
		if exe, err := os.Executable(); err == nil {
			dest = filepath.Join(filepath.Dir(exe), "tools", "converter_pix.exe")
		} else {
			dest = filepath.Join("tools", "converter_pix.exe")
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return Status{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DownloadURL, nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Status{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Status{}, fmt.Errorf("ConverterPIX download HTTP %d", resp.StatusCode)
	}
	tmp := dest + ".download"
	f, err := os.Create(tmp)
	if err != nil {
		return Status{}, err
	}
	h := sha256.New()
	_, cpErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 64<<20))
	clErr := f.Close()
	if cpErr != nil {
		os.Remove(tmp)
		return Status{}, cpErr
	}
	if clErr != nil {
		os.Remove(tmp)
		return Status{}, clErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		return Status{}, err
	}
	return Status{Found: true, Path: dest, SHA256: hex.EncodeToString(h.Sum(nil)), Message: "Downloaded"}, nil
}

// Convert invokes ConverterPIX in single-model mode. MountPaths must be ordered
// low -> high priority (base first, selected mod last), matching the tool's VFS.
func Convert(ctx context.Context, exe string, mountPaths []string, modelPath, exportRoot string) (Result, error) {
	if exe == "" {
		exe = Find("")
	}
	if exe == "" {
		return Result{}, errors.New("ConverterPIX is not installed")
	}
	if len(mountPaths) == 0 {
		return Result{}, errors.New("no ETS2 mount paths supplied")
	}
	if err := os.MkdirAll(exportRoot, 0755); err != nil {
		return Result{}, err
	}
	model := strings.ReplaceAll(strings.TrimSpace(modelPath), "\\", "/")
	model = strings.TrimSuffix(model, filepath.Ext(model))
	if !strings.HasPrefix(model, "/") {
		model = "/" + model
	}
	args := []string{}
	for _, p := range mountPaths {
		if strings.TrimSpace(p) != "" {
			args = append(args, "-b", p)
		}
	}
	args = append(args, "-e", exportRoot, "-m", model)
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, exe, args...)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	res := Result{ModelPath: modelPath, WorkDir: exportRoot, Output: string(out), DurationMS: time.Since(start).Milliseconds()}
	if cctx.Err() != nil {
		return res, fmt.Errorf("ConverterPIX timeout: %w", cctx.Err())
	}
	if err != nil {
		return res, fmt.Errorf("ConverterPIX failed: %w\n%s", err, trimOutput(string(out)))
	}
	pim, pit := findPIX(exportRoot, filepath.Base(model))
	res.PIM = pim
	res.PIT = pit
	if pim == "" {
		return res, fmt.Errorf("ConverterPIX completed but no PIM was found for %s\n%s", modelPath, trimOutput(string(out)))
	}
	return res, nil
}

// ConvertTextureObject resolves one exact SCS TOBJ through the already-mounted
// package and asks ConverterPIX to export the referenced image beside it.
// This keeps V2 package-local: no ETS2 installation or base/DLC mount is required.
func ConvertTextureObject(ctx context.Context, exe string, mountPaths []string, tobjPath, exportRoot string) (Result, error) {
	if exe == "" {
		exe = Find("")
	}
	if exe == "" {
		return Result{}, errors.New("ConverterPIX is not installed")
	}
	if len(mountPaths) == 0 {
		return Result{}, errors.New("no SCS mount paths supplied")
	}
	if err := os.MkdirAll(exportRoot, 0755); err != nil {
		return Result{}, err
	}
	path := strings.ReplaceAll(strings.TrimSpace(tobjPath), "\\", "/")
	if path == "" {
		return Result{}, errors.New("empty TOBJ path")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	args := []string{}
	for _, p := range mountPaths {
		if strings.TrimSpace(p) != "" {
			args = append(args, "-b", p)
		}
	}
	args = append(args, "-e", exportRoot, "-t", path)
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, exe, args...)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	r := Result{ModelPath: path, WorkDir: exportRoot, Output: string(out), DurationMS: time.Since(start).Milliseconds()}
	if cctx.Err() != nil {
		return r, fmt.Errorf("ConverterPIX TOBJ timeout: %w", cctx.Err())
	}
	if err != nil {
		return r, fmt.Errorf("ConverterPIX TOBJ failed for %s: %w\n%s", path, err, trimOutput(string(out)))
	}
	return r, nil
}

// ExtractFile copies one exact file from the mounted SCS virtual filesystem.
func ExtractFile(ctx context.Context, exe string, mountPaths []string, filePath, exportRoot string) (Result, error) {
	if exe == "" {
		exe = Find("")
	}
	if exe == "" {
		return Result{}, errors.New("ConverterPIX is not installed")
	}
	if len(mountPaths) == 0 {
		return Result{}, errors.New("no SCS mount paths supplied")
	}
	if err := os.MkdirAll(exportRoot, 0755); err != nil {
		return Result{}, err
	}
	path := strings.ReplaceAll(strings.TrimSpace(filePath), "\\", "/")
	if path == "" {
		return Result{}, errors.New("empty extract path")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	args := []string{}
	for _, p := range mountPaths {
		if strings.TrimSpace(p) != "" {
			args = append(args, "-b", p)
		}
	}
	args = append(args, "-e", exportRoot, "--extract-file", path)
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, exe, args...)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	r := Result{ModelPath: path, WorkDir: exportRoot, Output: string(out), DurationMS: time.Since(start).Milliseconds()}
	if cctx.Err() != nil {
		return r, fmt.Errorf("ConverterPIX extract timeout: %w", cctx.Err())
	}
	if err != nil {
		return r, fmt.Errorf("ConverterPIX extract failed for %s: %w\n%s", path, err, trimOutput(string(out)))
	}
	return r, nil
}

func findPIX(root, stem string) (string, string) {
	type hit struct {
		p     string
		score int
	}
	hs := []hit{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".pim" && ext != ".pit" {
			return nil
		}
		score := 0
		if strings.EqualFold(strings.TrimSuffix(filepath.Base(p), ext), stem) {
			score = 10
		}
		hs = append(hs, hit{p, score})
		return nil
	})
	sort.Slice(hs, func(i, j int) bool { return hs[i].score > hs[j].score })
	var pim, pit string
	for _, h := range hs {
		switch strings.ToLower(filepath.Ext(h.p)) {
		case ".pim":
			if pim == "" {
				pim = h.p
			}
		case ".pit":
			if pit == "" {
				pit = h.p
			}
		}
	}
	return pim, pit
}
func trimOutput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 5000 {
		return s[len(s)-5000:]
	}
	return s
}

// DefaultCacheRoot returns a persistent per-user cache for ConverterPIX output.
// The cache contains only generated interoperability files; deleting it is safe.
func DefaultCacheRoot() string {
	if d, err := os.UserCacheDir(); err == nil && strings.TrimSpace(d) != "" {
		return filepath.Join(d, "ETS2OMSI", "pix")
	}
	return filepath.Join(os.TempDir(), "ETS2OMSI-cache", "pix")
}

// ConvertCached reuses ConverterPIX output when the converter binary, source mount
// files and requested model have not changed. This matters for large traffic packs
// where many vehicles share wheels/materials/LODs.
func ConvertCached(ctx context.Context, exe string, mountPaths []string, modelPath, cacheRoot string) (Result, bool, error) {
	if strings.TrimSpace(cacheRoot) == "" {
		cacheRoot = DefaultCacheRoot()
	}
	if exe == "" {
		exe = Find("")
	}
	if exe == "" {
		return Result{}, false, errors.New("ConverterPIX is not installed")
	}
	key := cacheKey(exe, mountPaths, modelPath)
	final := filepath.Join(cacheRoot, key[:2], key)
	meta := filepath.Join(final, "ets2omsi_cache.json")
	if b, err := os.ReadFile(meta); err == nil {
		var r Result
		if json.Unmarshal(b, &r) == nil && r.PIM != "" {
			if st, e := os.Stat(r.PIM); e == nil && !st.IsDir() {
				r.WorkDir = final
				return r, true, nil
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(final), 0755); err != nil {
		return Result{}, false, err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(final), ".build-")
	if err != nil {
		return Result{}, false, err
	}
	defer os.RemoveAll(tmp)
	r, err := Convert(ctx, exe, mountPaths, modelPath, tmp)
	if err != nil {
		return r, false, err
	}
	_ = os.RemoveAll(final)
	if err := os.Rename(tmp, final); err != nil {
		return r, false, err
	}
	// Paths in Result pointed at the temporary directory; remap them after rename.
	remap := func(p string) string {
		if p == "" {
			return ""
		}
		rel, e := filepath.Rel(tmp, p)
		if e != nil {
			return p
		}
		return filepath.Join(final, rel)
	}
	r.PIM, r.PIT, r.WorkDir = remap(r.PIM), remap(r.PIT), final
	b, _ := json.MarshalIndent(r, "", "  ")
	_ = os.WriteFile(meta, b, 0644)
	return r, false, nil
}

func cacheKey(exe string, mounts []string, model string) string {
	h := sha256.New()
	_, _ = io.WriteString(h, strings.ToLower(filepath.Clean(exe))+"\n")
	if st, err := os.Stat(exe); err == nil {
		_, _ = io.WriteString(h, fmt.Sprintf("%d|%d\n", st.Size(), st.ModTime().UnixNano()))
	}
	for _, p := range mounts {
		a, _ := filepath.Abs(p)
		_, _ = io.WriteString(h, strings.ToLower(filepath.Clean(a)))
		if st, err := os.Stat(a); err == nil {
			_, _ = io.WriteString(h, fmt.Sprintf("|%d|%d", st.Size(), st.ModTime().UnixNano()))
		}
		_, _ = io.WriteString(h, "\n")
	}
	_, _ = io.WriteString(h, strings.ToLower(strings.ReplaceAll(model, "\\", "/")))
	return hex.EncodeToString(h.Sum(nil))
}
