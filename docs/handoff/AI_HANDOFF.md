# ETS2OMSI V2.2 — AI Development Handoff

## What this project is

ETS2OMSI converts **basic AI traffic cars contained inside one ETS2 `.scs` package** into **minimal OMSI 2 AI vehicle packages**.

Current active version: **V2.2 MATERIAL + PHYSICS**.

The project was intentionally narrowed after an over-complicated 1.0 experiment. The current direction is very important:

> Do not convert ETS2 as a game. Decode the low/medium-poly traffic car asset already inside the selected SCS, preserve its exterior appearance, and write a working OMSI AI car.

## Non-negotiable scope

IN:
- one selected traffic `.scs`
- storage-bound AI cars
- exterior body geometry
- wheel geometry
- package-local materials/textures
- package-local ETS2 LODs
- minimal OMSI O3D/model.cfg/OVH/AI config
- browser-based local UI

OUT unless the user explicitly asks later:
- ETS2 installation dependency
- base.scs / DLC mount requirement
- player trucks
- buses
- interiors/dashboard
- gameplay/engine/transmission conversion
- Blender as a required manual step
- manual texture-region light editor
- AI assistant/provider integrations

## Current working architecture

1. Open selected `.scs` as ZIP/HashFS/directory through `internal/archive`.
2. Parse SII/SUI definitions through `internal/sii`.
3. Find `traffic_storage*.sii`, including old Jazzycat naming styles.
4. Resolve modern + legacy traffic vehicle roots and package-local chassis/model sets.
5. V2 app filters the result to storage-bound `car` vehicles.
6. For a selected car, ConverterPIX decodes binary PMD/PMG into PIM/PIT.
7. `internal/pim` parses geometry into `scene.Scene`.
8. `internal/pit` supplies material/texture hints.
9. Conversion composes wheels, calibrates ground/orientation, resolves/falls back materials, estimates AI physics and writes O3D/config files.
10. O3D files are read back and validated before the result is reported.

## The user's real validation package

The project has been exercised against `ai_traffic_pack_by_Jazzycat_v2.9.scs`.
The scanner report demonstrated that the package can contain many traffic cars and that old/modern definitions coexist.
`docs/JAZZYCAT_REPORT_VALIDATION.md` records the deterministic bridge strategy used for old roots.

## Golden OMSI reference

A known-working `vwpassat98` OMSI AI vehicle was inspected as a behavior/reference package. Its assets are **not distributed with ETS2OMSI**.

Rules learned from that reference and already encoded in tests:
- O3D uses X width / Y height / Z length for the vehicle geometry.
- Internal scene remains X lateral / Y longitudinal / Z vertical.
- FL/FR/RL/RR wheels can be separate O3D meshes.
- Shared AI script/sound references are two levels up from `Vehicles/<vehicle>`.
- Ground should be based on wheel contact when possible, not the lowest bumper vertex.

## Important current behavior

### Material fallback
The active GUI API calls conversion with `StrictFidelity=false`.
Therefore the current V2.2 behavior is:
- use exact package-local texture/material relations when available;
- if a visible semantic material cannot be resolved, safe fallback materials may be generated so a structurally valid car can still export;
- structural failures (model decode, O3D write/readback etc.) remain fatal.

**Note:** `README_TR.md` still contains some older V2.1 wording saying unresolved visible textures always stop conversion. The V2.1.2 export hotfix changed that behavior. Treat current code (`cmd/ets2omsi-alpha/main.go` + `internal/convert/pipeline.go`) as source of truth.

### Material-slot bug already fixed
Do not undo this:
- PIM material indices can be sparse.
- A triangle that says material 4 must still bind to material slot 4.
- Compressing slots caused front/rear light textures and wheel textures to swap.

### Texture-channel bug already fixed
Do not use normal/mask/specular/reflection maps as the visible diffuse texture.
Use only visible base/diffuse/albedo/color relations.

### Physics
`EstimateAIPhysics` produces an OMSI traffic-physics estimate from:
- vehicle display/name hints (e.g. sedan vs large SUV classes), plus
- measured 3D dimensions.

The estimated mass is an OMSI AI physics target, **not a claim about exact manufacturer curb weight**.

## What to test first after any code change

1. `go test ./...`
2. `go test -race ./...`
3. `go vet ./...`
4. Build Windows GUI + CLI.
5. Scan a real multi-car SCS.
6. Convert one known car.
7. Confirm in OMSI:
   - orientation is correct;
   - body is not rotated/standing vertically;
   - ground/ride height looks plausible;
   - body texture is correct;
   - front/rear lights are not swapped;
   - wheel/jant texture is correct;
   - wheels are positioned correctly.

## Current likely next work

The latest reported issues before V2.2 were:
- front/rear light textures swapped on some cars;
- odd wheel/jant texture assignment;
- cars looked too compressed/soft in physical behavior.

V2.2 specifically changed sparse material-slot handling, diffuse texture selection, wheel material handling and physics profiles to address those issues. **The next step should be real OMSI in-game validation of V2.2 before adding new features.**

If an issue remains, do not add unrelated systems. Trace the specific conversion stage and fix the smallest root cause.

## Preferred development philosophy

- Small fixes, real regression tests.
- No speculative parser behavior when dependency evidence exists.
- Do not re-expand product scope.
- Prefer source package data over guessed metadata.
- Keep conversion usable even when nonessential ETS2 helper materials are absent.
- Preserve external appearance first; advanced effects second.
