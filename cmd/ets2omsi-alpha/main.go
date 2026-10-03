package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"ets2omsi/internal/app"
	"ets2omsi/internal/archive"
	conv "ets2omsi/internal/convert"
	"ets2omsi/internal/extract"
	"ets2omsi/internal/pixbridge"
)

//go:embed web/*
var webFS embed.FS

type appState struct {
	sync.RWMutex
	project app.Project
	ok      bool
}

var current appState

const appVersion = "V2.2 Material + Physics"

var (
	logPath     string
	instanceURL string
	quitOnce    sync.Once
	quitCh      = make(chan struct{})
)

// requestQuit asks the server loop to shut down; safe to call many times.
func requestQuit() { quitOnce.Do(func() { close(quitCh) }) }

// appDataDir is the absolute per-user data directory (%LocalAppData%\ETS2OMSI on Windows).
func appDataDir() string {
	if d, err := os.UserCacheDir(); err == nil && d != "" {
		return filepath.Join(d, "ETS2OMSI")
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return os.TempDir()
}

func main() {
	noBrowser := flag.Bool("no-browser", false, "do not open the web UI in a browser")
	noWindow := flag.Bool("no-window", false, "do not open the program window; serve the UI for a browser instead")
	flag.Parse()

	dataDir := appDataDir()
	_ = os.MkdirAll(dataDir, 0755)
	logPath = filepath.Join(dataDir, "ETS2OMSI_V2_2_MATERIAL_PHYSICS.log")
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		log.SetOutput(io.MultiWriter(f, os.Stderr))
		defer f.Close()
	}

	// Single instance: if a previous copy is still serving, reuse it instead
	// of leaving another orphan server process behind.
	instFile := filepath.Join(dataDir, "instance.json")
	if url := runningInstance(instFile); url != "" {
		log.Println("already running at", url)
		fmt.Println("ETS2OMSI zaten çalışıyor:", url)
		if *noWindow {
			if !*noBrowser {
				_ = openBrowser(url)
			}
		} else {
			platformShowInfo("ETS2OMSI zaten açık. Açık olan pencereyi kullan (görev çubuğuna bak).")
		}
		return
	}

	sub, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/status", status)
	mux.HandleFunc("/api/quit", quitAPI)
	mux.HandleFunc("/api/pick", pickAPI)
	mux.HandleFunc("/api/pick-folder", pickFolderAPI)
	mux.HandleFunc("/api/scan", scanAPI)
	mux.HandleFunc("/api/convert", convertAPI)
	mux.HandleFunc("/api/extract", extractAPI)
	mux.HandleFunc("/api/preview", previewAPI)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Println(err)
		platformShowError("ETS2OMSI başlatılamadı: " + err.Error())
		os.Exit(1)
	}
	instanceURL = "http://" + ln.Addr().String() + "/"
	log.Println("listening", instanceURL, "log:", logPath)
	fmt.Println("ETS2OMSI " + appVersion + " çalışıyor: " + instanceURL)
	fmt.Println("Kapatmak için arayüzdeki 'Kapat' düğmesini kullan veya Ctrl+C.")
	writeInstance(instFile, instanceURL)
	defer removeInstance(instFile, instanceURL)

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Println(err)
		}
		requestQuit()
	}()
	// Normal Windows start: the UI opens in its own program window. The
	// browser is only a fallback (no WebView2 runtime, or --no-window).
	if !*noWindow && platformRunWindow(instanceURL, dataDir, quitCh) {
		requestQuit()
	} else {
		if !*noBrowser {
			go func() { time.Sleep(180 * time.Millisecond); _ = openBrowser(instanceURL) }()
		}
		if !*noWindow {
			go platformStatusWindow(instanceURL, logPath, func() { _ = openBrowser(instanceURL) }, requestQuit)
		}
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		select {
		case <-quitCh:
		case <-sig:
		}
	}
	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func runningInstance(file string) string {
	b, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	var inst struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(b, &inst) != nil || !strings.HasPrefix(inst.URL, "http://127.0.0.1:") {
		return ""
	}
	c := http.Client{Timeout: 800 * time.Millisecond}
	resp, err := c.Get(inst.URL + "api/status")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var st map[string]any
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&st) != nil || st["app"] != "ETS2OMSI" {
		return ""
	}
	return inst.URL
}

func writeInstance(file, url string) {
	b, _ := json.Marshal(map[string]any{"url": url, "pid": os.Getpid()})
	_ = os.WriteFile(file, b, 0644)
}

func removeInstance(file, url string) {
	if b, err := os.ReadFile(file); err == nil && strings.Contains(string(b), url) {
		_ = os.Remove(file)
	}
}

func quitAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
	go func() { time.Sleep(150 * time.Millisecond); requestQuit() }()
}

