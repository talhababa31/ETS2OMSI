package convert

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ets2omsi/internal/o3d"
	"ets2omsi/internal/omsi"
	"ets2omsi/internal/pim"
	"ets2omsi/internal/pit"
	"ets2omsi/internal/pixbridge"
	"ets2omsi/internal/scanner"
	"ets2omsi/internal/scene"
)

type Options struct {
	Vehicle        scanner.Vehicle
	MountPaths     []string // low -> high priority for ConverterPIX
	ConverterPIX   string
	OutputRoot     string
	WorkRoot       string
	KeepWork       bool
	StrictFidelity bool   // fail instead of committing when diagnostic/fallback textures are required
	CacheRoot      string // optional persistent ConverterPIX cache root
}
type ModelReport struct {
	Source     string `json:"source"`
	Role       string `json:"role"`
	PIM        string `json:"pim,omitempty"`
	O3D        string `json:"o3d,omitempty"`
	Vertices   int    `json:"vertices"`
	Triangles  int    `json:"triangles"`
	Materials  int    `json:"materials"`
	ReadbackOK bool   `json:"readback_ok"`
	CacheHit   bool   `json:"cache_hit,omitempty"`
	Error      string `json:"error,omitempty"`
}
type TextureReport struct {
	Copied             int      `json:"copied"`
	ExactResolved      int      `json:"exact_resolved"`
	OnDemandResolved   int      `json:"on_demand_resolved"`
	GeneratedFallbacks int      `json:"generated_fallbacks"`
	Files              []string `json:"files,omitempty"`
	Unresolved         []string `json:"unresolved,omitempty"`
}
type AutoReport struct {
	WheelBasis     string  `json:"wheel_basis"`
	WheelVisuals   int     `json:"wheel_visuals"`
	WheelModels    int     `json:"wheel_models"`
	LightBasis     string  `json:"light_basis"`
	Lights         int     `json:"lights"`
	Width          float64 `json:"width"`
	Length         float64 `json:"length"`
	Height         float64 `json:"height"`
	PhysicsProfile string  `json:"physics_profile,omitempty"`
	EstimatedMass  float64 `json:"estimated_mass_t,omitempty"`
}
type ValidationReport struct {
	O3DReadbackOK      bool       `json:"o3d_readback_ok"`
	OVHExists          bool       `json:"ovh_exists"`
	ModelCFGExists     bool       `json:"model_cfg_exists"`
	BodyO3DExists      bool       `json:"body_o3d_exists"`
	MissingTextures    []string   `json:"missing_textures,omitempty"`
	DiagnosticTextures int        `json:"diagnostic_textures"`
	OrientationOK      bool       `json:"orientation_ok"`
	GroundOK           bool       `json:"ground_ok"`
	WheelMeshes        int        `json:"wheel_meshes"`
	O3DDimensions      [3]float64 `json:"o3d_dimensions_xyz,omitempty"`
	FileCount          int        `json:"file_count"`
	Ready              bool       `json:"ready"`
}

type ManifestEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Report struct {
	Version    string           `json:"version"`
	Stage      string           `json:"stage,omitempty"`
	VehicleID  string           `json:"vehicle_id"`
	Name       string           `json:"name"`
	Status     string           `json:"status"`
	Output     string           `json:"output"`
	Started    string           `json:"started"`
	DurationMS int64            `json:"duration_ms"`
	Models     []ModelReport    `json:"models"`
	Textures   TextureReport    `json:"textures"`
	Materials  []MaterialDiag   `json:"materials,omitempty"`
	Auto       AutoReport       `json:"automation"`
	Warnings   []string         `json:"warnings,omitempty"`
	Errors     []string         `json:"errors,omitempty"`
	Created    []string         `json:"created,omitempty"`
	Validation ValidationReport `json:"validation"`
}

type convertedModel struct {
	source, role, variant, look, o3dName string
	sc                                   scene.Scene
	pix                                  pixbridge.Result
	hints                                map[string][]string
}

type wheelVisual struct {
	Slot   string
	Pos    scene.Vec3
	Radius float64
	Scene  scene.Scene
	Pix    pixbridge.Result
	Hints  map[string][]string
}

