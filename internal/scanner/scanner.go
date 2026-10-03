package scanner

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"ets2omsi/internal/archive"
	"ets2omsi/internal/graph"
	"ets2omsi/internal/sii"
)

const Version = "V2.6.0"

type parsedFile struct {
	doc sii.Document
	err error
}

type unitRef struct {
	path string
	unit sii.Unit
}

type engine struct {
	src         archive.Source
	parsed      map[string]parsedFile
	unitIndex   map[string][]unitRef
	unitIndexed bool
	files       map[string]string // logical lower path -> actual archive path
	logical     []archive.FileInfo
	rootPrefix  string
	warnings    []string
	diag        Diagnostics
}

func Scan(src archive.Source) (PackageReport, error) {
	e := newEngine(src)
	storages := e.findStorageFiles()
	byKey := map[string]*Vehicle{}

	// Primary, authoritative discovery: every traffic_storage*.sii variant, including
	// traffic_storage.something.sii and traffic_storage_car.something.sii.
	for _, storage := range storages {
		docs, _ := e.expandIncludes(storage, map[string]bool{})
		storageType := typeFromStorage(storage)
		for _, doc := range docs {
			for _, u := range doc.Units {
				if !strings.EqualFold(u.Type, "traffic_vehicle") {
					continue
				}
				key := vehicleKey(u, doc.Path)
				if _, exists := byKey[key]; exists {
					continue
				}
				chain := e.findIncludePath(storage, doc.Path, map[string]bool{})
				if len(chain) == 0 {
					chain = []string{storage, doc.Path}
				}
				vtype := vehicleTypeFromUnit(u, storageType)
				v := e.buildVehicle(u, doc.Path, storage, vtype, true, chain)
				v.DiscoveryBasis = "traffic storage include graph"
				byKey[key] = &v
			}
		}
	}

	// Legacy bridge: older ETS2 traffic packs (notably pre/around the 1.19 storage
	// transition) can use traffic_storage*.sii files whose directly included vehicle
	// definitions do not expose a modern traffic_vehicle root.  We only promote a
	// legacy root when BOTH conditions are true: (1) it is directly included by a
	// discovered traffic storage file and (2) the definition contains an explicit PMD
	// model reference.  This keeps discovery evidence-based instead of filename-based.
	e.discoverLegacyStorageVehicles(storages, byKey)

	// Secondary discovery: vehicle-related definitions can still be inspected even if
	// a mod uses an unusual storage chain. These are marked unbound, never silently
	// promoted to storage-bound vehicles.
	for _, fi := range e.logical {
		p := graph.Normalize(fi.Path)
		lp := strings.ToLower(p)
		if !(strings.HasPrefix(lp, "/def/vehicle/") && (strings.HasSuffix(lp, ".sii") || strings.HasSuffix(lp, ".sui"))) {
			continue
		}
		doc, err := e.parse(p)
		if err != nil {
			continue
		}
		for _, u := range doc.Units {
			if !strings.EqualFold(u.Type, "traffic_vehicle") {
				continue
			}
			key := vehicleKey(u, p)
			if _, ok := byKey[key]; ok {
				continue
			}
			v := e.buildVehicle(u, p, "", vehicleTypeFromUnit(u, "unknown"), false, []string{p})
			v.DiscoveryBasis = "unbound traffic_vehicle definition"
			v.Warnings = append(v.Warnings, "Definition is not referenced by a discovered traffic_storage*.sii file")
			byKey[key] = &v
		}
	}

	vehicles := make([]Vehicle, 0, len(byKey))
	for _, v := range byKey {
		vehicles = append(vehicles, *v)
	}
	sort.Slice(vehicles, func(i, j int) bool {
		if vehicles[i].VehicleType == vehicles[j].VehicleType {
			return vehicles[i].DisplayName < vehicles[j].DisplayName
		}
		return vehicles[i].VehicleType < vehicles[j].VehicleType
	})

	// Model-linked chassis candidates are also used by V2 as a package-local bridge.
	// Older traffic packs often keep the traffic_vehicle and accessory_chassis_data
	// in separate files without an explicit file-path edge.  We only bridge when the
	// relationship is deterministic: exact unit suffix (traffic.foo -> chassis.foo)
	// or a unique chassis directory matching the traffic definition stem.
	candidates := e.discoverCandidates(vehicles, storages)
	e.diag.PackageModelBridges = e.bridgePackageCandidates(vehicles, candidates)

	// Shared dependency analysis.
	use := map[string]int{}
	for _, v := range vehicles {
		seen := map[string]bool{}
		for _, n := range v.Dependencies {
			if !seen[n.Path] {
				use[n.Path]++
				seen[n.Path] = true
			}
		}
	}
	shared := []string{}
	for p, n := range use {
		if n > 1 {
			shared = append(shared, p)
		}
	}
	sort.Strings(shared)
	sharedSet := map[string]bool{}
	for _, p := range shared {
		sharedSet[p] = true
	}
	for i := range vehicles {
		for j := range vehicles[i].Dependencies {
			if sharedSet[vehicles[i].Dependencies[j].Path] {
				vehicles[i].Dependencies[j].Shared = true
			}
		}
	}

	e.collectDiagnostics()
	stats := Stats{VehicleCount: len(vehicles), UniqueDependencies: len(use), CandidateCount: len(candidates)}
	for _, v := range vehicles {
		if v.Readiness == "ready" {
			stats.ReadyCount++
		}
		if len(v.Warnings) > 0 || len(v.Missing) > 0 {
			stats.WarningCount++
		}
		if !v.Bound {
			stats.UnboundCount++
		}
	}
	if len(vehicles) == 0 {
		e.warnings = append(e.warnings, fmt.Sprintf(
			"No finalized traffic_vehicle roots found. Diagnostics: %d storage file(s), %d vehicle definition file(s), %d traffic_vehicle unit(s), %d chassis candidate(s).",
			e.diag.TrafficStorageFiles, e.diag.VehicleDefinitionFiles, e.diag.TrafficVehicleUnits, len(candidates)))
	}

	return PackageReport{
		Version: Version, Package: src.Name(), Kind: src.Kind(), FileCount: len(src.List()),
		Vehicles: vehicles, Candidates: candidates, SharedAssets: shared,
		Warnings: uniqueText(e.warnings), Diagnostics: e.diag, Stats: stats,
	}, nil
}