func status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"app": "ETS2OMSI", "version": appVersion, "platform": runtime.GOOS,
		"url": instanceURL, "log": logPath,
		"phase": "SCS → AI CAR → PMD/PMG → OMSI O3D",
		"mode":  "SCS_ONLY", "converterpix": pixbridge.ToolStatus(""), "scs_helper": archive.ToolStatus(nil),
	})
}
func pickAPI(w http.ResponseWriter, r *http.Request) {
	p, e := platformPickSCS()
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"path": p})
}
func pickFolderAPI(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	if title == "" {
		title = "Output klasörü seç"
	}
	p, e := platformPickFolder(title)
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"path": p})
}
func scanAPI(w http.ResponseWriter, r *http.Request) {
	var req app.ScanOptions
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	if strings.TrimSpace(req.PackagePath) == "" {
		http.Error(w, "SCS path is required", 400)
		return
	}
	p, e := app.ScanProject(req)
	if e != nil {
		writeJSONStatus(w, 500, map[string]any{"error": e.Error()})
		return
	}
	current.Lock()
	current.project = p
	current.ok = true
	current.Unlock()
	writeJSON(w, p)
}
func ensureConverterPIX(ctx context.Context) (string, error) {
	if p := pixbridge.Find(""); p != "" {
		return p, nil
	}
	dest := ""
	if exe, e := os.Executable(); e == nil {
		dest = filepath.Join(filepath.Dir(exe), "tools", "converter_pix.exe")
	}
	st, e := pixbridge.Download(ctx, dest)
	if e != nil {
		return "", fmt.Errorf("3D model engine could not be installed automatically: %w", e)
	}
	if !st.Found || st.Path == "" {
		return "", fmt.Errorf("3D model engine installation failed")
	}
	return st.Path, nil
}

func convertAPI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs        []string `json:"ids"`
		OutputRoot string   `json:"output_root"`
	}
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	current.RLock()
	p := current.project
	ok := current.ok
	current.RUnlock()
	if !ok {
		http.Error(w, "önce SCS paketini tara", 409)
		return
	}
	if len(req.IDs) == 0 {
		http.Error(w, "en az bir araç seç", 400)
		return
	}
	if req.OutputRoot == "" {
		req.OutputRoot = "output"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	pix, err := ensureConverterPIX(ctx)
	if err != nil {
		writeJSONStatus(w, 500, map[string]any{"error": err.Error(), "stage": "3D model engine"})
		return
	}
	type item struct {
		ID     string      `json:"id"`
		Name   string      `json:"name"`
		Report conv.Report `json:"report"`
		Error  string      `json:"error,omitempty"`
	}
	out := []item{}
	for _, id := range req.IDs {
		v, found := app.FindVehicle(p.Report, id)
		if !found {
			out = append(out, item{ID: id, Error: "araç mevcut taramada bulunamadı"})
			continue
		}
		rp, e := conv.Vehicle(ctx, conv.Options{Vehicle: v, MountPaths: []string{p.Options.PackagePath}, ConverterPIX: pix, OutputRoot: req.OutputRoot, StrictFidelity: false})
		it := item{ID: id, Name: v.DisplayName, Report: rp}
		if e != nil {
			it.Error = e.Error()
		}
		out = append(out, it)
	}
	summary := map[string]int{"total": len(out), "pass": 0, "warn": 0, "fail": 0}
	for _, it := range out {
		if it.Error != "" || it.Report.Status == "fail" {
			summary["fail"]++
		} else if it.Report.Status == "warn" {
			summary["warn"]++
		} else {
			summary["pass"]++
		}
	}
	writeJSON(w, map[string]any{"items": out, "summary": summary})
}

func extractAPI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs        []string `json:"ids"`
		OutputRoot string   `json:"output_root"`
	}
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	current.RLock()
	p := current.project
	ok := current.ok
	current.RUnlock()
	if !ok {
		http.Error(w, "önce SCS paketini tara", 409)
		return
	}
	if len(req.IDs) == 0 {
		http.Error(w, "en az bir araç seç", 400)
		return
	}
	if req.OutputRoot == "" {
		req.OutputRoot = "extracted"
	}
	type item struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Output    string `json:"output,omitempty"`
		FileCount int    `json:"file_count,omitempty"`
		Error     string `json:"error,omitempty"`
	}
	out := []item{}
	for _, id := range req.IDs {
		v, found := app.FindVehicle(p.Report, id)
		if !found {
			out = append(out, item{ID: id, Error: "araç bulunamadı"})
			continue
		}
		rr, e := extract.Vehicle(p.Options.PackagePath, v, req.OutputRoot)
		it := item{ID: id, Name: v.DisplayName, Output: rr.Output, FileCount: len(rr.Files)}
		if e != nil {
			it.Error = e.Error()
		}
		out = append(out, it)
	}
	writeJSON(w, map[string]any{"items": out})
}

func previewAPI(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	current.RLock()
	p := current.project
	ok := current.ok
	current.RUnlock()
	if !ok {
		http.Error(w, "önce SCS paketini tara", 409)
		return
	}
	v, found := app.FindVehicle(p.Report, id)
	if !found {
		http.Error(w, "araç bulunamadı", 404)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	pix, e := ensureConverterPIX(ctx)
	if e != nil {
		writeJSONStatus(w, 500, map[string]any{"error": e.Error()})
		return
	}
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	d, e := conv.PreviewMode(ctx, v, []string{p.Options.PackagePath}, pix, mode)
	if e != nil {
		writeJSONStatus(w, 500, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, d)
}

func writeJSON(w http.ResponseWriter, v any) { writeJSONStatus(w, 200, v) }
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func openBrowser(url string) error {
	if runtime.GOOS == "windows" {
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}