func Vehicle(ctx context.Context, opt Options) (rep Report, err error) {
	start := time.Now()
	rep = Report{Version: "V2.3.0", VehicleID: opt.Vehicle.ID, Name: opt.Vehicle.DisplayName, Started: start.Format(time.RFC3339), Status: "failed", Stage: "prepare"}
	defer func() { rep.DurationMS = time.Since(start).Milliseconds() }()
	if len(opt.Vehicle.Models) == 0 {
		rep.Stage = "resolve model"
		return rep, fmt.Errorf("main 3D model was not resolved for %s", opt.Vehicle.DisplayName)
	}
	if len(opt.MountPaths) == 0 {
		rep.Stage = "open SCS"
		return rep, fmt.Errorf("selected SCS package was not supplied")
	}
	exe := pixbridge.Find(opt.ConverterPIX)
	if exe == "" {
		rep.Stage = "3D model engine"
		return rep, fmt.Errorf("3D model engine is unavailable")
	}
	outRoot := opt.OutputRoot
	if outRoot == "" {
		outRoot = "output"
	}
	if err = os.MkdirAll(outRoot, 0755); err != nil {
		return rep, err
	}
	folder := safeName(opt.Vehicle.DisplayName)
	if folder == "" {
		folder = safeName(opt.Vehicle.ID)
	}
	if folder == "" {
		folder = "ETS2_AI_Vehicle"
	}
	final := uniqueDir(outRoot, "ETS2OMSI_"+folder)
	stage, er := os.MkdirTemp(outRoot, ".ets2omsi-")
	if er != nil {
		return rep, er
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stage)
		}
	}()
	for _, d := range []string{"model", "texture", "script"} {
		if er = os.MkdirAll(filepath.Join(stage, d), 0755); er != nil {
			return rep, er
		}
	}
	work := opt.WorkRoot
	if work == "" {
		work, er = os.MkdirTemp("", "ets2omsi-pix-")
		if er != nil {
			return rep, er
		}
		if !opt.KeepWork {
			defer os.RemoveAll(work)
		}
	} else {
		os.MkdirAll(work, 0755)
	}

	rep.Stage = "decode PMD/PMG"
	assets := conversionAssets(opt.Vehicle)
	converted := []convertedModel{}
	for i, asset := range assets {
		mp := asset.Path
		role := asset.Role
		if role == "" {
			role = "lod"
		}
		o3n := fmt.Sprintf("lod_%d.o3d", i)
		if i == 0 || role == "main" {
			role = "main"
			o3n = "body.o3d"
		} else {
			o3n = fmt.Sprintf("lod_%d.o3d", i)
		}
		modelWork := filepath.Join(work, fmt.Sprintf("model_%02d", i))
		os.MkdirAll(modelWork, 0755)
		px, cacheHit, er := pixbridge.ConvertCached(ctx, exe, opt.MountPaths, mp, opt.CacheRoot)
		_ = modelWork // retained for backwards-compatible WorkRoot layout
		mr := ModelReport{Source: mp, Role: role, PIM: px.PIM, O3D: o3n, CacheHit: cacheHit}
		if er != nil {
			mr.Error = er.Error()
			rep.Models = append(rep.Models, mr)
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", mp, er))
			if i == 0 {
				return rep, fmt.Errorf("main model conversion failed: %w", er)
			}
			continue
		}
		sc, filterNote, er := parseSceneVariant(px, asset.Variant)
		if filterNote != "" {
			rep.Warnings = append(rep.Warnings, filterNote)
		}
		if er != nil {
			mr.Error = er.Error()
			rep.Models = append(rep.Models, mr)
			rep.Errors = append(rep.Errors, er.Error())
			if i == 0 {
				return rep, er
			}
			continue
		}
		mr.Vertices = len(sc.Vertices)
		mr.Triangles = len(sc.Triangles)
		mr.Materials = len(sc.Materials)
		rep.Models = append(rep.Models, mr)
		look := vehicleLook(opt.Vehicle, asset.Look)
		hints := loadPITMaterials(&sc, px.PIT, look)
		converted = append(converted, convertedModel{source: mp, role: role, variant: asset.Variant, look: look, o3dName: o3n, sc: sc, pix: px, hints: hints})
	}
	if len(converted) == 0 {
		rep.Stage = "decode PMD/PMG"
		return rep, fmt.Errorf("no 3D geometry could be decoded from the selected SCS")
	}
	// Golden-reference coordinate policy: internal scenes stay X=lateral,
	// Y=longitudinal, Z=vertical. Center only the horizontal plane here.
	// Ground height is calibrated from the wheel contact patches after wheel
	// locators/accessories have been resolved.
	b := converted[0].sc.Bounds()
	dx := -(b.Min.X + b.Max.X) / 2
	dy := -(b.Min.Y + b.Max.Y) / 2
	for i := range converted {
		converted[i].sc.Translate(dx, dy, 0)
	}

	rep.Stage = "compose exterior"
	wheelVisuals, wheelPix, wheelRadii, wheelReports, wheelWarnings := resolveWheelVisuals(ctx, exe, opt.MountPaths, opt.Vehicle, converted[0].sc, work, opt.CacheRoot)
	rep.Models = append(rep.Models, wheelReports...)
	rep.Auto.WheelVisuals = len(wheelVisuals)
	rep.Auto.WheelModels = len(wheelPix)
	rep.Warnings = append(rep.Warnings, wheelWarnings...)

	// Calibrate the actual ground plane from wheel center - radius. This mirrors
	// proven OMSI AI vehicles much better than forcing the lowest bumper/body
	// vertex to Y=0 in O3D.
	wheelsBefore, wheelBasis := detectWheels(converted[0].sc, wheelRadii)
	if overlayWheelVisuals(wheelsBefore, wheelVisuals) > 0 {
		wheelBasis = "resolved ETS2 wheel model positions/radii"
	}
	ground := groundPlane(converted[0].sc, wheelsBefore)
	for i := range converted {
		converted[i].sc.Translate(0, 0, -ground)
	}
	for i := range wheelVisuals {
		wheelVisuals[i].Scene.Translate(0, 0, -ground)
		wheelVisuals[i].Pos.Z -= ground
	}
	wheels, wheelBasis2 := detectWheels(converted[0].sc, wheelRadii)
	if wheelBasis2 != "" && wheelBasis == "" {
		wheelBasis = wheelBasis2
	}
	if overlayWheelVisuals(wheels, wheelVisuals) > 0 {
		wheelBasis = "resolved ETS2 wheel model positions/radii"
	}
	rep.Auto.WheelBasis = wheelBasis

	mainBounds := converted[0].sc.Bounds()
	rep.Auto.Width = round3(mainBounds.Width())
	rep.Auto.Length = round3(mainBounds.Length())
	rep.Auto.Height = round3(mainBounds.Height())

	// PIT is the authoritative link from model material alias -> texture object.
	// Resolve those exact package-local paths first; no ETS2 installation is used.
	hints := map[string][]string{}
	for _, cm := range converted {
		mergeHints(hints, cm.hints)
	}
	for i := range wheelVisuals {
		if wheelVisuals[i].Hints == nil {
			wheelVisuals[i].Hints = pitHints(wheelVisuals[i].Pix.PIT)
		}
		mergeHints(hints, wheelVisuals[i].Hints)
	}

	texDir := filepath.Join(stage, "texture")
	rep.Stage = "resolve exact textures"
	resolver := newTextureResolver(opt.MountPaths, texDir)
	defer resolver.Close()
	if !resolver.Native() {
		// Package cannot be read directly (e.g. HashFS without helper): use
		// the textures ConverterPIX exports instead.
		texResolveRoot := filepath.Join(work, "exact_textures")
		_ = os.MkdirAll(texResolveRoot, 0755)
		onDemand, texWarnings := resolveTextureHints(ctx, exe, opt.MountPaths, hints, texResolveRoot)
		rep.Warnings = append(rep.Warnings, texWarnings...)
		allWork := []string{texResolveRoot}
		for _, cm := range converted {
			allWork = append(allWork, cm.pix.WorkDir)
		}
		for _, wp := range wheelPix {
			allWork = append(allWork, wp.WorkDir)
		}
		resolver.usePIXExports(allWork, &rep.Textures)
		rep.Textures.OnDemandResolved = onDemand
	}

	lods := []omsi.LOD{}
	var bodyMats []omsi.MaterialOverride
	writtenO3D := []string{}
	for i := range converted {
		cm := &converted[i]
		texIndex, diags := resolver.prepareScene(&cm.sc, cm.hints, cm.o3dName)
		rep.Materials = append(rep.Materials, diags...)
		unresolved := applyMaterials(&cm.sc, cm.hints, texIndex, texDir, &rep.Textures, &rep.Warnings)
		if len(unresolved) > 0 {
			rep.Textures.Unresolved = uniqueStringsLocal(append(rep.Textures.Unresolved, unresolved...))
			if opt.StrictFidelity {
				if i == 0 {
					rep.Stage = "resolve exact textures"
					return rep, fmt.Errorf("main model has unresolved visible texture(s): %s", strings.Join(unresolved, ", "))
				}
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("LOD %s skipped because exact package texture(s) could not be resolved: %s", cm.o3dName, strings.Join(unresolved, ", ")))
				continue
			}
			applySafeMaterialFallbacks(&cm.sc, unresolved, texDir, &rep.Textures, &rep.Warnings)
		}
		model := toO3D(cm.sc)
		op := filepath.Join(stage, "model", cm.o3dName)
		if er = o3d.WriteFile(op, &model); er != nil {
			return rep, er
		}
		writtenO3D = append(writtenO3D, op)
		parsed, er := os.ReadFile(op)
		ok := false
		if er == nil {
			if rb, e2 := o3d.Parse(parsed); e2 == nil && len(rb.Vertices) == len(model.Vertices) && len(rb.Triangles) == len(model.Triangles) {
				ok = true
			}
		}
		for j := range rep.Models {
			if rep.Models[j].Source == cm.source {
				rep.Models[j].ReadbackOK = ok
				break
			}
		}
		if !ok {
			rep.Errors = append(rep.Errors, "O3D readback validation failed for "+cm.o3dName)
		}
		mats := materialOverrides(cm.sc)
		if i == 0 {
			bodyMats = mats
		} else {
			threshold := []float64{.035, .012, .004}[minInt(i-1, 2)]
			lods = append(lods, omsi.LOD{ScreenSize: threshold, File: cm.o3dName, Materials: mats})
		}
	}

	// Wheels are separate OMSI meshes, like the proven Passat reference. Their
	// geometry is already translated to the final absolute vehicle position;
	// model.cfg supplies rotation/steering/suspension origins at the wheel centers.
	wheelFiles := map[string]string{}
	for i := range wheelVisuals {
		wv := &wheelVisuals[i]
		texIndex, diags := resolver.prepareScene(&wv.Scene, wv.Hints, "wheel_"+strings.ToLower(wv.Slot))
		rep.Materials = append(rep.Materials, diags...)
		unresolved := applyMaterials(&wv.Scene, wv.Hints, texIndex, texDir, &rep.Textures, &rep.Warnings)
		if len(unresolved) > 0 {
			rep.Textures.Unresolved = uniqueStringsLocal(append(rep.Textures.Unresolved, unresolved...))
			if opt.StrictFidelity {
				rep.Stage = "resolve wheel textures"
				return rep, fmt.Errorf("wheel %s has unresolved visible texture(s): %s", wv.Slot, strings.Join(unresolved, ", "))
			}
			applySafeMaterialFallbacks(&wv.Scene, unresolved, texDir, &rep.Textures, &rep.Warnings)
		}
		name := map[string]string{"FL": "wheel_fl.o3d", "FR": "wheel_fr.o3d", "RL": "wheel_rl.o3d", "RR": "wheel_rr.o3d"}[wv.Slot]
		if name == "" {
			continue
		}
		op := filepath.Join(stage, "model", name)
		wm := toO3D(wv.Scene)
		if er = o3d.WriteFile(op, &wm); er != nil {
			return rep, er
		}
		if b, e := os.ReadFile(op); e != nil {
			return rep, e
		} else if rb, e2 := o3d.Parse(b); e2 != nil || len(rb.Vertices) != len(wm.Vertices) || len(rb.Triangles) != len(wm.Triangles) {
			return rep, fmt.Errorf("wheel O3D readback validation failed for %s", name)
		}
		writtenO3D = append(writtenO3D, op)
		wheelFiles[wv.Slot] = name
	}
	rep.Validation.WheelMeshes = len(wheelFiles)
	if resolver.Native() {
		for _, n := range resolver.byImage {
			rep.Textures.Files = append(rep.Textures.Files, n)
		}
		rep.Textures.Copied = len(resolver.byImage)
		sort.Strings(rep.Textures.Files)
	}
	for _, d := range rep.Materials {
		if d.Status == "missing" && d.Ref != "" {
			rep.Textures.Unresolved = uniqueStringsLocal(append(rep.Textures.Unresolved, d.Alias+" -> "+d.Ref))
		}
	}
	rep.Auto.LightBasis = "V2.2 exterior-only: ETS2 gameplay light helpers intentionally ignored"
	rep.Auto.Lights = 0

	rep.Stage = "write OMSI vehicle"
	physics := omsi.EstimateAIPhysics(opt.Vehicle.DisplayName, mainBounds.Length(), mainBounds.Width(), mainBounds.Height())
	rep.Auto.PhysicsProfile = physics.Profile
	rep.Auto.EstimatedMass = round3(physics.Mass)
	spec := omsi.VehicleSpec{
		Name: folder, Type: "car",
		Length: mainBounds.Length(), Width: mainBounds.Width(), Height: mainBounds.Height(),
		Wheels: wheels, WheelFiles: wheelFiles, Materials: bodyMats, LODs: lods, Lights: nil,
		HighDetailScreenSize: .080, Physics: physics,
	}
	if er = omsi.Write(stage, spec); er != nil {
		return rep, er
	}

	rep.Stage = "validate OMSI package"
	missingTex := validateAllO3DTextureRefs(stage)
	if len(missingTex) > 0 {
		rep.Errors = append(rep.Errors, "missing OMSI output texture(s): "+strings.Join(missingTex, ", "))
	}
	rep.Validation.MissingTextures = append([]string(nil), missingTex...)
	rep.Validation.DiagnosticTextures = rep.Textures.GeneratedFallbacks
	rep.Validation.OVHExists = fileExists(filepath.Join(stage, folder+".ovh"))
	rep.Validation.ModelCFGExists = fileExists(filepath.Join(stage, "model", "model.cfg"))
	rep.Validation.BodyO3DExists = fileExists(filepath.Join(stage, "model", "body.o3d"))
	rep.Validation.O3DReadbackOK = len(writtenO3D) > 0
	for _, op := range writtenO3D {
		b, e := os.ReadFile(op)
		if e != nil {
			rep.Validation.O3DReadbackOK = false
			break
		}
		if _, e = o3d.Parse(b); e != nil {
			rep.Validation.O3DReadbackOK = false
			break
		}
	}
	rep.Validation.OrientationOK, rep.Validation.O3DDimensions = validateOMSIOrientation(filepath.Join(stage, "model", "body.o3d"), mainBounds)
	rep.Validation.GroundOK = validateGround(wheels, mainBounds)
	if !rep.Validation.OrientationOK {
		if opt.StrictFidelity {
			rep.Errors = append(rep.Errors, "OMSI axis validation failed: expected O3D X=width, Y=height, Z=length")
		} else {
			rep.Warnings = append(rep.Warnings, "OMSI axis validation is outside the preferred profile; SCS-only safe export continues")
		}
	}
	if !rep.Validation.GroundOK {
		rep.Warnings = append(rep.Warnings, "ground/wheel contact validation is outside the preferred tolerance")
	}

	snippet := fmt.Sprintf("vehicles\\%s\\%s.ovh\r\n", filepath.Base(final), folder)
	_ = os.WriteFile(filepath.Join(stage, "ailists_snippet.txt"), []byte("; ETS2OMSI generated vehicle reference\r\n"+snippet), 0644)
	rep.Stage = "complete"
	rep.Warnings = uniqueStringsLocal(rep.Warnings)
	rep.Status = "pass"
	if len(rep.Errors) > 0 {
		rep.Status = "fail"
	} else if len(rep.Warnings) > 0 {
		rep.Status = "warn"
	}
	orientationReady := rep.Validation.OrientationOK || !opt.StrictFidelity
	rep.Validation.Ready = rep.Status != "fail" && rep.Validation.O3DReadbackOK && rep.Validation.OVHExists && rep.Validation.ModelCFGExists && rep.Validation.BodyO3DExists && orientationReady && len(rep.Validation.MissingTextures) == 0
	rep.Output = final
	rb, _ := json.MarshalIndent(rep, "", "  ")
	_ = os.WriteFile(filepath.Join(stage, "conversion_report.json"), rb, 0644)
	_ = os.WriteFile(filepath.Join(stage, "conversion_report.txt"), []byte(textReport(rep)), 0644)
	manifest := buildManifest(stage)
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	_ = os.WriteFile(filepath.Join(stage, "package_manifest.json"), mb, 0644)
	rep.Created = listRelative(stage)
	rep.Validation.FileCount = len(rep.Created)
	rb, _ = json.MarshalIndent(rep, "", "  ")
	_ = os.WriteFile(filepath.Join(stage, "conversion_report.json"), rb, 0644)
	_ = os.WriteFile(filepath.Join(stage, "conversion_report.txt"), []byte(textReport(rep)), 0644)
	if rep.Status == "fail" || !rep.Validation.Ready {
		return rep, fmt.Errorf("preflight validation failed; staging output not committed")
	}
	if er = os.Rename(stage, final); er != nil {
		return rep, er
	}
	committed = true
	return rep, nil
}