func newEngine(src archive.Source) *engine {
	e := &engine{src: src, parsed: map[string]parsedFile{}, files: map[string]string{}}
	list := src.List()
	e.rootPrefix = detectRootPrefix(list)
	e.diag.LogicalRootPrefix = e.rootPrefix
	top := map[string]bool{}
	for _, f := range list {
		actual := graph.Normalize(f.Path)
		logical := logicalPath(actual, e.rootPrefix)
		// Logical game-root path wins. Also index actual path for reports/manual inspection.
		if _, ok := e.files[strings.ToLower(logical)]; !ok {
			e.files[strings.ToLower(logical)] = actual
		}
		e.files[strings.ToLower(actual)] = actual
		e.logical = append(e.logical, archive.FileInfo{Path: logical, Size: f.Size})
		parts := strings.Split(strings.TrimPrefix(logical, "/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			top[parts[0]] = true
		}
		switch strings.ToLower(filepath.Ext(logical)) {
		case ".sii":
			e.diag.SIIFiles++
		case ".sui":
			e.diag.SUIFiles++
		case ".pmd":
			e.diag.PMDFiles++
		case ".pmg":
			e.diag.PMGFiles++
		case ".pmc":
			e.diag.PMCFiles++
		case ".mat":
			e.diag.MATFiles++
		case ".tobj":
			e.diag.TOBJFiles++
		case ".dds", ".png", ".tga", ".jpg", ".jpeg":
			e.diag.TextureFiles++
		}
		lp := strings.ToLower(logical)
		if strings.HasPrefix(lp, "/def/vehicle/") && (strings.HasSuffix(lp, ".sii") || strings.HasSuffix(lp, ".sui")) {
			e.diag.VehicleDefinitionFiles++
		}
	}
	sort.Slice(e.logical, func(i, j int) bool { return e.logical[i].Path < e.logical[j].Path })
	for k := range top {
		e.diag.TopLevelFolders = append(e.diag.TopLevelFolders, k)
	}
	sort.Strings(e.diag.TopLevelFolders)
	return e
}

func detectRootPrefix(list []archive.FileInfo) string {
	counts := map[string]int{}
	for _, f := range list {
		p := strings.ToLower(graph.Normalize(f.Path))
		if i := strings.Index(p, "/def/vehicle/"); i >= 0 {
			prefix := graph.Normalize(f.Path)[:i]
			counts[prefix]++
		}
	}
	best, bestN := "", 0
	for p, n := range counts {
		if n > bestN || (n == bestN && len(p) < len(best)) {
			best, bestN = p, n
		}
	}
	return strings.TrimSuffix(best, "/")
}
func logicalPath(actual, prefix string) string {
	actual = graph.Normalize(actual)
	if prefix == "" || prefix == "/" {
		return actual
	}
	pl := strings.ToLower(strings.TrimSuffix(prefix, "/"))
	al := strings.ToLower(actual)
	if al == pl {
		return "/"
	}
	if strings.HasPrefix(al, pl+"/") {
		return graph.Normalize(actual[len(prefix):])
	}
	return actual
}
func (e *engine) resolve(p string) (string, bool) {
	p = graph.Normalize(p)
	actual, ok := e.files[strings.ToLower(p)]
	return actual, ok
}
func (e *engine) exists(p string) bool { _, ok := e.resolve(p); return ok }
func (e *engine) read(p string) ([]byte, error) {
	actual, ok := e.resolve(p)
	if !ok {
		return nil, fmt.Errorf("file not found: %s", p)
	}
	return e.src.Read(actual)
}

func (e *engine) findStorageFiles() []string {
	out := []string{}
	for _, f := range e.logical {
		p := graph.Normalize(f.Path)
		b := strings.ToLower(path.Base(p))
		dir := strings.ToLower(path.Dir(p))
		// Official forms include traffic_storage_car.sii and traffic_storage_car.mymod.sii.
		// Older real-world packs also used traffic_storage.jazzycat.sii.
		if strings.HasPrefix(dir, "/def/vehicle") && strings.HasPrefix(b, "traffic_storage") && strings.HasSuffix(b, ".sii") {
			out = append(out, p)
		}
	}
	out = unique(out)
	e.diag.TrafficStorageFiles = len(out)
	e.diag.TrafficStoragePaths = append([]string(nil), out...)
	return out
}

func typeFromStorage(p string) string {
	b := strings.TrimSuffix(strings.ToLower(path.Base(p)), ".sii")
	if !strings.HasPrefix(b, "traffic_storage") {
		return "unknown"
	}
	rest := strings.TrimPrefix(b, "traffic_storage")
	if strings.HasPrefix(rest, "_") {
		rest = strings.TrimPrefix(rest, "_")
		if i := strings.IndexByte(rest, '.'); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" {
			return rest
		}
	}
	return "unknown"
}
func vehicleTypeFromUnit(u sii.Unit, fallback string) string {
	if t := strings.ToLower(strings.TrimSpace(sii.First(u, "type"))); t != "" {
		switch t {
		case "car", "truck", "bus", "motorcycle":
			return t
		}
	}
	if fallback == "semi_trailer" {
		return "trailer"
	}
	if fallback == "" {
		return "unknown"
	}
	return fallback
}
func vehicleKey(u sii.Unit, root string) string {
	if strings.TrimSpace(u.Name) != "" {
		return strings.ToLower(strings.TrimSpace(u.Name))
	}
	return strings.ToLower(root)
}

