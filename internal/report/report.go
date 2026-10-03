package report

import (
	"encoding/json"
	"ets2omsi/internal/scanner"
	"fmt"
	"html/template"
	"io"
	"sort"
)

func WriteJSON(w io.Writer, r scanner.PackageReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

var page = template.Must(template.New("r").Funcs(template.FuncMap{"pct": func(v float64) float64 { return v * 100 }}).Parse(`<!doctype html><html><head><meta charset="utf-8"><title>ETS2OMSI scan report</title><style>body{font:14px system-ui;background:#f5f5f7;color:#1d1d1f;margin:40px}.card{background:white;border-radius:18px;padding:20px;margin:14px 0;box-shadow:0 8px 30px #00000010}.ok{color:#178a43}.warn{color:#b26a00}.bad{color:#c73333}code{background:#f0f0f2;padding:2px 5px;border-radius:6px}.grid{display:grid;grid-template-columns:repeat(5,1fr);gap:12px}.muted{color:#6e6e73}.diag{display:grid;grid-template-columns:repeat(5,1fr);gap:8px}.diag div{background:#f5f5f7;border-radius:12px;padding:10px}.candidate{border-top:1px solid #eee;padding:10px 0}</style></head><body><h1>ETS2 → OMSI 2 Package Scan</h1><p class="muted">{{.Package}} · {{.Version}} · {{.Kind}}</p><div class="grid"><div class="card"><b>{{.Stats.VehicleCount}}</b><br>Vehicles</div><div class="card"><b>{{.Stats.ReadyCount}}</b><br>Ready roots</div><div class="card"><b>{{len .SharedAssets}}</b><br>Shared assets</div><div class="card"><b>{{.Stats.WarningCount}}</b><br>Warnings</div><div class="card"><b>{{.Stats.CandidateCount}}</b><br>Recovery candidates</div></div>
<div class="card"><h2>Scanner diagnostics</h2><p class="muted">Logical root: {{if .Diagnostics.LogicalRootPrefix}}<code>{{.Diagnostics.LogicalRootPrefix}}</code>{{else}}archive root{{end}}</p><div class="diag"><div><b>{{.Diagnostics.TrafficStorageFiles}}</b><br>traffic storage</div><div><b>{{.Diagnostics.StorageDirectIncludes}}</b><br>storage includes</div><div><b>{{.Diagnostics.VehicleDefinitionFiles}}</b><br>vehicle defs</div><div><b>{{.Diagnostics.TrafficVehicleUnits}}</b><br>traffic_vehicle</div><div><b>{{.Diagnostics.LegacyModelLinkedIncludes}}</b><br>legacy model links</div><div><b>{{.Diagnostics.LegacyStorageRoots}}</b><br>legacy roots</div><div><b>{{.Diagnostics.ChassisUnits}}</b><br>chassis units</div><div><b>{{.Diagnostics.IncludeDirectives}}</b><br>includes</div><div><b>{{.Diagnostics.PMDFiles}}</b><br>PMD</div><div><b>{{.Diagnostics.PMGFiles}}</b><br>PMG</div><div><b>{{.Diagnostics.MATFiles}}</b><br>MAT</div><div><b>{{.Diagnostics.TOBJFiles}}</b><br>TOBJ</div><div><b>{{.Diagnostics.ParseErrors}}</b><br>parse errors</div></div>{{if .Warnings}}<p class="warn">{{range .Warnings}}{{.}}<br>{{end}}</p>{{end}}</div>
{{range .Vehicles}}<div class="card"><h2>{{.DisplayName}}</h2><p><code>{{.VehicleType}}</code> · {{if .Bound}}<span class="ok">storage-bound</span>{{else}}<span class="warn">unbound</span>{{end}}</p><p class="muted">{{.RootDefinition}}</p><p class="muted">{{.DiscoveryBasis}}</p><p>{{.Counts.Models}} models · {{.Counts.Textures}} texture refs · {{.Counts.Dependencies}} dependencies · {{.Counts.Variants}} variants</p>{{if .Missing}}<p class="warn">Missing: {{range .Missing}}<br><code>{{.}}</code>{{end}}</p>{{end}}{{if .Warnings}}<p class="warn">{{range .Warnings}}{{.}}<br>{{end}}</p>{{end}}</div>{{end}}
{{if .Candidates}}<div class="card"><h2>Recovery candidates</h2><p class="muted">Diagnostic-only model-linked chassis definitions. These are not silently promoted to finalized vehicles.</p>{{range .Candidates}}<div class="candidate"><b>{{.DisplayName}}</b> · {{.UnitType}} · {{printf "%.0f" (pct .Confidence)}}%<br><code>{{.Path}}</code>{{range .ModelRefs}}<br><span class="muted">PMD: {{.}}</span>{{end}}</div>{{end}}</div>{{end}}</body></html>`))

func WriteHTML(w io.Writer, r scanner.PackageReport) error { return page.Execute(w, r) }

func Summary(r scanner.PackageReport) string {
	types := map[string]int{}
	for _, v := range r.Vehicles {
		types[v.VehicleType]++
	}
	keys := make([]string, 0, len(types))
	for k := range types {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s := fmt.Sprintf("%d vehicle(s), %d ready, %d warning(s), %d recovery candidate(s)", r.Stats.VehicleCount, r.Stats.ReadyCount, r.Stats.WarningCount, r.Stats.CandidateCount)
	for _, k := range keys {
		s += fmt.Sprintf(" | %s:%d", k, types[k])
	}
	return s
}