type wheelPlacement struct {
	Family string
	Index  int
	Slot   string
	Pos    scene.Vec3
}

func conversionAssets(v scanner.Vehicle) []scanner.ModelAsset {
	if len(v.ModelAssets) == 0 {
		m := orderedModels(v.Models)
		out := make([]scanner.ModelAsset, 0, len(m))
		for i, p := range m {
			role := "lod"
			if i == 0 {
				role = "main"
			}
			out = append(out, scanner.ModelAsset{Path: p, Role: role})
		}
		return out
	}
	out := []scanner.ModelAsset{}
	mainFound := false
	for _, a := range v.ModelAssets {
		if a.Role == "main" && !mainFound {
			out = append(out, a)
			mainFound = true
		}
	}
	// V2 is exterior-only. Never promote ETS2 detail/interior/accessory models.
	// If a semantic main role is absent, fall back to the first package-resolved
	// traffic PMD rather than importing a player/detail model.
	if !mainFound && len(v.Models) > 0 {
		out = append(out, scanner.ModelAsset{Path: v.Models[0], Role: "main"})
		mainFound = true
	}
	for _, a := range v.ModelAssets {
		if a.Role == "lod" {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return conversionAssets(scanner.Vehicle{Models: v.Models})
	}
	return out
}

func parseSceneVariant(px pixbridge.Result, variant string) (scene.Scene, string, error) {
	if px.PIT != "" {
		want := strings.TrimSpace(variant)
		if want == "" {
			want = "default"
		}
		vf, err := pit.Variant(px.PIT, want)
		if err == nil && vf.Found {
			sc, er := pim.ParseFileWithOptions(px.PIM, pim.Options{VisibleParts: vf.VisibleParts})
			return sc, "", er
		}
		if strings.TrimSpace(variant) != "" && !strings.EqualFold(variant, "default") {
			sc, er := pim.ParseFile(px.PIM)
			return sc, fmt.Sprintf("variant %q was not found in PIT for %s; all parts were preserved", variant, filepath.Base(px.PIM)), er
		}
	}
	sc, err := pim.ParseFile(px.PIM)
	return sc, "", err
}

func resolveWheelVisuals(ctx context.Context, exe string, mounts []string, v scanner.Vehicle, main scene.Scene, work, cacheRoot string) ([]wheelVisual, []pixbridge.Result, map[string]float64, []ModelReport, []string) {
	visuals := []wheelVisual{}
	results := []pixbridge.Result{}
	radii := map[string]float64{}
	reports := []ModelReport{}
	warnings := []string{}
	if len(v.WheelAttachments) == 0 {
		return visuals, results, radii, reports, warnings
	}
	placements := wheelPlacements(main)
	if len(placements) < 4 {
		placements = estimatedWheelPlacements(main.Bounds())
		warnings = append(warnings, "complete ETS2 wheel locator set was not found; wheel positions use a conservative geometry estimate")
	}
	type cachedWheel struct {
		pix      pixbridge.Result
		raw      scene.Scene
		variants []string
	}
	cache := map[string]*cachedWheel{}
	for _, pl := range placements {
		att, ok := wheelAttachmentFor(v.WheelAttachments, pl)
		if !ok || att.Model == "" || pl.Slot == "" {
			continue
		}
		key := strings.ToLower(att.Model)
		cw := cache[key]
		if cw == nil {
			wd := filepath.Join(work, fmt.Sprintf("wheel_%02d", len(cache)))
			_ = os.MkdirAll(wd, 0755)
			px, cacheHit, err := pixbridge.ConvertCached(ctx, exe, mounts, att.Model, cacheRoot)
			mr := ModelReport{Source: att.Model, Role: "wheel", PIM: px.PIM, CacheHit: cacheHit}
			if err != nil {
				mr.Error = err.Error()
				reports = append(reports, mr)
				warnings = append(warnings, fmt.Sprintf("wheel model %s could not be converted: %v", att.Model, err))
				cache[key] = &cachedWheel{}
				continue
			}
			raw, err := pim.ParseFile(px.PIM)
			if err != nil {
				mr.Error = err.Error()
				reports = append(reports, mr)
				cache[key] = &cachedWheel{pix: px}
				continue
			}
			vars, _ := pit.AvailableVariants(px.PIT)
			mr.Vertices, mr.Triangles, mr.Materials = len(raw.Vertices), len(raw.Triangles), len(raw.Materials)
			reports = append(reports, mr)
			cache[key] = &cachedWheel{pix: px, raw: raw, variants: vars}
			cw = cache[key]
			results = append(results, px)
		}
		if cw == nil || cw.pix.PIM == "" || len(cw.raw.Vertices) == 0 {
			continue
		}
		variant := wheelVariantForSide(cw.variants, pl.Pos.X, att.Variant)
		wsc := cw.raw
		if variant != "" && cw.pix.PIT != "" {
			if vf, err := pit.Variant(cw.pix.PIT, variant); err == nil && vf.Found {
				if fs, er := pim.ParseFileWithOptions(cw.pix.PIM, pim.Options{VisibleParts: vf.VisibleParts}); er == nil && len(fs.Vertices) > 0 {
					wsc = fs
				}
			}
		}
		r := wheelRadius(wsc)
		if r > .05 && r < 2.5 {
			radii[pl.Slot] = r
		}
		// Deep copy: the cached wheel scene is shared by all four slots.
		placed := wsc.Clone()
		placed.Translate(pl.Pos.X, pl.Pos.Y, pl.Pos.Z)
		hints := loadPITMaterials(&placed, cw.pix.PIT, att.Look)
		visuals = append(visuals, wheelVisual{Slot: pl.Slot, Pos: pl.Pos, Radius: r, Scene: placed, Pix: cw.pix, Hints: hints})
	}
	if len(v.WheelAttachments) > 0 && len(visuals) == 0 {
		warnings = append(warnings, "wheel accessories were detected but no wheel model could be resolved; body conversion is kept intact")
	}
	return visuals, results, radii, reports, uniqueStringsLocal(warnings)
}

func wheelPlacements(sc scene.Scene) []wheelPlacement {
	re := regexp.MustCompile(`(?i)(?:^|[^a-z0-9])wheel_([fr])_([0-9]+)(?:$|[^0-9])`)
	out := []wheelPlacement{}
	for _, l := range sc.Locators {
		text := strings.ToLower(l.Name + " " + l.Hookup)
		m := re.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		i, _ := strconv.Atoi(m[2])
		family := strings.ToLower(m[1])
		slot := ""
		if family == "f" {
			if l.Position.X < 0 {
				slot = "FL"
			} else {
				slot = "FR"
			}
		}
		if family == "r" {
			if l.Position.X < 0 {
				slot = "RL"
			} else {
				slot = "RR"
			}
		}
		out = append(out, wheelPlacement{Family: family, Index: i, Slot: slot, Pos: l.Position})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Family == out[j].Family {
			return out[i].Index < out[j].Index
		}
		return out[i].Family < out[j].Family
	})
	return out
}