func (e *engine) parse(p string) (sii.Document, error) {
	p = graph.Normalize(p)
	k := strings.ToLower(p)
	if x, ok := e.parsed[k]; ok {
		return x.doc, x.err
	}
	data, err := e.read(p)
	if err != nil {
		e.parsed[k] = parsedFile{err: err}
		e.diag.ParseErrors++
		e.diag.ParseErrorPaths = append(e.diag.ParseErrorPaths, p)
		return sii.Document{}, err
	}
	d, err := sii.Parse(p, data)
	e.parsed[k] = parsedFile{doc: d, err: err}
	if err != nil {
		e.diag.ParseErrors++
		e.diag.ParseErrorPaths = append(e.diag.ParseErrorPaths, p)
		if errors.Is(err, sii.ErrEncoded) {
			e.diag.EncodedFiles++
			e.warnings = append(e.warnings, fmt.Sprintf("Encoded/non-text SII/SUI could not be parsed directly: %s", p))
		} else {
			e.warnings = append(e.warnings, fmt.Sprintf("SII parse error: %s: %v", p, err))
		}
	}
	return d, err
}

func (e *engine) collectDiagnostics() {
	e.diag.UnitTypes = map[string]int{}
	includeCount, trafficCount, chassisCount := 0, 0, 0
	for _, fi := range e.logical {
		p := graph.Normalize(fi.Path)
		lp := strings.ToLower(p)
		if !(strings.HasPrefix(lp, "/def/vehicle/") && (strings.HasSuffix(lp, ".sii") || strings.HasSuffix(lp, ".sui"))) {
			continue
		}
		doc, err := e.parse(p)
		if err != nil {
			continue
		}
		includeCount += len(doc.Includes)
		for _, u := range doc.Units {
			ut := strings.ToLower(u.Type)
			e.diag.UnitTypes[ut]++
			if ut == "traffic_vehicle" {
				trafficCount++
			}
			if ut == "accessory_chassis_data" {
				chassisCount++
			}
		}
	}
	e.diag.IncludeDirectives = includeCount
	e.diag.TrafficVehicleUnits = trafficCount
	e.diag.ChassisUnits = chassisCount
}

func (e *engine) expandIncludes(root string, seen map[string]bool) ([]sii.Document, []string) {
	root = graph.Normalize(root)
	k := strings.ToLower(root)
	if seen[k] {
		return nil, nil
	}
	seen[k] = true
	doc, err := e.parse(root)
	if err != nil {
		return nil, []string{root}
	}
	docs := []sii.Document{doc}
	chain := []string{root}
	for _, inc := range doc.Includes {
		if !e.exists(inc) {
			e.warnings = append(e.warnings, fmt.Sprintf("Missing include: %s -> %s", root, inc))
			continue
		}
		sub, subc := e.expandIncludes(inc, seen)
		docs = append(docs, sub...)
		chain = append(chain, subc...)
	}
	return docs, unique(chain)
}

func (e *engine) findIncludePath(start, target string, seen map[string]bool) []string {
	start = graph.Normalize(start)
	target = graph.Normalize(target)
	if strings.EqualFold(start, target) {
		return []string{start}
	}
	k := strings.ToLower(start)
	if seen[k] {
		return nil
	}
	seen[k] = true
	doc, err := e.parse(start)
	if err != nil {
		return nil
	}
	for _, inc := range doc.Includes {
		if strings.EqualFold(inc, target) {
			return []string{start, inc}
		}
		if !e.exists(inc) {
			continue
		}
		if sub := e.findIncludePath(inc, target, seen); len(sub) > 0 {
			return append([]string{start}, sub...)
		}
	}
	return nil
}

