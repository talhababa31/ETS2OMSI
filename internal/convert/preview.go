package convert

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"ets2omsi/internal/pixbridge"
	"ets2omsi/internal/scanner"
	"ets2omsi/internal/scene"
)

type PreviewMaterial struct {
	Name    string     `json:"name"`
	Class   string     `json:"class"`
	Color   [3]float64 `json:"color"`
	Texture string     `json:"texture,omitempty"` // file name inside the preview texture set
	Alpha   bool       `json:"alpha,omitempty"`
	Ref     string     `json:"ref,omitempty"`
	Image   string     `json:"image,omitempty"`
	Status  string     `json:"status,omitempty"`
	Reason  string     `json:"reason,omitempty"`
	Model   string     `json:"model,omitempty"`
}
type PreviewData struct {
	VehicleID         string            `json:"vehicle_id"`
	Name              string            `json:"name"`
	Mode              string            `json:"mode"`
	Positions         []float32         `json:"positions"`
	Normals           []float32         `json:"normals"`
	UVs               []float32         `json:"uvs"`
	TextureSet        string            `json:"texture_set,omitempty"`
	Class             string            `json:"class,omitempty"`
	ClassBasis        string            `json:"class_basis,omitempty"`
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
	set := PreviewTextureSetID(v.ID, mode)
	texDir := PreviewTextureDir(set)
	_ = os.RemoveAll(texDir)
	_ = os.MkdirAll(texDir, 0755)
	bodyHints := loadPITMaterials(&sc, px.PIT, vehicleLook(v, main.Look))
	class, classBasis := detectVehicleClass(v.DisplayName+" "+v.ID, sc)
	texRoots := []string{px.WorkDir}
	resolver := newTextureResolver(mounts, texDir)
	defer resolver.Close()
	diags := []MaterialDiag{}
	b := sc.Bounds()
	sc.Translate(-(b.Min.X+b.Max.X)/2, -(b.Min.Y+b.Max.Y)/2, 0)
	if mode == "final" {
		wv, _, radii, _, _ := resolveWheelVisuals(ctx, exe, mounts, v, sc, work, pixbridge.DefaultCacheRoot())
		wheels, _ := detectWheels(sc, radii)
		g := groundPlane(sc, wheels)
		if gc, ok := wheelContactGround(wv); ok {
			g = gc
		}
		sc.Translate(0, 0, -g)
		hints := map[string][]string{}
		mergeHints(hints, bodyHints)
		for i := range wv {
			if wv[i].Hints == nil {
				wv[i].Hints = pitHints(wv[i].Pix.PIT)
			}
			mergeHints(hints, wv[i].Hints)
			texRoots = append(texRoots, wv[i].Pix.WorkDir)
		}
		previewPIXFallback(ctx, exe, mounts, hints, texRoots, work, resolver)
		diags = append(diags, previewMaterials(resolver, &sc, bodyHints, "body")...)
		for i := range wv {
			diags = append(diags, previewMaterials(resolver, &wv[i].Scene, wv[i].Hints, "wheel_"+strings.ToLower(wv[i].Slot))...)
			wv[i].Scene.Translate(0, 0, -g)
			sc.AppendTranslated(wv[i].Scene, 0, 0, 0)
		}
		visuals = len(wv)
	} else {
		// Source preview remains simple but upright in internal coordinates.
		bb := sc.Bounds()
		sc.Translate(0, 0, -bb.Min.Z)
		previewPIXFallback(ctx, exe, mounts, bodyHints, texRoots, work, resolver)
		diags = append(diags, previewMaterials(resolver, &sc, bodyHints, "body")...)
	}
	p := previewFromScene(v, sc)
	if len(diags) == len(p.Materials) {
		for i, d := range diags {
			pm := &p.Materials[i]
			pm.Ref, pm.Image, pm.Status, pm.Reason, pm.Model = d.Ref, d.Image, d.Status, d.Reason, d.Model
			if pm.Status == "missing" || pm.Status == "fallback" {
				pm.Reason += " → yedek: " + pm.Texture
			}
		}
	}
	p.TextureSet = set
	p.Class, p.ClassBasis = class, classBasis
	p.Mode = mode
	p.TrianglesOriginal = original
	p.WheelVisuals = visuals
	if mode == "final" {
		p.Note = "FINAL preview: OMSI ground/origin, separate wheels and the same textures/fallbacks the OMSI export uses."
	} else {
		p.Note = "SOURCE preview: ETS2 body model after the selected variant filter, with the textures the OMSI export uses."
	}
	return p, nil
}
func previewFromScene(v scanner.Vehicle, sc scene.Scene) PreviewData {
	p := PreviewData{VehicleID: v.ID, Name: v.DisplayName, Locators: sc.Locators, TrianglesOriginal: len(sc.Triangles)}
	b := sc.Bounds()
	p.Width, p.Length, p.Height = b.Width(), b.Length(), b.Height()
	for _, m := range sc.Materials {
		cl := materialClass(m)
		p.Materials = append(p.Materials, PreviewMaterial{Name: m.Alias, Class: cl, Color: classColor(cl), Texture: m.Texture, Alpha: m.Alpha})
	}
	if len(p.Materials) == 0 {
		p.Materials = append(p.Materials, PreviewMaterial{Name: "default", Class: "body", Color: classColor("body")})
	}
	// Full detail: skipping triangles punched visible holes into the car.
	maxTri := 400000
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
		p.UVs = append(p.UVs, float32(vv.UV.X), float32(vv.UV.Y))
		return x
	}
	for i := 0; i < len(sc.Triangles); i += step {
		t := sc.Triangles[i]
		if t.A < 0 || t.B < 0 || t.C < 0 || t.A >= len(sc.Vertices) || t.B >= len(sc.Vertices) || t.C >= len(sc.Vertices) {
			continue
		}
		// Keep preview winding consistent with vertex normals, exactly like
		// toO3D() does for the exported body. Without this the WebGL viewer
		// shades ~all faces as back faces and the car renders almost black.
		a, b, c := t.A, t.B, t.C
		if sceneFaceOpposesNormals(sc.Vertices, a, b, c) {
			b, c = c, b
		}
		ia, ib, ic := add(a), add(b), add(c)
		p.Indices = append(p.Indices, ia, ib, ic)
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

// PreviewTextureSetID is a filesystem-safe id for one vehicle preview.
func PreviewTextureSetID(vehicleID, mode string) string {
	return shortHash(vehicleID+"|"+mode) + "_" + mode
}

// PreviewTextureDir is where the preview's resolved textures are kept so the
// UI can request them after the preview call returns.
func PreviewTextureDir(set string) string {
	base := os.TempDir()
	if d, err := os.UserCacheDir(); err == nil && d != "" {
		base = d
	}
	return filepath.Join(base, "ETS2OMSI", "preview", filepath.Base(set))
}

// previewPIXFallback feeds ConverterPIX exports to the resolver when the
// package cannot be read directly (same rule as the conversion).
func previewPIXFallback(ctx context.Context, exe string, mounts []string, hints map[string][]string, roots []string, work string, r *textureResolver) {
	if r.Native() {
		return
	}
	resolveDir := filepath.Join(work, "exact_textures")
	_ = os.MkdirAll(resolveDir, 0755)
	_, _ = resolveTextureHints(ctx, exe, mounts, hints, resolveDir)
	tr := TextureReport{}
	r.usePIXExports(append(append([]string{}, roots...), resolveDir), &tr)
}

func previewMaterials(r *textureResolver, sc *scene.Scene, hints map[string][]string, model string) []MaterialDiag {
	index, diags := r.prepareScene(sc, hints, model)
	tr := TextureReport{}
	warnings := []string{}
	if un := applyMaterials(sc, hints, index, r.texDir, &tr, &warnings); len(un) > 0 {
		applySafeMaterialFallbacks(sc, un, r.texDir, &tr, &warnings)
	}
	return diags
}

// PreviewTexturePNG returns one preview texture as PNG (DDS is decoded) for
// the WebGL viewer. The PNG is cached beside the source file.
func PreviewTexturePNG(set, name string) ([]byte, error) {
	dir := PreviewTextureDir(set)
	name = filepath.Base(name)
	if name == "." || name == "/" || strings.HasPrefix(name, "..") {
		return nil, fmt.Errorf("bad texture name")
	}
	src := filepath.Join(dir, name)
	if strings.EqualFold(filepath.Ext(name), ".png") {
		return os.ReadFile(src)
	}
	cache := filepath.Join(dir, ".png_cache", name+".png")
	if b, err := os.ReadFile(cache); err == nil {
		return b, nil
	}
	im, err := decodeTextureFile(src)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, im); err != nil {
		return nil, err
	}
	_ = os.MkdirAll(filepath.Dir(cache), 0755)
	_ = os.WriteFile(cache, buf.Bytes(), 0644)
	return buf.Bytes(), nil
}