func estimatedWheelPlacements(b scene.Bounds) []wheelPlacement {
	x := b.Width() * .42
	fy := b.Max.Y - b.Length()*.21
	ry := b.Min.Y + b.Length()*.21
	z := b.Min.Z + math.Max(.24, math.Min(.62, b.Height()*.22))
	return []wheelPlacement{
		{Family: "f", Index: 0, Slot: "FL", Pos: scene.Vec3{X: -x, Y: fy, Z: z}},
		{Family: "f", Index: 1, Slot: "FR", Pos: scene.Vec3{X: x, Y: fy, Z: z}},
		{Family: "r", Index: 0, Slot: "RL", Pos: scene.Vec3{X: -x, Y: ry, Z: z}},
		{Family: "r", Index: 1, Slot: "RR", Pos: scene.Vec3{X: x, Y: ry, Z: z}},
	}
}

func wheelAttachmentFor(in []scanner.WheelAttachment, p wheelPlacement) (scanner.WheelAttachment, bool) {
	// First pass: documented family and offset pair (0/1, 2/3, ...).
	for _, w := range in {
		fam := w.Family
		if fam == "unknown" {
			n := strings.ToLower(w.UnitID + " " + w.DataPath)
			if strings.Contains(n, "fwheel") || strings.Contains(n, "f_wheel") {
				fam = "f"
			}
			if strings.Contains(n, "rwheel") || strings.Contains(n, "r_wheel") {
				fam = "r"
			}
		}
		if fam == p.Family && p.Index >= w.Offset && p.Index < w.Offset+2 {
			return w, true
		}
	}
	// If exactly one accessory exists for the requested family, use it.
	var candidate scanner.WheelAttachment
	n := 0
	for _, w := range in {
		if w.Family == p.Family || w.Family == "ai" || w.Family == "unknown" {
			candidate = w
			n++
		}
	}
	return candidate, n == 1
}

func wheelVariantForSide(vars []string, x float64, configured string) string {
	wantLeft := x < 0
	for _, v := range vars {
		l := strings.ToLower(v)
		if wantLeft && (l == "left" || l == "l" || strings.Contains(l, "left")) {
			return v
		}
		if !wantLeft && (l == "right" || l == "r" || strings.Contains(l, "right")) {
			return v
		}
	}
	if configured != "" {
		for _, v := range vars {
			if strings.EqualFold(v, configured) {
				return v
			}
		}
	}
	for _, v := range vars {
		if strings.EqualFold(v, "default") {
			return v
		}
	}
	if len(vars) > 0 {
		return vars[0]
	}
	return ""
}