func (e *engine) buildVehicle(u sii.Unit, root, storage, vtype string, bound bool, includeChain []string) Vehicle {
	g := graph.New()
	root = graph.Normalize(root)
	g.AddNode(root, true)
	if storage != "" {
		g.AddNode(storage, true)
		g.AddEdge(storage, root, "storage_include_or_unit", "explicit storage traversal", true)
	}
	for i, p := range includeChain {
		g.AddNode(p, e.exists(p))
		if i > 0 {
			prev := includeChain[i-1]
			if !strings.EqualFold(prev, p) {
				g.AddEdge(prev, p, "include", "explicit @include traversal", e.exists(p))
			}
		}
	}
	display := firstUseful(u, "name", "display_name", "brand")
	if display == "" {
		display = prettyID(u.Name)
	}
	variants := uniqueText(append(sii.UnitValues(u, "variant"), sii.UnitValues(u, "variants")...))
	looks := uniqueText(append(sii.UnitValues(u, "look"), sii.UnitValues(u, "looks")...))
	missing, warnings := []string{}, []string{}
	models := []string{}
	modelAssets := []ModelAsset{}

	for _, f := range u.Fields {
		for _, p := range sii.PathsInValueAt(root, f.Value) {
			ex := e.exists(p)
			g.AddEdge(root, p, reasonForField(f.Key), "explicit SII field", ex)
			if strings.HasSuffix(strings.ToLower(p), ".pmd") {
				models = append(models, p)
				if role := modelRoleForField(f.Key); role != "" {
					modelAssets = appendModelAsset(modelAssets, ModelAsset{Path: p, Role: role, Variant: firstUseful(u, "variant"), Look: firstUseful(u, "look"), SourceDefinition: root})
				}
			}
			if !ex {
				missing = append(missing, p)
			}
		}
	}

	// A real ETS2 traffic file may keep traffic_vehicle, vehicle_accessory and
	// vehicle_wheel_accessory units in the same SII document and connect them by
	// unit IDs rather than file paths. Follow only explicit unit-name references;
	// never infer a PMD from a vehicle name.
	var wheels []WheelAttachment
	extraChassis := []string{}
	if doc, err := e.parse(root); err == nil {
		unitMap := map[string]sii.Unit{}
		for _, du := range doc.Units {
			if strings.TrimSpace(du.Name) != "" {
				unitMap[strings.ToLower(strings.TrimSpace(du.Name))] = du
			}
		}
		e.followUnitReferences(g, root, u, unitMap, map[string]bool{}, &models, &variants, &looks, &missing)
		// Some legacy traffic packs keep vehicle_accessory / chassis unit IDs in
		// separate SII files. Resolve only exact, unambiguous unit-ID references
		// across /def/vehicle; no filename or model-path guessing is used.
		e.followGlobalUnitReferences(g, root, u, map[string]bool{}, &models, &variants, &looks, &missing)
		wheels, extraChassis = e.resolveStructuredAccessories(root, u, unitMap, g, &missing)
	}

	chassis := append([]string{}, extraChassis...)
	for _, ed := range g.Edges {
		if strings.Contains(strings.ToLower(ed.To), "chassis") && (strings.HasSuffix(strings.ToLower(ed.To), ".sii") || strings.HasSuffix(strings.ToLower(ed.To), ".sui")) {
			chassis = append(chassis, ed.To)
		}
	}
	conv := documentedChassisPath(root)
	if conv != "" && e.exists(conv) {
		chassis = append(chassis, conv)
		g.AddEdge(root, conv, "chassis", "documented SCS AI vehicle layout", true)
	}
	chassis = unique(chassis)

	for _, cp := range chassis {
		doc, err := e.parse(cp)
		if err != nil {
			warnings = append(warnings, "Unable to parse chassis: "+cp)
			continue
		}
		for _, inc := range doc.Includes {
			ex := e.exists(inc)
			g.AddEdge(cp, inc, "include", "explicit @include", ex)
			if !ex {
				missing = append(missing, inc)
			}
		}
		for _, cu := range doc.Units {
			variants = append(variants, sii.UnitValues(cu, "variant")...)
			looks = append(looks, sii.UnitValues(cu, "look")...)
			cv := firstUseful(cu, "variant")
			cl := firstUseful(cu, "look")
			for _, f := range cu.Fields {
				for _, p := range sii.PathsInValueAt(cp, f.Value) {
					ex := e.exists(p)
					g.AddEdge(cp, p, reasonForField(f.Key), "explicit chassis field", ex)
					if strings.HasSuffix(strings.ToLower(p), ".pmd") {
						models = append(models, p)
						if role := modelRoleForField(f.Key); role != "" {
							modelAssets = appendModelAsset(modelAssets, ModelAsset{Path: p, Role: role, Variant: cv, Look: cl, SourceDefinition: cp})
						}
					}
					if !ex {
						missing = append(missing, p)
					}
				}
			}
		}
	}

	boundaries := map[string]bool{strings.ToLower(root): true}
	if storage != "" {
		boundaries[strings.ToLower(graph.Normalize(storage))] = true
	}
	for _, p := range includeChain {
		boundaries[strings.ToLower(graph.Normalize(p))] = true
	}
	e.expandTextDeps(g, boundaries, map[string]bool{}, &missing)
	nodes := g.SortedNodes()
	wheelSet := map[string]bool{}
	wheelModels := []string{}
	for _, w := range wheels {
		if w.Model != "" {
			wp := graph.Normalize(w.Model)
			wheelSet[strings.ToLower(wp)] = true
			wheelModels = append(wheelModels, wp)
		}
	}
	// If structured SCS fields gave us no semantic model list (some legacy packs),
	// fall back to explicit model nodes while still excluding known wheel PMDs.
	if len(modelAssets) == 0 {
		for _, n := range nodes {
			if n.Type == graph.NodeModel && !wheelSet[strings.ToLower(n.Path)] {
				models = append(models, n.Path)
			}
		}
	}
	models = uniqueOrdered(models)
	filteredModels := models[:0]
	for _, m := range models {
		if !wheelSet[strings.ToLower(graph.Normalize(m))] {
			filteredModels = append(filteredModels, m)
		}
	}
	models = filteredModels
	// ModelAssets are authoritative when present. Keep the compatibility Models list
	// in the same semantic order: main first, then LODs, then optional detail/accessory.
	if len(modelAssets) > 0 {
		modelAssets = normalizeModelAssets(modelAssets)
		models = models[:0]
		for _, ma := range modelAssets {
			if !wheelSet[strings.ToLower(graph.Normalize(ma.Path))] {
				models = append(models, ma.Path)
			}
		}
		models = uniqueOrdered(models)
	}
	wheelModels = uniqueOrdered(wheelModels)
	variants = uniqueText(variants)
	looks = uniqueText(looks)
	missing = unique(missing)

	c := Counts{Dependencies: len(nodes), Variants: len(variants), Missing: len(missing), Wheels: len(wheels), WheelModels: len(wheelModels)}
	for _, n := range nodes {
		switch n.Type {
		case graph.NodeMaterial:
			c.Materials++
		case graph.NodeTexture, graph.NodeTextureObj:
			c.Textures++
		}
	}
	c.Models = len(models)
	if len(models) == 0 {
		warnings = append(warnings, "No PMD body model reference has been resolved yet")
	}
	if len(missing) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d dependency path(s) are missing from this package/mount", len(missing)))
	}
	readiness, blocking, sharedReq := classifyReadiness(bound, models, missing)
	return Vehicle{ID: u.Name, DisplayName: display, VehicleType: vtype, RootDefinition: root, StorageDefinition: storage, Bound: bound, Chassis: chassis, Models: models, ModelAssets: modelAssets, WheelAttachments: wheels, WheelModels: wheelModels, Variants: variants, Looks: looks, Dependencies: nodes, Edges: g.Edges, Missing: missing, Warnings: uniqueText(warnings), Counts: c, Readiness: readiness, Blocking: blocking, SharedRequirements: sharedReq}
}

func modelRoleForField(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "model":
		return "main"
	case "detail_model":
		return "detail"
	case "lod", "lods":
		return "lod"
	case "exterior_model":
		return "accessory"
	default:
		if strings.HasPrefix(k, "lod") && !strings.Contains(k, "collision") {
			return "lod"
		}
	}
	return ""
}

