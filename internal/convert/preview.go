package convert

import (
	"context"
	"fmt"
	"os"
	"strings"

	"ets2omsi/internal/pixbridge"
	"ets2omsi/internal/scanner"
	"ets2omsi/internal/scene"
)

type PreviewMaterial struct {
	Name  string     `json:"name"`
	Class string     `json:"class"`
	Color [3]float64 `json:"color"`
}
type PreviewData struct {
	VehicleID         string            `json:"vehicle_id"`
	Name              string            `json:"name"`
	Mode              string            `json:"mode"`
	Positions         []float32         `json:"positions"`
	Normals           []float32         `json:"normals"`
	Indices           []uint32          `json:"indices"`
	TriangleMaterials []uint16          `json:"triangle_materials"`
	Materials         []PreviewMaterial `json:"materials"`
	Locators          []scene.Locator   `json:"locators,omitempty"`
	Width             float64           `json:"width"`
	Length            float64           `json:"length"`
	Height            float64           `json:"height"`
	TrianglesOriginal int               `json:"triangles_original"`
	TrianglesPreview  int               `json:"triangles_preview"`
	WheelVisuals      int               `json:"wheel_visuals,omitempty"`
	Note              string            `json:"note"`
}

func Preview(ctx context.Context, v scanner.Vehicle, mounts []string, exe string) (PreviewData, error) {
	return PreviewMode(ctx, v, mounts, exe, "source")
}

// PreviewMode creates a non-destructive browser preview. "source" shows the selected
// ETS2 gameplay model after variant filtering. "final" additionally composes explicit
// ETS2 wheel accessories exactly as the conversion pipeline does. It is still a
// diagnostic WebGL preview, not an emulation of OMSI's DirectX renderer.
func PreviewMode(ctx context.Context, v scanner.Vehicle, mounts []string, exe, mode string) (PreviewData, error) {
	if len(v.Models) == 0 {
		return PreviewData{}, fmt.Errorf("no PMD model resolved")
	}
	exe = pixbridge.Find(exe)
	if exe == "" {
		return PreviewData{}, fmt.Errorf("ConverterPIX is not installed")
	}
	work, e := os.MkdirTemp("", "ets2omsi-preview-")
	if e != nil {
		return PreviewData{}, e
	}
	defer os.RemoveAll(work)
	assets := conversionAssets(v)
	if len(assets) == 0 {
		return PreviewData{}, fmt.Errorf("no convertible body model resolved")
	}
	main := assets[0]
	px, _, e := pixbridge.ConvertCached(ctx, exe, mounts, main.Path, pixbridge.DefaultCacheRoot())
	if e != nil {
		return PreviewData{}, e
	}
	sc, _, e := parseSceneVariant(px, main.Variant)
	if e != nil {
		return PreviewData{}, e
	}
	original := len(sc.Triangles)
	visuals := 0
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "final" {
		mode = "source"
	}
	b := sc.Bounds()
	sc.Translate(-(b.Min.X+b.Max.X)/2, -(b.Min.Y+b.Max.Y)/2, 0)
	if mode == "final" && len(v.WheelAttachments) > 0 {
		wv, _, radii, _, _ := resolveWheelVisuals(ctx, exe, mounts, v, sc, work, pixbridge.DefaultCacheRoot())
		wheels, _ := detectWheels(sc, radii)
		g := groundPlane(sc, wheels)
		sc.Translate(0, 0, -g)
		for i := range wv {
			wv[i].Scene.Translate(0, 0, -g)
			sc.AppendTranslated(wv[i].Scene, 0, 0, 0)
		}
		visuals = len(wv)
	} else {
		// Source preview remains simple but upright in internal coordinates.
		bb := sc.Bounds()
		sc.Translate(0, 0, -bb.Min.Z)
	}
	p := previewFromScene(v, sc)
	p.Mode = mode
	p.TrianglesOriginal = original
	p.WheelVisuals = visuals
	if mode == "final" {
		p.Note = "FINAL geometry preview: Passat-calibrated ground/origin + separate wheel accessory composition. Material colors are diagnostic; exported O3D uses exact texture references."
	} else {
		p.Note = "SOURCE geometry preview generated from ConverterPIX PIM after the selected ETS2 variant filter."
	}
	return p, nil
}
func previewFromScene(v scanner.Vehicle, sc scene.Scene) PreviewData {
	p := PreviewData{VehicleID: v.ID, Name: v.DisplayName, Locators: sc.Locators, TrianglesOriginal: len(sc.Triangles)}
	b := sc.Bounds()
	p.Width, p.Length, p.Height = b.Width(), b.Length(), b.Height()
	for _, m := range sc.Materials {
		cl := materialClass(m)
		p.Materials = append(p.Materials, PreviewMaterial{Name: m.Alias, Class: cl, Color: classColor(cl)})
	}
	if len(p.Materials) == 0 {
		p.Materials = append(p.Materials, PreviewMaterial{Name: "default", Class: "body", Color: classColor("body")})
	}
	maxTri := 30000
	step := 1
	if len(sc.Triangles) > maxTri {
		step = (len(sc.Triangles) + maxTri - 1) / maxTri
	}
	remap := map[int]uint32{}
	add := func(i int) uint32 {
		if x, ok := remap[i]; ok {
			return x
		}
		x := uint32(len(remap))
		remap[i] = x
		vv := sc.Vertices[i]
		p.Positions = append(p.Positions, float32(vv.Position.X), float32(vv.Position.Y), float32(vv.Position.Z))
		p.Normals = append(p.Normals, float32(vv.Normal.X), float32(vv.Normal.Y), float32(vv.Normal.Z))
		return x
	}
	for i := 0; i < len(sc.Triangles); i += step {
		t := sc.Triangles[i]
		if t.A < 0 || t.B < 0 || t.C < 0 || t.A >= len(sc.Vertices) || t.B >= len(sc.Vertices) || t.C >= len(sc.Vertices) {
			continue
		}
		p.Indices = append(p.Indices, add(t.A), add(t.B), add(t.C))
		mat := t.Material
		if mat < 0 || mat >= len(p.Materials) {
			mat = 0
		}
		p.TriangleMaterials = append(p.TriangleMaterials, uint16(mat))
	}
	p.TrianglesPreview = len(p.TriangleMaterials)
	return p
}
func classColor(c string) [3]float64 {
	switch c {
	case "glass":
		return [3]float64{.22, .35, .46}
	case "light":
		return [3]float64{1, .7, .2}
	case "chrome":
		return [3]float64{.72, .74, .78}
	case "rubber":
		return [3]float64{.10, .10, .11}
	case "paint":
		return [3]float64{.54, .58, .65}
	default:
		return [3]float64{.62, .64, .68}
	}
}