func wheelRadius(sc scene.Scene) float64 {
	for _, l := range sc.Locators {
		if strings.EqualFold(strings.TrimSpace(l.Name), "bb") {
			// SCS uses the bb locator to describe wheel size. Its distance from the
			// model origin is a robust radius estimate across legacy wheel models.
			return l.Position.Length()
		}
	}
	b := sc.Bounds()
	return math.Max(.05, math.Min(b.Width(), b.Height())*.5)
}

func uniqueStringsLocal(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		k := strings.ToLower(s)
		if !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	return out
}

func orderedModels(in []string) []string {
	u := []string{}
	seen := map[string]bool{}
	for _, p := range in {
		k := strings.ToLower(p)
		if !seen[k] {
			seen[k] = true
			u = append(u, p)
		}
	}
	sort.SliceStable(u, func(i, j int) bool {
		a, b := strings.Contains(strings.ToLower(filepath.Base(u[i])), "lod"), strings.Contains(strings.ToLower(filepath.Base(u[j])), "lod")
		if a != b {
			return !a
		}
		return u[i] < u[j]
	})
	return u
}
func safeName(s string) string {
	r := regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(strings.TrimSpace(s), "_")
	return strings.Trim(r, "_")
}
func uniqueDir(root, name string) string {
	p := filepath.Join(root, name)
	if _, e := os.Stat(p); os.IsNotExist(e) {
		return p
	}
	for i := 2; ; i++ {
		x := filepath.Join(root, fmt.Sprintf("%s_%d", name, i))
		if _, e := os.Stat(x); os.IsNotExist(e) {
			return x
		}
	}
}
func copyFile(src, dst string) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.Create(dst)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	ce := out.Close()
	if e == nil {
		e = ce
	}
	return e
}

const ambiguousTexture = "<ambiguous>"

func normalizeTextureKey(s string) string {
	s = strings.TrimSpace(strings.Trim(s, "\"'"))
	s = strings.ReplaceAll(s, "\\", "/")
	for strings.HasPrefix(s, "./") {
		s = strings.TrimPrefix(s, "./")
	}
	s = strings.TrimLeft(s, "/")
	return strings.ToLower(filepath.ToSlash(s))
}

func addTextureIndex(index map[string]string, key, value string) {
	key = normalizeTextureKey(key)
	if key == "" || value == "" {
		return
	}
	if old, ok := index[key]; ok && old != value {
		index[key] = ambiguousTexture
		return
	}
	index[key] = value
}

func textureFingerprint(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func collectTextures(roots []string, dst string, index map[string]string) TextureReport {
	r := TextureReport{}
	contentDest := map[string]string{}
	usedNames := map[string]bool{}
	for _, root := range uniqueStringsLocal(roots) {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
			if e != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".dds" && ext != ".png" && ext != ".tga" && ext != ".jpg" && ext != ".jpeg" && ext != ".bmp" {
				return nil
			}
			rel, er := filepath.Rel(root, p)
			if er != nil {
				rel = filepath.Base(p)
			}
			rel = filepath.ToSlash(rel)
			fp := textureFingerprint(p)
			name := ""
			if fp != "" {
				name = contentDest[fp]
			}
			if name == "" {
				// OMSI texture names are written into O3D/model.cfg; keep them
				// free of spaces.
				base := strings.ReplaceAll(filepath.Base(p), " ", "_")
				name = base
				if usedNames[strings.ToLower(name)] {
					name = shortHash(normalizeTextureKey(rel)) + "_" + base
				}
				if copyFile(p, filepath.Join(dst, name)) != nil {
					return nil
				}
				usedNames[strings.ToLower(name)] = true
				if fp != "" {
					contentDest[fp] = name
				}
				r.Copied++
				r.Files = append(r.Files, name)
			}
			// Exact relative path is strongest. PIT normally points to the same
			// virtual path but with .tobj; indexing the extensionless path makes
			// /foo/body.tobj resolve deterministically to /foo/body.dds/png/tga.
			relKey := normalizeTextureKey(rel)
			relStem := strings.TrimSuffix(relKey, filepath.Ext(relKey))
			addTextureIndex(index, relKey, name)
			addTextureIndex(index, relStem, name)
			baseKey := normalizeTextureKey(filepath.Base(rel))
			baseStem := strings.TrimSuffix(baseKey, filepath.Ext(baseKey))
			addTextureIndex(index, baseKey, name)
			addTextureIndex(index, baseStem, name)
			addTextureIndex(index, tightIndexKey(textureTightKey(rel)), name)
			addTextureIndex(index, tightIndexKey(textureTightBase(rel)), name)
			return nil
		})
	}
	sort.Strings(r.Files)
	return r
}

func shortHash(s string) string { h := sha1.Sum([]byte(s)); return hex.EncodeToString(h[:3]) }
func mergeHints(dst, src map[string][]string) {
	for k, v := range src {
		dst[k] = uniqueStringsLocal(append(dst[k], v...))
	}
}

// pitHints preserves the explicit Material Alias -> visible/diffuse texture
// relation from ConverterPIX PIT. Normal/specular/mask/reflection maps must never
// become an OMSI diffuse texture. Doing so caused legacy head/rear lamp textures
// and wheel materials to appear swapped or nonsensical.
func pitHints(file string) map[string][]string {
	return pitHintsFromMaterials(pitLookMaterials(file, ""))
}

func isVisibleBaseTextureTag(tag string) bool {
	t := strings.ToLower(strings.TrimSpace(tag))
	return strings.Contains(t, "texture_base") || strings.Contains(t, "texture_diffuse") || strings.Contains(t, "texture_albedo") || strings.Contains(t, "texture_color")
}

func isNonDiffuseTextureTag(tag string) bool {
	t := strings.ToLower(strings.TrimSpace(tag))
	for _, bad := range []string{"nmap", "normal", "mask", "spec", "gloss", "reflection", "reflect", "env", "shadow", "lightmap", "ao"} {
		if strings.Contains(t, bad) {
			return true
		}
	}
	return false
}

func resolveTextureHints(ctx context.Context, exe string, mounts []string, hints map[string][]string, out string) (int, []string) {
	refs := []string{}
	for _, vv := range hints {
		refs = append(refs, vv...)
	}
	refs = uniqueStringsLocal(refs)
	okCount := 0
	warnings := []string{}
	for _, ref := range refs {
		l := strings.ToLower(strings.TrimSpace(strings.Trim(ref, "\"'")))
		var err error
		switch {
		case strings.HasSuffix(l, ".tobj"):
			_, err = pixbridge.ConvertTextureObject(ctx, exe, mounts, ref, out)
		case strings.HasSuffix(l, ".dds"), strings.HasSuffix(l, ".png"), strings.HasSuffix(l, ".tga"), strings.HasSuffix(l, ".bmp"), strings.HasSuffix(l, ".jpg"), strings.HasSuffix(l, ".jpeg"):
			_, err = pixbridge.ExtractFile(ctx, exe, mounts, ref, out)
		default:
			// PIT references are usually extensionless. Ask ConverterPIX for the
			// matching TOBJ, but silently: base-game references are expected to
			// be absent in SCS-only mode.
			if _, e := pixbridge.ConvertTextureObject(ctx, exe, mounts, ref+".tobj", out); e == nil {
				okCount++
			}
			continue
		}
		if err == nil {
			okCount++
			continue
		}
		// Missing common ETS2 helper textures are intentionally not fatal. Keep
		// diagnostics concise: the material resolver later decides whether the
		// unresolved reference is actually visible/required.
		if len(warnings) < 16 {
			warnings = append(warnings, fmt.Sprintf("package texture reference could not be extracted: %s", ref))
		}
	}
	return okCount, uniqueStringsLocal(warnings)
}