func appendModelAsset(in []ModelAsset, x ModelAsset) []ModelAsset {
	x.Path = graph.Normalize(x.Path)
	for i := range in {
		if strings.EqualFold(in[i].Path, x.Path) {
			// Prefer a stronger semantic role over an earlier generic/accessory role.
			if in[i].Role == "accessory" && x.Role != "accessory" {
				in[i] = x
			}
			return in
		}
	}
	return append(in, x)
}

func normalizeModelAssets(in []ModelAsset) []ModelAsset {
	out := []ModelAsset{}
	for _, x := range in {
		out = appendModelAsset(out, x)
	}
	rank := func(r string) int {
		switch r {
		case "main":
			return 0
		case "lod":
			return 1
		case "detail":
			return 2
		case "accessory":
			return 3
		}
		return 4
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i].Role) < rank(out[j].Role) })
	return out
}

// resolveStructuredAccessories follows only unit IDs explicitly referenced by the
// traffic_vehicle. This captures the documented vehicle_wheel_accessory/data_path
// chain and separates wheel PMDs from body LODs.
func (e *engine) resolveStructuredAccessories(root string, start sii.Unit, units map[string]sii.Unit, g *graph.Graph, missing *[]string) ([]WheelAttachment, []string) {
	wheels := []WheelAttachment{}
	chassis := []string{}
	seen := map[string]bool{}
	var walk func(sii.Unit)
	walk = func(u sii.Unit) {
		uk := strings.ToLower(strings.TrimSpace(u.Name))
		if uk != "" {
			if seen[uk] {
				return
			}
			seen[uk] = true
		}
		ut := strings.ToLower(strings.TrimSpace(u.Type))
		if ut == "vehicle_wheel_accessory" {
			dp := strings.TrimSpace(sii.First(u, "data_path"))
			if dp != "" {
				dp = sii.Resolve(root, dp)
				ex := e.exists(dp)
				g.AddEdge(root, dp, "wheel_data", "vehicle_wheel_accessory.data_path", ex)
				if !ex {
					*missing = append(*missing, dp)
				}
				off, _ := strconv.Atoi(strings.TrimSpace(sii.First(u, "offset")))
				wa := WheelAttachment{UnitID: u.Name, DataPath: dp, Offset: off, Family: wheelFamily(dp)}
				if ex {
					if d, err := e.parse(dp); err == nil {
						for _, wu := range d.Units {
							wt := strings.ToLower(strings.TrimSpace(wu.Type))
							if wt != "accessory_wheel_data" && wt != "accessory_rim_data" {
								continue
							}
							wa.Look = firstUseful(wu, "look")
							wa.Variant = firstUseful(wu, "variant")
							if mp := strings.TrimSpace(sii.First(wu, "model")); mp != "" {
								mp = sii.Resolve(dp, mp)
								wa.Model = mp
								mex := e.exists(mp)
								g.AddEdge(dp, mp, "wheel_model", "accessory_wheel_data.model", mex)
								if !mex {
									*missing = append(*missing, mp)
								}
							}
							break
						}
					}
				}
				wheels = append(wheels, wa)
			}
		}
		if ut == "vehicle_accessory" {
			if dp := strings.TrimSpace(sii.First(u, "data_path")); dp != "" {
				dp = sii.Resolve(root, dp)
				if e.exists(dp) {
					if d, err := e.parse(dp); err == nil {
						for _, au := range d.Units {
							if strings.EqualFold(au.Type, "accessory_chassis_data") {
								chassis = append(chassis, dp)
								break
							}
						}
					}
				}
			}
		}
		for _, f := range u.Fields {
			ref := unitRefValue(f.Value)
			if next, ok := units[ref]; ok {
				walk(next)
			}
		}
	}
	walk(start)
	return uniqueWheels(wheels), unique(chassis)
}

func wheelFamily(p string) string {
	p = strings.ToLower(graph.Normalize(p))
	switch {
	case strings.Contains(p, "/f_wheel/"):
		return "f"
	case strings.Contains(p, "/r_wheel/"):
		return "r"
	case strings.Contains(p, "/t_wheel/") || strings.Contains(p, "/tr_wheel/"):
		return "t"
	case strings.Contains(p, "/ai_wheel/"):
		return "ai"
	}
	return "unknown"
}

func uniqueWheels(in []WheelAttachment) []WheelAttachment {
	out := []WheelAttachment{}
	seen := map[string]bool{}
	for _, w := range in {
		k := strings.ToLower(w.UnitID + "|" + w.DataPath + "|" + strconv.Itoa(w.Offset) + "|" + w.Model)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, w)
	}
	return out
}

func uniqueOrdered(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = graph.Normalize(s)
		if s == "" || s == "/" {
			continue
		}
		k := strings.ToLower(s)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}

func unitRefValue(v string) string {
	v = strings.Trim(strings.TrimSpace(v), "\"'[](),")
	return strings.ToLower(v)
}

func (e *engine) followUnitReferences(g *graph.Graph, root string, start sii.Unit, units map[string]sii.Unit, seen map[string]bool, models, variants, looks, missing *[]string) {
	key := strings.ToLower(strings.TrimSpace(start.Name))
	if key != "" {
		if seen[key] {
			return
		}
		seen[key] = true
	}
	*variants = append(*variants, sii.UnitValues(start, "variant")...)
	*looks = append(*looks, sii.UnitValues(start, "look")...)
	for _, f := range start.Fields {
		for _, p := range sii.PathsInValueAt(root, f.Value) {
			ex := e.exists(p)
			g.AddEdge(root, p, reasonForField(f.Key), "explicit same-document unit field", ex)
			if strings.HasSuffix(strings.ToLower(p), ".pmd") {
				*models = append(*models, p)
			}
			if !ex {
				*missing = append(*missing, p)
			}
		}
		ref := unitRefValue(f.Value)
		if ref == "" {
			continue
		}
		if next, ok := units[ref]; ok {
			// The edge records the unit relation without inventing a fake filesystem node.
			// Asset paths exposed by the target unit are attached to the real root document.
			e.followUnitReferences(g, root, next, units, seen, models, variants, looks, missing)
		}
	}
}

