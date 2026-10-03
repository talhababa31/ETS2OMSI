# ETS2OMSI V2.2 — Project Structure

## Active product boundary

**Input:** one ETS2 traffic `.scs` package  
**Output:** minimal OMSI 2 AI car packages (`.ovh`, O3D meshes, model.cfg, textures, minimal AI config)

Do not require ETS2 itself. Do not add player-truck, bus or interior conversion unless the user explicitly changes scope.

## Source tree

```text
ETS2OMSI_V2.2-MATERIAL-PHYSICS_Source/
├─ go.mod                         # module ets2omsi, Go 1.23
├─ START_HERE.txt                 # shortest end-user flow
├─ README.md / README_TR.md       # product behavior and user instructions
├─ CHANGELOG.md                   # current release notes
├─ BUILD.md                       # test/build commands
├─ TEST_REPORT.md                 # validation status
├─ MATERIAL_PHYSICS_NOTES_TR.md   # V2.2 material/physics changes
├─ GOLDEN_REFERENCE_NOTES_TR.md   # Passat-derived OMSI behavior rules
├─ EXPORT_HOTFIX_NOTES_TR.md      # V2.1.2 fallback/export logic
├─ GLASS_HOTFIX_NOTES_TR.md       # V2.1.1 glass behavior
├─ THIRD_PARTY_NOTICES.md
│
├─ cmd/
│  ├─ ets2omsi-alpha/
│  │  ├─ main.go                  # local HTTP app, API endpoints, launches browser UI
│  │  ├─ filepicker_windows.go    # native Windows SCS/folder picker
│  │  └─ filepicker_other.go
│  └─ ets2omsi-cli/
│     └─ main.go                  # CLI entry point
│
├─ internal/
│  ├─ app/
│  │  ├─ project.go               # V2 cars-only project boundary; selected SCS is the only mount
│  │  └─ scan.go                  # scan orchestration
│  │
│  ├─ archive/
│  │  ├─ archive.go               # Source abstraction/opening
│  │  ├─ zip.go                   # native ZIP .scs reader
│  │  ├─ hashfs.go                # HashFS helper adapter
│  │  ├─ dir.go                   # extracted-directory source
│  │  └─ multi.go                 # layered source implementation (mostly infrastructure)
│  │
│  ├─ sii/
│  │  └─ parser.go                # SII/SUI parser: units, includes, arrays, comments
│  │
│  ├─ scanner/
│  │  ├─ scanner.go               # traffic storage discovery, legacy bridge, dependency graph
│  │  └─ types.go                 # Vehicle, PackageReport, Diagnostics, readiness data
│  │
│  ├─ graph/
│  │  └─ graph.go                 # normalized dependency nodes/edges
│  │
│  ├─ extract/
│  │  └─ extract.go               # "Seçileni Ayır" / package-local source extraction
│  │
│  ├─ pixbridge/
│  │  └─ bridge.go                # ConverterPIX discovery/download/cached conversion
│  │
│  ├─ pixtext/
│  │  └─ parser.go                # low-level PIX text parsing helpers
│  │
│  ├─ pim/
│  │  └─ pim.go                   # PIM geometry -> internal scene; material slot handling
│  │
│  ├─ pit/
│  │  └─ pit.go                   # material/texture hints; diffuse/base selection rules
│  │
│  ├─ scene/
│  │  └─ scene.go                 # internal geometry representation, bounds, transforms, merging
│  │
│  ├─ convert/
│  │  ├─ pipeline.go              # MAIN conversion pipeline and material/physics/wheel logic
│  │  └─ preview.go               # source/final preview generation
│  │
│  ├─ o3d/
│  │  └─ o3d.go                   # OMSI O3D writer/reader/readback validation
│  │
│  ├─ omsi/
│  │  └─ config.go                # `.ovh`, `model.cfg`, minimal AI constfile, physics formatting
│  │
│  └─ report/
│     └─ report.go                # JSON/HTML/report output
│
├─ docs/
│  ├─ TECHNICAL_FOUNDATION.md     # V2 architecture contract
│  └─ JAZZYCAT_REPORT_VALIDATION.md
│
├─ examples/
│  ├─ demo_multi_vehicle.scs
│  ├─ demo_jazzycat_legacy_style.scs
│  └─ demo_jazzycat_pre119_legacy.scs
│
└─ tools/
   ├─ setup_converter_pix.ps1     # helper installer for ConverterPIX
   └─ setup_scs_packer.ps1        # HashFS helper setup
```

## Data flow

```text
Selected traffic .SCS
  -> archive.Source
  -> scanner.Scan
  -> storage-bound car Vehicle objects
  -> optional extract.Vehicle
  -> convert.Vehicle
  -> ConverterPIX (PMD/PMG -> PIM/PIT)
  -> pim.Parse + pit.Parse
  -> scene.Scene
  -> wheel/material/texture/ground/physics processing
  -> o3d.Write
  -> omsi.ModelCFG + omsi.OVH + AI constfile
  -> O3D readback + validation
  -> output/<vehicle>/
```

## Key code entry points

- `internal/app/project.go::ScanProject`
  - Enforces V2 scope: only **bound `car`** vehicles remain visible.
  - `MountPaths` contains only the selected SCS.

- `internal/scanner/scanner.go::Scan`
  - Finds every `traffic_storage*.sii` variation.
  - Expands includes.
  - Handles modern and legacy traffic structures.
  - Bridges old traffic roots to package-local chassis/model sets only when deterministic.

- `internal/convert/pipeline.go::Vehicle`
  - Main end-to-end conversion.
  - Current API calls it with `StrictFidelity: false`.
  - Exact textures are preferred, but safe semantic fallbacks are allowed in normal V2 mode.

- `internal/pim/pim.go`
  - Preserve sparse material slot indices. **Do not compress material IDs.**

- `internal/pit/pit.go`
  - Only visible base/diffuse/albedo/color texture tags may become diffuse output.
  - Do not promote normal/mask/specular/reflection textures to visible diffuse.

- `internal/o3d/o3d.go`
  - Internal axes -> O3D axes conversion and readback validation.

- `internal/omsi/config.go`
  - Generates working OMSI AI vehicle config structure.
  - Keep common AI paths at `..\\..\\Scripts\\AI_Cars` and `..\\..\\Sounds\\AI_Cars`.

## Coordinate contract — DO NOT REGRESS

Internal scene:
- X = lateral / left-right
- Y = longitudinal / front-back
- Z = vertical / height

OMSI O3D:
- X = lateral / width
- Y = vertical / height
- Z = longitudinal / length

Normals must use the same basis conversion. Triangle winding must stay correct after basis conversion.

## V2 product rules — DO NOT REGRESS

1. ETS2 installation is not required.
2. Only the selected `.scs` package is the active source for conversion.
3. Default UI shows storage-bound AI **cars** only.
4. Do not bring back trucks, buses, player interiors or DLC/base mounting into the default flow.
5. Optional ETS2 runtime helpers must not block a basic OMSI exterior conversion.
6. A structural model decode/O3D failure is fatal; harmless material-helper gaps are not.
7. Never hide front/rear/wheel material index errors by compressing sparse slots.
8. Prefer the ETS2-provided LOD models instead of generating destructive low-poly geometry.
9. Keep the browser UI simple: Select SCS -> Find Cars -> Extract/Preview -> Convert.