func lookupTexture(index map[string]string, candidate string) string {
	key := normalizeTextureKey(candidate)
	if key == "" {
		return ""
	}
	keys := []string{key}
	ext := filepath.Ext(key)
	if ext != "" {
		keys = append(keys, strings.TrimSuffix(key, ext))
	}
	base := normalizeTextureKey(filepath.Base(key))
	keys = append(keys, base)
	if e := filepath.Ext(base); e != "" {
		keys = append(keys, strings.TrimSuffix(base, e))
	}
	for _, k := range uniqueStringsLocal(keys) {
		if v := index[k]; v != "" && v != ambiguousTexture {
			return v
		}
	}
	// Space/underscore-insensitive match: PIT "tableaudebord" must find
	// "tableau de bord.dds". Ambiguous tight keys are rejected.
	for _, k := range []string{tightIndexKey(textureTightKey(key)), tightIndexKey(textureTightBase(key))} {
		if v := index[k]; v != "" && v != ambiguousTexture {
			return v
		}
	}
	return ""
}

func tightIndexKey(k string) string {
	if k == "" {
		return ""
	}
	return "tight:" + k
}

func converterPIXAliasTextureStem(alias string) string {
	a := strings.TrimSpace(strings.ToLower(alias))
	if !strings.HasPrefix(a, "mat_") || len(a) < 10 {
		return ""
	}
	// Expected form: mat_ + four decimal material digits + _ + texture stem.
	for i := 4; i < 8; i++ {
		if a[i] < '0' || a[i] > '9' {
			return ""
		}
	}
	if a[8] != '_' {
		return ""
	}
	stem := strings.TrimSpace(a[9:])
	if stem == "" || strings.ContainsAny(stem, "/\\") {
		return ""
	}
	return stem
}

func materialClass(m scene.Material) string {
	s := strings.ToLower(m.Alias + " " + m.Effect)
	switch {
	case strings.Contains(s, "glass") || strings.Contains(s, "window"):
		return "glass"
	case strings.Contains(s, "lamp") || strings.Contains(s, "light") || strings.Contains(s, "flare"):
		return "light"
	case strings.Contains(s, "chrome"):
		return "chrome"
	case strings.Contains(s, "rubber") || strings.Contains(s, "tire") || strings.Contains(s, "tyre"):
		return "rubber"
	case strings.Contains(s, "paint"):
		return "paint"
	default:
		return "body"
	}
}
func applyMaterials(sc *scene.Scene, hints map[string][]string, index map[string]string, texDir string, tr *TextureReport, warnings *[]string) []string {
	unresolved := []string{}
	for i := range sc.Materials {
		m := &sc.Materials[i]
		m.Class = materialClass(*m)
		m.Alpha = m.Class == "glass" || strings.Contains(strings.ToLower(m.Effect), ".a") || strings.Contains(strings.ToLower(m.Effect), "blend")
		candidates := append([]string{}, hints[strings.ToLower(strings.TrimSpace(m.Alias))]...)
		// Some legacy PIMs put a material/texture path directly in Alias. This is
		// still exact resolution, not fuzzy matching.
		if strings.Contains(m.Alias, "/") || strings.Contains(m.Alias, "\\") || filepath.Ext(m.Alias) != "" {
			candidates = append(candidates, m.Alias)
		}
		explicitTextureRef := len(uniqueStringsLocal(candidates)) > 0
		tex := ""
		for _, c := range uniqueStringsLocal(candidates) {
			if v := lookupTexture(index, c); v != "" {
				tex = v
				break
			}
		}
		if tex == "" {
			// ConverterPIX commonly emits deterministic aliases such as
			// mat_0000_body where the suffix is the first/base texture name.
			// Treat only that exact suffix as a candidate; this is deliberately
			// not fuzzy matching.
			if stem := converterPIXAliasTextureStem(m.Alias); stem != "" {
				tex = lookupTexture(index, stem)
			}
		}
		if tex == "" {
			// Last safe fallback: the alias itself, but only when it resolves to
			// one globally-unambiguous basename/stem in the copied texture index.
			// There is deliberately no substring/fuzzy score.
			tex = lookupTexture(index, m.Alias)
		}
		if tex == "" {
			sig := strings.ToLower(m.Alias + " " + m.Effect)
			// A number of legacy ETS2 AI traffic models use a glass shader (for
			// example aliases ending in glass_ex) without any diffuse/base texture.
			// That is a valid material, not a missing asset. Only synthesize OMSI
			// glass when the PIT/PIM did not explicitly reference a texture; if an
			// explicit texture path exists but cannot be resolved, keep failing so a
			// genuinely missing window texture is never hidden.
			texturelessGlass := m.Class == "glass" && !explicitTextureRef
			optionalHelper := m.Class == "light" || strings.Contains(sig, "shadow") || strings.Contains(sig, "occlusion") || strings.Contains(sig, "trucklight") || strings.Contains(sig, "reflection")
			if texturelessGlass || optionalHelper {
				// Legacy textureless glass shaders and ETS2-only helpers
				// (lamps, shadows, flares): generated OMSI replacement.
				kind, _, alpha := generatedKind(sc, i, strings.Join(candidates, " "))
				if texturelessGlass {
					kind, alpha = genGlass, true
				}
				if kind == genPaint {
					kind = genTrim
				}
				tex = writeGeneratedTexture(texDir, kind, tr)
				m.Alpha = m.Alpha || alpha
			} else {
				label := strings.TrimSpace(m.Alias)
				if label == "" {
					label = fmt.Sprintf("material_%d", i)
				}
				unresolved = append(unresolved, label)
				continue
			}
		} else {
			tr.ExactResolved++
			if m.HasTint {
				// Bake the ETS2 diffuse colour into the texture (grey body
				// textures are coloured by it in ETS2).
				if baked := bakeTint(texDir, tex, m.Tint); baked != "" {
					tex = baked
					m.HasTint = false
				}
			}
		}
		m.Texture = tex
	}
	return uniqueStringsLocal(unresolved)
}

func applySafeMaterialFallbacks(sc *scene.Scene, unresolved []string, texDir string, tr *TextureReport, warnings *[]string) {
	wanted := map[string]bool{}
	for _, u := range unresolved {
		wanted[strings.ToLower(strings.TrimSpace(u))] = true
	}
	var paint color.NRGBA
	paintKnown, havePaint := false, false
	for i := range sc.Materials {
		m := &sc.Materials[i]
		label := strings.TrimSpace(m.Alias)
		if label == "" {
			label = fmt.Sprintf("material_%d", i)
		}
		if !wanted[strings.ToLower(strings.TrimSpace(label))] || strings.TrimSpace(m.Texture) != "" {
			continue
		}
		class := m.Class
		if class == "" {
			class = materialClass(*m)
			m.Class = class
		}
		if class == "glass" {
			m.Alpha = true
		}
		// Known part (glass, lamp, rim, tyre, chrome, interior ...): use a
		// generated texture of that kind instead of a flat placeholder.
		if kind, solid, alpha := generatedKind(sc, i, ""); kind != genPaint || solid != nil {
			n := ""
			if solid != nil {
				n = writeSolidTexture(texDir, *solid, tr)
			} else {
				n = writeGeneratedTexture(texDir, kind, tr)
			}
			if n != "" {
				m.Texture = n
				m.Alpha = m.Alpha || alpha
				m.HasTint = false
				continue
			}
		}
		name := "fallback_" + class + ".png"
		if isPaintLikeClass(class) {
			name = "fallback_body.png"
			class = "body"
			if m.HasTint {
				// The material's own ETS2 paint colour is the best evidence.
				if n := writePaintFallback(texDir, tintColor(m.Tint), tr); n != "" {
					m.Texture = n
					m.HasTint = false
					continue
				}
			}
			if !paintKnown {
				paintKnown = true
				paint, havePaint = scenePaintColor(sc, texDir)
			}
			if havePaint {
				if n := writePaintFallback(texDir, paint, tr); n != "" {
					name = n
				}
			}
		}
		p := filepath.Join(texDir, name)
		if _, e := os.Stat(p); os.IsNotExist(e) {
			_ = writeNeutral(p, class)
			tr.GeneratedFallbacks++
			tr.Files = append(tr.Files, name)
		}
		m.Texture = name
		if class == "glass" {
			// Shared ETS2 glass (e.g. /vehicle/truck/share/glass_ex) lives in
			// base.scs; translucent OMSI glass is the intended SCS-only result.
			continue
		}
		msg := fmt.Sprintf("SCS-only safe export: unresolved material %s received neutral OMSI %s fallback; conversion continues", label, class)
		dup := false
		for _, w := range *warnings {
			if w == msg {
				dup = true
				break
			}
		}
		if !dup {
			*warnings = append(*warnings, msg)
		}
	}
}

