package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
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

func main() {
	if f, err := os.OpenFile("ETS2OMSI_V2_2_MATERIAL_PHYSICS.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		log.SetOutput(f)
		defer f.Close()
	}
	sub, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/status", status)
	mux.HandleFunc("/api/pick", pickAPI)
	mux.HandleFunc("/api/pick-folder", pickFolderAPI)
	mux.HandleFunc("/api/scan", scanAPI)
	mux.HandleFunc("/api/convert", convertAPI)
	mux.HandleFunc("/api/extract", extractAPI)
	mux.HandleFunc("/api/preview", previewAPI)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	url := "http://" + ln.Addr().String() + "/"
	log.Println("listening", url)
	go func() { time.Sleep(180 * time.Millisecond); _ = openBrowser(url) }()
	if err := http.Serve(ln, mux); err != nil {
		log.Println(err)
	}
}

func status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"version": "V2.2 Material + Physics", "platform": runtime.GOOS,
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