func (e *engine) buildGlobalUnitIndex() {
	if e.unitIndexed {
		return
	}
	e.unitIndexed = true
	e.unitIndex = map[string][]unitRef{}
	for _, fi := range e.logical {
		p := graph.Normalize(fi.Path)
		lp := strings.ToLower(p)
		if !strings.HasPrefix(lp, "/def/vehicle/") || !(strings.HasSuffix(lp, ".sii") || strings.HasSuffix(lp, ".sui")) {
			continue
		}
		doc, err := e.parse(p)
		if err != nil {
			continue
		}
		for _, u := range doc.Units {
			name := strings.ToLower(strings.TrimSpace(u.Name))
			if name == "" {
				continue
			}
			e.unitIndex[name] = append(e.unitIndex[name], unitRef{path: p, unit: u})
		}
	}
}

func (e *engine) followGlobalUnitReferences(g *graph.Graph, currentPath string, start sii.Unit, seen map[string]bool, models, variants, looks, missing *[]string) {
	e.buildGlobalUnitIndex()
	visitKey := strings.ToLower(graph.Normalize(currentPath) + "|" + strings.TrimSpace(start.Name))
	if seen[visitKey] {
		return
	}
	seen[visitKey] = true
	*variants = append(*variants, sii.UnitValues(start, "variant")...)
	*looks = append(*looks, sii.UnitValues(start, "look")...)
	for _, f := range start.Fields {
		for _, p := range sii.PathsInValueAt(currentPath, f.Value) {
			ex := e.exists(p)
			g.AddEdge(currentPath, p, reasonForField(f.Key), "explicit global unit field", ex)
			if strings.HasSuffix(strings.ToLower(p), ".pmd") {
				*models = append(*models, p)
			}
			if !ex {
				*missing = append(*missing, p)
			}
		}
		ref := unitRefValue(f.Value)
		if ref == "" {
			continue
		}
		matches := e.unitIndex[ref]
		// Ambiguous unit IDs are deliberately ignored. Choosing one would turn the
		// resolver back into a heuristic and can cross-link unrelated vehicles.
		if len(matches) != 1 {
			continue
		}
		target := matches[0]
		if strings.EqualFold(target.path, currentPath) && strings.EqualFold(target.unit.Name, start.Name) {
			continue
		}
		g.AddEdge(currentPath, target.path, "unit_reference", "explicit unambiguous SII unit id", true)
		e.diag.GlobalUnitRefsResolved++
		e.followGlobalUnitReferences(g, target.path, target.unit, seen, models, variants, looks, missing)
	}
}

func classifyReadiness(bound bool, models, missing []string) (string, []string, []string) {
	blocking := []string{}
	optional := []string{}
	if len(models) == 0 {
		blocking = append(blocking, "no PMD model reference resolved")
	}
	for _, p := range missing {
		lp := strings.ToLower(p)
		// V2 is SCS-only. ETS2 runtime helpers (light masks, shared shadows, global
		// materials) are optional for a basic OMSI AI exterior conversion. Keep them
		// visible as diagnostics, but never require an ETS2 installation.
		if strings.HasPrefix(lp, "/material/") || strings.Contains(lp, "shadow") || strings.Contains(lp, "trucklight") || strings.HasPrefix(lp, "/vehicle/ai/share/") || strings.HasPrefix(lp, "/vehicle/share/") {
			optional = append(optional, p)
			continue
		}
		if strings.HasSuffix(lp, ".pmd") || strings.HasSuffix(lp, ".pmg") {
			blocking = append(blocking, p)
		} else {
			optional = append(optional, p)
		}
	}
	blocking = uniqueText(blocking)
	optional = unique(optional)
	if len(blocking) == 0 {
		return "ready", blocking, optional
	}
	if len(models) == 0 {
		return "model_missing", blocking, optional
	}
	return "blocked", blocking, optional
}

func (e *engine) expandTextDeps(g *graph.Graph, boundaries map[string]bool, seen map[string]bool, missing *[]string) {
	queue := []string{}
	for p := range g.Nodes {
		if boundaries[strings.ToLower(graph.Normalize(p))] {
			continue
		}
		queue = append(queue, p)
	}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		k := strings.ToLower(p)
		if seen[k] {
			continue
		}
		seen[k] = true
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".sii" && ext != ".sui" && ext != ".mat" && ext != ".tobj" {
			continue
		}
		if !e.exists(p) {
			continue
		}
		data, err := e.read(p)
		if err != nil {
			continue
		}
		if ext == ".sii" || ext == ".sui" {
			doc, err := sii.Parse(p, data)
			if err != nil {
				continue
			}
			for _, inc := range doc.Includes {
				ex := e.exists(inc)
				g.AddEdge(p, inc, "include", "explicit @include", ex)
				if !ex {
					*missing = append(*missing, inc)
				} else {
					queue = append(queue, inc)
				}
			}
			for _, u := range doc.Units {
				for _, f := range u.Fields {
					for _, ref := range sii.PathsInValueAt(p, f.Value) {
						ex := e.exists(ref)
						g.AddEdge(p, ref, reasonForField(f.Key), "explicit SII field", ex)
						if !ex {
							*missing = append(*missing, ref)
						} else {
							queue = append(queue, ref)
						}
					}
				}
			}
			continue
		}
		text := string(data)
		for _, ref := range sii.PathsInValueAt(p, text) {
			ex := e.exists(ref)
			g.AddEdge(p, ref, "asset_reference", "explicit textual path", ex)
			if !ex {
				*missing = append(*missing, ref)
			} else {
				queue = append(queue, ref)
			}
		}
	}
}

