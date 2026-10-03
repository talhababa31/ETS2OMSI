package main

import (
	"context"
	"ets2omsi/internal/app"
	conv "ets2omsi/internal/convert"
	"ets2omsi/internal/extract"
	"ets2omsi/internal/pixbridge"
	"ets2omsi/internal/report"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	input := flag.String("input", "", "traffic .scs/.zip package")
	jsonOut := flag.String("json", "", "write JSON scan report")
	convertIDs := flag.String("convert", "", "comma-separated car IDs or 'ready'/'all'")
	extractIDs := flag.String("extract", "", "comma-separated car IDs or 'ready'/'all'")
	out := flag.String("out", "output", "output folder")
	pix := flag.String("converterpix", "", "optional ConverterPIX executable override")
	flag.Parse()
	if strings.TrimSpace(*input) == "" {
		fmt.Fprintln(os.Stderr, "usage: ETS2OMSI_V2_2_MATERIAL_PHYSICS_CLI.exe -input traffic_pack.scs [-convert ready -out output]")
		os.Exit(2)
	}
	p, err := app.ScanProject(app.ScanOptions{PackagePath: *input})
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan failed:", err)
		os.Exit(1)
	}
	fmt.Println(report.Summary(p.Report))
	if *jsonOut != "" {
		if err := write(*jsonOut, func(f *os.File) error { return report.WriteJSON(f, p.Report) }); err != nil {
			panic(err)
		}
	}
	idsFor := func(spec string) []string {
		spec = strings.TrimSpace(strings.ToLower(spec))
		if spec == "" {
			return nil
		}
		if spec == "all" || spec == "ready" {
			r := []string{}
			for _, v := range p.Report.Vehicles {
				if spec == "all" || v.Readiness == "ready" {
					r = append(r, v.ID)
				}
			}
			return r
		}
		out := []string{}
		for _, x := range strings.Split(spec, ",") {
			if x = strings.TrimSpace(x); x != "" {
				out = append(out, x)
			}
		}
		return out
	}
	if ids := idsFor(*extractIDs); len(ids) > 0 {
		for _, id := range ids {
			v, ok := app.FindVehicle(p.Report, id)
			if !ok {
				fmt.Fprintln(os.Stderr, "unknown car:", id)
				continue
			}
			r, e := extract.Vehicle(*input, v, *out)
			if e != nil {
				fmt.Fprintln(os.Stderr, "extract failed:", v.DisplayName, e)
			} else {
				fmt.Println("EXTRACTED", v.DisplayName, "->", r.Output)
			}
		}
	}
	ids := idsFor(*convertIDs)
	if len(ids) == 0 {
		return
	}
	exe := pixbridge.Find(*pix)
	if exe == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		st, e := pixbridge.Download(ctx, "")
		if e != nil {
			fmt.Fprintln(os.Stderr, "3D model engine install failed:", e)
			os.Exit(1)
		}
		exe = st.Path
	}
	failed := 0
	for _, id := range ids {
		v, ok := app.FindVehicle(p.Report, id)
		if !ok {
			fmt.Fprintln(os.Stderr, "unknown car:", id)
			failed++
			continue
		}
		if v.Readiness != "ready" {
			fmt.Fprintln(os.Stderr, "not ready:", v.DisplayName)
			failed++
			continue
		}
		fmt.Printf("Converting %s...\n", v.DisplayName)
		r, e := conv.Vehicle(context.Background(), conv.Options{Vehicle: v, MountPaths: []string{*input}, ConverterPIX: exe, OutputRoot: *out})
		if e != nil {
			fmt.Fprintf(os.Stderr, "  FAILED [%s]: %v\n", r.Stage, e)
			failed++
			continue
		}
		fmt.Printf("  OMSI READY -> %s (%s)\n", r.Output, r.Status)
	}
	if failed > 0 {
		os.Exit(1)
	}
}
func write(p string, fn func(*os.File) error) error {
	if d := filepath.Dir(p); d != "." {
		_ = os.MkdirAll(d, 0755)
	}
	f, e := os.Create(p)
	if e != nil {
		return e
	}
	defer f.Close()
	return fn(f)
}