func writeNeutral(p, class string) error {
	c := color.NRGBA{48, 48, 52, 255}
	switch class {
	case "body", "paint":
		// No paint information at all: a light neutral paint reads as a car
		// body in OMSI; the former near-black looked like missing panels.
		c = color.NRGBA{200, 200, 205, 255}
	case "glass":
		c = color.NRGBA{95, 110, 120, 120}
	case "light":
		c = color.NRGBA{30, 30, 32, 255}
	case "chrome":
		c = color.NRGBA{185, 185, 190, 255}
	case "rubber":
		c = color.NRGBA{24, 24, 26, 255}
	}
	im := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			im.Set(x, y, c)
		}
	}
	f, e := os.Create(p)
	if e != nil {
		return e
	}
	e = png.Encode(f, im)
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return e
}

func writeChecker(p string, seed int) error {
	im := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	c1 := color.NRGBA{255, uint8(40 + seed*31%180), 220, 255}
	c2 := color.NRGBA{30, 30, 34, 255}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if (x/8+y/8)%2 == 0 {
				im.Set(x, y, c1)
			} else {
				im.Set(x, y, c2)
			}
		}
	}
	f, e := os.Create(p)
	if e != nil {
		return e
	}
	e = png.Encode(f, im)
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return e
}
func toO3D(sc scene.Scene) o3d.Model {
	m := o3d.Model{Transform: o3d.IdentityTransform()}
	for _, v := range sc.Vertices {
		// Internal/ETS2OMSI coordinates: X=lateral, Y=longitudinal, Z=vertical.
		// Proven OMSI O3D vehicle files (golden Passat reference) store
		// X=lateral, Y=vertical, Z=longitudinal. Swap Y/Z for both positions
		// and normals. This is the critical orientation fix for V2.
		m.Vertices = append(m.Vertices, o3d.Vertex{
			X: float32(v.Position.X), Y: float32(v.Position.Z), Z: float32(v.Position.Y),
			NX: float32(v.Normal.X), NY: float32(v.Normal.Z), NZ: float32(v.Normal.Y),
			U: float32(v.UV.X), V: float32(v.UV.Y),
		})
	}
	for _, t := range sc.Triangles {
		mat := t.Material
		if mat < 0 || mat >= len(sc.Materials) {
			mat = 0
		}
		a, b, c := uint32(t.A), uint32(t.B), uint32(t.C)
		// Y/Z swap changes handedness. Flip once, then keep face winding
		// consistent with the imported vertex normals.
		b, c = c, b
		if faceOpposesNormals(m.Vertices, a, b, c) {
			b, c = c, b
		}
		m.Triangles = append(m.Triangles, o3d.Triangle{A: a, B: b, C: c, Material: uint16(mat)})
	}
	for _, x := range sc.Materials {
		d := [4]float32{1, 1, 1, 1}
		if x.HasTint {
			// Tint that could not be baked into a texture: let OMSI's
			// material diffuse colour carry it.
			d = [4]float32{float32(x.Tint[0]), float32(x.Tint[1]), float32(x.Tint[2]), 1}
		}
		// Working OMSI references keep O3D diffuse alpha opaque and let
		// [matl_alpha] + the image alpha channel drive glass transparency.
		spec := [3]float32{.25, .25, .25}
		pow := float32(12)
		if x.Class == "chrome" {
			spec = [3]float32{.8, .8, .8}
			pow = 50
		}
		if x.Class == "rubber" {
			spec = [3]float32{.05, .05, .05}
			pow = 4
		}
		m.Materials = append(m.Materials, o3d.Material{Diffuse: d, Specular: spec, SpecularPower: pow, Texture: x.Texture})
	}
	if len(m.Materials) == 0 {
		m.Materials = append(m.Materials, o3d.Material{Diffuse: [4]float32{1, 1, 1, 1}, Texture: "missing_mat_00.png"})
	}
	return m
}

func faceOpposesNormals(v []o3d.Vertex, a, b, c uint32) bool {
	if int(a) >= len(v) || int(b) >= len(v) || int(c) >= len(v) {
		return false
	}
	p, q, r := v[a], v[b], v[c]
	ux, uy, uz := float64(q.X-p.X), float64(q.Y-p.Y), float64(q.Z-p.Z)
	vx, vy, vz := float64(r.X-p.X), float64(r.Y-p.Y), float64(r.Z-p.Z)
	nx, ny, nz := uy*vz-uz*vy, uz*vx-ux*vz, ux*vy-uy*vx
	anx := float64(p.NX + q.NX + r.NX)
	any := float64(p.NY + q.NY + r.NY)
	anz := float64(p.NZ + q.NZ + r.NZ)
	if math.Abs(anx)+math.Abs(any)+math.Abs(anz) < 1e-9 {
		return false
	}
	return nx*anx+ny*any+nz*anz < 0
}

// sceneFaceOpposesNormals is the internal-coordinate twin of
// faceOpposesNormals, used by the browser preview.
func sceneFaceOpposesNormals(v []scene.Vertex, a, b, c int) bool {
	if a < 0 || b < 0 || c < 0 || a >= len(v) || b >= len(v) || c >= len(v) {
		return false
	}
	p, q, r := v[a].Position, v[b].Position, v[c].Position
	ux, uy, uz := q.X-p.X, q.Y-p.Y, q.Z-p.Z
	vx, vy, vz := r.X-p.X, r.Y-p.Y, r.Z-p.Z
	nx, ny, nz := uy*vz-uz*vy, uz*vx-ux*vz, ux*vy-uy*vx
	anx := v[a].Normal.X + v[b].Normal.X + v[c].Normal.X
	any := v[a].Normal.Y + v[b].Normal.Y + v[c].Normal.Y
	anz := v[a].Normal.Z + v[b].Normal.Z + v[c].Normal.Z
	if math.Abs(anx)+math.Abs(any)+math.Abs(anz) < 1e-9 {
		return false
	}
	return nx*anx+ny*any+nz*anz < 0
}

func materialOverrides(sc scene.Scene) []omsi.MaterialOverride {
	out := []omsi.MaterialOverride{}
	for i, m := range sc.Materials {
		a := 0
		if m.Alpha {
			a = 2
		}
		out = append(out, omsi.MaterialOverride{Texture: m.Texture, Instance: i, AlphaMode: a, Class: m.Class})
	}
	return out
}
func detectWheels(sc scene.Scene, radii map[string]float64) (map[string]omsi.Wheel, string) {
	out := map[string]omsi.Wheel{}
	b := sc.Bounds()
	fallbackRad := math.Max(.24, math.Min(.62, b.Height()*.22))
	for _, p := range wheelPlacements(sc) {
		if p.Slot == "" {
			continue
		}
		r := radii[p.Slot]
		if r <= 0 {
			r = fallbackRad
		}
		out[p.Slot] = omsi.Wheel{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z, Radius: r, Width: .2}
	}
	if len(out) >= 4 {
		return out, "ETS2 wheel_f/wheel_r locators + wheel accessory radius"
	}
	x := b.Width() * .42
	fy := b.Max.Y - b.Length()*.21
	ry := b.Min.Y + b.Length()*.21
	for slot, p := range map[string]scene.Vec3{"FL": {X: -x, Y: fy, Z: fallbackRad}, "FR": {X: x, Y: fy, Z: fallbackRad}, "RL": {X: -x, Y: ry, Z: fallbackRad}, "RR": {X: x, Y: ry, Z: fallbackRad}} {
		if _, ok := out[slot]; ok {
			continue
		}
		r := radii[slot]
		if r <= 0 {
			r = fallbackRad
		}
		out[slot] = omsi.Wheel{X: p.X, Y: p.Y, Z: p.Z, Radius: r, Width: .2}
	}
	return out, "mixed ETS2 locators / conservative geometry estimate"
}