func (e *engine) discoverLegacyStorageVehicles(storages []string, byKey map[string]*Vehicle) {
	for _, storage := range storages {
		sdoc, err := e.parse(storage)
		if err != nil {
			continue
		}
		stype := typeFromStorage(storage)
		e.diag.StorageDirectIncludes += len(sdoc.Includes)
		for _, inc := range sdoc.Includes {
			inc = graph.Normalize(inc)
			if !e.exists(inc) {
				continue
			}
			// A direct storage include is the strongest legacy vehicle boundary we have.
			// Never walk into chassis/material helper files here; recursive dependencies are
			// added later by buildVehicle/expandTextDeps.
			doc, err := e.parse(inc)
			if err != nil {
				continue
			}
			chain := []string{storage, inc}
			promoted := false
			modelLinked := false
			for idx, u := range doc.Units {
				ut := strings.ToLower(strings.TrimSpace(u.Type))
				if ut == "traffic_vehicle" {
					continue // authoritative path already handled above
				}
				if strings.HasPrefix(ut, "accessory_") || strings.Contains(ut, "vehicle_accessory") {
					continue // helper/accessory unit is not a vehicle root by itself
				}
				refs := explicitPMDRefs(u, inc)
				if len(refs) == 0 {
					continue
				}
				modelLinked = true
				key := legacyVehicleKey(storage, inc, u, idx)
				if _, exists := byKey[key]; exists {
					continue
				}
				v := e.buildVehicle(u, inc, storage, vehicleTypeFromUnit(u, stype), true, chain)
				if len(v.Models) == 0 {
					continue
				}
				v.DiscoveryBasis = "legacy storage direct include + explicit PMD reference"
				v.Warnings = append(v.Warnings, "Legacy ETS2 traffic definition: modern traffic_vehicle root was not present; vehicle was bound by storage include + explicit model evidence")
				byKey[key] = &v
				e.diag.LegacyStorageRoots++
				promoted = true
			}
			if modelLinked {
				e.diag.LegacyModelLinkedIncludes++
			}
			if promoted {
				continue
			}
			if len(doc.Units) > 0 {
				continue // parsed helper/non-root units are not reinterpreted as vehicles
			}

			// Some legacy files are syntactically odd enough that fields may not become a
			// parsed unit even though explicit PMD paths remain visible in the text.  In that
			// case create one synthetic root from those explicit paths only.  No path guessing.
			data, rerr := e.read(inc)
			if rerr != nil {
				continue
			}
			refs := explicitPMDPathsInText(inc, string(data))
			if len(refs) == 0 {
				continue
			}
			e.diag.LegacyModelLinkedIncludes++
			stem := strings.TrimSuffix(path.Base(inc), path.Ext(inc))
			u := sii.Unit{Type: "legacy_storage_vehicle", Name: "legacy." + stem}
			for _, ref := range refs {
				u.Fields = append(u.Fields, sii.Field{Key: "model", Value: ref})
			}
			key := strings.ToLower("legacy|" + storage + "|" + inc)
			if _, exists := byKey[key]; exists {
				continue
			}
			v := e.buildVehicle(u, inc, storage, stype, true, chain)
			if len(v.Models) == 0 {
				continue
			}
			v.DisplayName = prettyID(stem)
			v.DiscoveryBasis = "legacy storage direct include + raw explicit PMD reference"
			v.Warnings = append(v.Warnings, "Legacy definition required raw PMD-path recovery; no model path was guessed")
			byKey[key] = &v
			e.diag.LegacyStorageRoots++
		}
	}
}

func explicitPMDRefs(u sii.Unit, current string) []string {
	refs := []string{}
	for _, f := range u.Fields {
		for _, r := range sii.PathsInValueAt(current, f.Value) {
			if strings.HasSuffix(strings.ToLower(r), ".pmd") {
				refs = append(refs, r)
			}
		}
	}
	return unique(refs)
}

func explicitPMDPathsInText(current, text string) []string {
	refs := []string{}
	for _, r := range sii.PathsInValueAt(current, text) {
		if strings.HasSuffix(strings.ToLower(r), ".pmd") {
			refs = append(refs, r)
		}
	}
	return unique(refs)
}

func legacyVehicleKey(storage, root string, u sii.Unit, idx int) string {
	if strings.TrimSpace(u.Name) != "" {
		return strings.ToLower("legacy|" + storage + "|" + u.Name)
	}
	return strings.ToLower(fmt.Sprintf("legacy|%s|%s|%d", storage, root, idx))
}

func (e *engine) discoverCandidates(vehicles []Vehicle, storages []string) []Candidate {
	roots := map[string]bool{}
	for _, v := range vehicles {
		roots[strings.ToLower(v.RootDefinition)] = true
		for _, c := range v.Chassis {
			roots[strings.ToLower(c)] = true
		}
	}
	storageIncluded := map[string]bool{}
	for _, s := range storages {
		docs, _ := e.expandIncludes(s, map[string]bool{})
		for _, d := range docs {
			storageIncluded[strings.ToLower(d.Path)] = true
		}
	}
	out := []Candidate{}
	seen := map[string]bool{}
	for _, fi := range e.logical {
		p := graph.Normalize(fi.Path)
		lp := strings.ToLower(p)
		if !strings.HasPrefix(lp, "/def/vehicle/ai/") || !(strings.HasSuffix(lp, ".sii") || strings.HasSuffix(lp, ".sui")) {
			continue
		}
		if roots[lp] {
			continue
		}
		doc, err := e.parse(p)
		if err != nil {
			continue
		}
		for _, u := range doc.Units {
			models := explicitPMDRefs(u, p)
			if len(models) == 0 {
				continue
			}
			key := strings.ToLower(p + "|" + u.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
			conf, basis := 0.55, "model-linked vehicle definition"
			if storageIncluded[lp] {
				conf, basis = 0.82, "storage-included model-linked vehicle definition"
			}
			out = append(out, Candidate{ID: u.Name, DisplayName: prettyID(u.Name), Path: p, UnitType: u.Type, Basis: basis, Confidence: conf, ModelRefs: unique(models)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			return out[i].Path < out[j].Path
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
}

func (e *engine) bridgePackageCandidates(vehicles []Vehicle, candidates []Candidate) int {
	bySuffix := map[string][]Candidate{}
	byDir := map[string][]Candidate{}
	for _, c := range candidates {
		if len(c.ModelRefs) == 0 {
			continue
		}
		valid := []string{}
		for _, m := range c.ModelRefs {
			m = graph.Normalize(m)
			if e.exists(m) {
				valid = append(valid, m)
			}
		}
		if len(valid) == 0 {
			continue
		}
		c.ModelRefs = valid
		cs := strings.ToLower(strings.TrimSpace(c.ID))
		if i := strings.IndexByte(cs, '.'); i >= 0 && i+1 < len(cs) {
			cs = cs[i+1:]
		}
		if cs != "" {
			bySuffix[cs] = append(bySuffix[cs], c)
		}
		dir := strings.ToLower(path.Base(path.Dir(graph.Normalize(c.Path))))
		if dir != "" && dir != "." && dir != "/" {
			byDir[dir] = append(byDir[dir], c)
		}
	}
	bridged := 0
	for i := range vehicles {
		v := &vehicles[i]
		if len(v.Models) > 0 {
			continue
		}
		vs := strings.ToLower(strings.TrimSpace(v.ID))
		if j := strings.IndexByte(vs, '.'); j >= 0 && j+1 < len(vs) {
			vs = vs[j+1:]
		}
		matches := bySuffix[vs]
		basis := "exact traffic/chassis unit suffix"
		if len(matches) != 1 {
			stem := strings.ToLower(strings.TrimSuffix(path.Base(v.RootDefinition), path.Ext(v.RootDefinition)))
			for _, suf := range []string{"_jazzycat", "_jazzy", "_traffic", "_ai"} {
				stem = strings.TrimSuffix(stem, suf)
			}
			matches = byDir[stem]
			basis = "unique package-local chassis directory"
		}
		if len(matches) != 1 {
			continue
		}
		c := matches[0]
		models := uniqueOrdered(c.ModelRefs)
		if len(models) == 0 {
			continue
		}
		v.Chassis = unique(append(v.Chassis, c.Path))
		v.Models = models
		v.ModelAssets = nil
		for mi, m := range models {
			role := "lod"
			if mi == 0 {
				role = "main"
			}
			v.ModelAssets = append(v.ModelAssets, ModelAsset{Path: m, Role: role, SourceDefinition: c.Path})
		}
		// Merge the exact candidate definition and PMDs into the dependency graph.
		seenNode := map[string]bool{}
		for _, n := range v.Dependencies {
			seenNode[strings.ToLower(n.Path)] = true
		}
		addNode := func(p string) {
			p = graph.Normalize(p)
			k := strings.ToLower(p)
			if !seenNode[k] {
				v.Dependencies = append(v.Dependencies, graph.Node{Path: p, Type: graph.TypeForPath(p), Exists: e.exists(p)})
				seenNode[k] = true
			}
		}
		addNode(c.Path)
		v.Edges = append(v.Edges, graph.Edge{From: v.RootDefinition, To: c.Path, Reason: "package_chassis_bridge", Basis: basis})
		for _, m := range models {
			addNode(m)
			v.Edges = append(v.Edges, graph.Edge{From: c.Path, To: m, Reason: "model", Basis: "explicit accessory_chassis_data field"})
		}
		v.Counts.Models = len(models)
		v.Counts.Dependencies = len(v.Dependencies)
		v.Warnings = removeWarningPrefix(v.Warnings, "No PMD body model reference")
		v.DiscoveryBasis += "; package-local chassis bridge"
		v.Readiness, v.Blocking, v.SharedRequirements = classifyReadiness(v.Bound, v.Models, v.Missing)
		bridged++
	}
	return bridged
}

func removeWarningPrefix(in []string, prefix string) []string {
	out := in[:0]
	for _, s := range in {
		if strings.HasPrefix(s, prefix) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func documentedChassisPath(root string) string {
	lr := strings.ToLower(root)
	if !(strings.HasPrefix(lr, "/def/vehicle/ai/") && (strings.HasSuffix(lr, ".sii") || strings.HasSuffix(lr, ".sui"))) {
		return ""
	}
	base := strings.TrimSuffix(root, path.Ext(root))
	return base + "/chassis.sii"
}
func reasonForField(k string) string {
	s := strings.ToLower(k)
	switch {
	case strings.Contains(s, "model") || strings.Contains(s, "lod"):
		return "model"
	case strings.Contains(s, "collision"):
		return "collision"
	case strings.Contains(s, "material") || strings.Contains(s, "texture"):
		return "material_or_texture"
	case strings.Contains(s, "accessor") || strings.Contains(s, "data_path"):
		return "accessory"
	default:
		return "reference"
	}
}
func firstUseful(u sii.Unit, keys ...string) string {
	for _, k := range keys {
		if v := sii.First(u, k); v != "" && !strings.HasPrefix(v, "@@") {
			return strings.Trim(v, "\"'")
		}
	}
	return ""
}
func prettyID(s string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"traffic.", "chassis.", "."} {
		s = strings.TrimPrefix(s, prefix)
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '_' || r == '-' })
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	if len(parts) == 0 {
		return s
	}
	return strings.Join(parts, " ")
}
func unique(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = graph.Normalize(s)
		if s == "/" || s == "" {
			continue
		}
		k := strings.ToLower(s)
		if !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
func uniqueText(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = strings.Trim(strings.TrimSpace(s), "\"'")
		if s == "" {
			continue
		}
		k := strings.ToLower(s)
		if !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