func overlayWheelVisuals(dst map[string]omsi.Wheel, visuals []wheelVisual) int {
	n := 0
	for _, v := range visuals {
		if v.Slot == "" || v.Radius <= 0 {
			continue
		}
		dst[v.Slot] = omsi.Wheel{X: v.Pos.X, Y: v.Pos.Y, Z: v.Pos.Z, Radius: v.Radius, Width: .2}
		n++
	}
	return n
}

func groundPlane(sc scene.Scene, wheels map[string]omsi.Wheel) float64 {
	contacts := []float64{}
	for _, slot := range []string{"FL", "FR", "RL", "RR"} {
		w, ok := wheels[slot]
		if !ok || w.Radius < .08 || w.Radius > 1.5 {
			continue
		}
		contacts = append(contacts, w.Z-w.Radius)
	}
	if len(contacts) >= 2 {
		sort.Float64s(contacts)
		if len(contacts)%2 == 1 {
			return contacts[len(contacts)/2]
		}
		return (contacts[len(contacts)/2-1] + contacts[len(contacts)/2]) / 2
	}
	return sc.Bounds().Min.Z
}

func validateGround(wheels map[string]omsi.Wheel, body scene.Bounds) bool {
	contacts := []float64{}
	for _, w := range wheels {
		if w.Radius > .08 && w.Radius < 1.5 {
			contacts = append(contacts, w.Z-w.Radius)
		}
	}
	if len(contacts) >= 2 {
		for _, c := range contacts {
			if math.Abs(c) > .12 {
				return false
			}
		}
		return true
	}
	return math.Abs(body.Min.Z) < .08
}

func detectLights(sc scene.Scene) ([]omsi.PointLight, string) {
	out := []omsi.PointLight{}
	for _, l := range sc.Locators {
		n := strings.ToLower(l.Name + " " + l.Hookup)
		var v string
		var r, g, b int
		dy := 1.0
		if strings.Contains(n, "head") || strings.Contains(n, "low_beam") {
			v = "AI_Light"
			r, g, b = 255, 245, 220
			dy = 1
		} else if strings.Contains(n, "brake") {
			v = "AI_Brakelight"
			r, g, b = 255, 0, 0
			dy = -1
		} else if strings.Contains(n, "tail") || strings.Contains(n, "rear_light") {
			v = "AI_Light"
			r, g, b = 255, 0, 0
			dy = -1
		} else if strings.Contains(n, "blinker") || strings.Contains(n, "indicator") || strings.Contains(n, "turn") {
			if l.Position.X < 0 {
				v = "lights_blinker_l"
			} else {
				v = "lights_blinker_r"
			}
			r, g, b = 255, 130, 0
			dy = map[bool]float64{true: 1, false: -1}[l.Position.Y > 0]
		} else if strings.Contains(n, "reverse") {
			v = "lights_rueckfahr"
			r, g, b = 255, 255, 235
			dy = -1
		}
		if v != "" {
			out = append(out, omsi.PointLight{X: l.Position.X, Y: l.Position.Y, Z: l.Position.Z, DY: dy, R: r, G: g, B: b, Size: .22, Variable: v, Strength: 1})
		}
	}
	if len(out) > 0 {
		return out, "ETS2 light/hookup locators"
	}
	return nil, "no high-confidence light locators; no guessed lights exported"
}
func validateAllO3DTextureRefs(stage string) []string {
	missing := []string{}
	seen := map[string]bool{}
	modelDir := filepath.Join(stage, "model")
	_ = filepath.WalkDir(modelDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".o3d") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		m, err := o3d.Parse(b)
		if err != nil {
			return nil
		}
		for _, mat := range m.Materials {
			tex := strings.TrimSpace(mat.Texture)
			if tex == "" {
				continue
			}
			name := filepath.Base(strings.ReplaceAll(tex, "\\", "/"))
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			if _, err := os.Stat(filepath.Join(stage, "texture", name)); err != nil {
				missing = append(missing, name)
			}
		}
		return nil
	})
	sort.Strings(missing)
	return missing
}

func validateOMSIOrientation(bodyO3D string, expected scene.Bounds) (bool, [3]float64) {
	var dims [3]float64
	b, err := os.ReadFile(bodyO3D)
	if err != nil {
		return false, dims
	}
	m, err := o3d.Parse(b)
	if err != nil || len(m.Vertices) == 0 {
		return false, dims
	}
	mn := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	mx := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, v := range m.Vertices {
		a := [3]float64{float64(v.X), float64(v.Y), float64(v.Z)}
		for i := 0; i < 3; i++ {
			if a[i] < mn[i] {
				mn[i] = a[i]
			}
			if a[i] > mx[i] {
				mx[i] = a[i]
			}
		}
	}
	for i := 0; i < 3; i++ {
		dims[i] = mx[i] - mn[i]
	}
	want := [3]float64{expected.Width(), expected.Height(), expected.Length()}
	for i := 0; i < 3; i++ {
		tol := math.Max(.025, want[i]*.015)
		if math.Abs(dims[i]-want[i]) > tol {
			return false, dims
		}
	}
	return true, dims
}

func listRelative(root string) []string {
	out := []string{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			if r, er := filepath.Rel(root, p); er == nil {
				out = append(out, filepath.ToSlash(r))
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func textReport(r Report) string {
	return fmt.Sprintf("ETS2OMSI V2.3.0 Conversion Report\r\nVehicle: %s\r\nStatus: %s\r\nOutput: %s\r\nModels: %d\r\nTextures copied: %d\r\nExact texture bindings: %d\r\nOn-demand package textures: %d\r\nUnresolved visible textures: %d\r\nDimensions LxWxH: %.3f x %.3f x %.3f m\r\nO3D XYZ dims: %.3f x %.3f x %.3f m\r\nOrientation: %t\r\nGround: %t\r\nWheel meshes: %d\r\nWheel basis: %s\r\nWarnings: %d\r\nErrors: %d\r\n", r.Name, r.Status, r.Output, len(r.Models), r.Textures.Copied, r.Textures.ExactResolved, r.Textures.OnDemandResolved, len(r.Textures.Unresolved), r.Auto.Length, r.Auto.Width, r.Auto.Height, r.Validation.O3DDimensions[0], r.Validation.O3DDimensions[1], r.Validation.O3DDimensions[2], r.Validation.OrientationOK, r.Validation.GroundOK, r.Validation.WheelMeshes, r.Auto.WheelBasis, len(r.Warnings), len(r.Errors))
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func buildManifest(root string) []ManifestEntry {
	out := []ManifestEntry{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(p) == "package_manifest.json" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		h := sha256.Sum256(b)
		rel, _ := filepath.Rel(root, p)
		out = append(out, ManifestEntry{Path: filepath.ToSlash(rel), Size: int64(len(b)), SHA256: hex.EncodeToString(h[:])})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// vehicleLook picks the ETS2 look for a model: the asset's own look, else the
// first look the traffic definition lists, else "" (PIT default/first look).
func vehicleLook(v scanner.Vehicle, assetLook string) string {
	if l := strings.TrimSpace(assetLook); l != "" {
		return l
	}
	for _, l := range v.Looks {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}
