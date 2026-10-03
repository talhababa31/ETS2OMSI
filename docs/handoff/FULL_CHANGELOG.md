# ETS2OMSI — Full Project Changelog

This document is a continuation/history handoff for the ETS2OMSI project. The current active branch is **V2.2 MATERIAL + PHYSICS**.

## 0.0.1 ALPHA — Foundation Scanner
- Created a new ETS2 → OMSI project instead of reusing the GTA importer directly.
- Added `.scs` package scanning infrastructure.
- Added ZIP SCS virtual scanning without destructive extraction.
- Added HashFS helper adapter path.
- Added recursive SII/include parsing foundations.
- Added traffic-storage based AI vehicle detection.
- Added per-vehicle dependency graph model.
- Added shared dependency analysis.
- Added JSON/HTML scan reports.
- Added the first browser-based local UI and CLI.
- Conversion intentionally remained disabled; the goal was scanner correctness first.

## 0.0.1a SCANNER RECOVERY
- Expanded `traffic_storage*.sii` discovery to support dotted/infix legacy names such as `traffic_storage.jazzycat.sii` and `traffic_storage_car.<mod>.sii`.
- Added nested/root path normalization for unusual ZIP layouts.
- Hardened SII parsing for comments, includes and indexed array fields.
- Added a Recovery Inspector with counts for SII/SUI, PMD/PMG, MAT/TOBJ, includes, storage files and candidates.
- Added fallback discovery diagnostics instead of showing an empty vehicle list with no explanation.

## 0.0.1b LEGACY BRIDGE
- Added legacy Jazzycat/pre-1.19 style storage support.
- Added direct storage-include traversal for old traffic definitions.
- Added evidence-based legacy vehicle promotion rules.
- Avoided guessing PMD paths solely from filenames.
- Added legacy model-link/root diagnostics and tests.

## 0.0.1c SII PARSER FIX
- Fixed SII unit declarations where the opening `{` is on the next line.
- This unlocked real `traffic_vehicle` / `accessory_chassis_data` parsing in the Jazzycat test package.
- Real package scan started resolving hundreds of vehicle definitions and model/chassis links correctly.

## 1.0 — Broad Conversion Experiment (SUPERSEDED)
- Added a larger conversion pipeline, installation/base/DLC ideas, ConverterPIX bridge, O3D generation, validators and batch concepts.
- This branch became too broad for the user's real target.
- **Do not restore its ETS2-install/base/DLC requirement into V2.**

## V2 — SCS-ONLY Scope Reset
The product goal was deliberately narrowed to:

`one traffic .SCS -> storage-bound AI cars -> exterior 3D/materials/textures -> OMSI .ovh AI cars`

Removed from the active product scope:
- ETS2 installation requirement
- base/DLC mounting requirement
- player trucks
- buses
- interiors/dashboard/gameplay systems
- AI advisor integration
- manual texture-light editor

Added/retained:
- Cars-only vehicle list.
- Package-local vehicle/model bridge.
- `Extract Vehicle` workflow.
- ConverterPIX as an internal binary model decoder.
- PMD/PMG -> PIM/PIT -> internal scene -> O3D pipeline.
- Minimal OMSI `.ovh`, `model.cfg`, textures and AI config generation.
- User-facing conversion stage/error reporting.

## V2.1 GOLDEN
- Used a known-working OMSI 2 Volkswagen Passat AI vehicle as a behavioral reference only.
- Corrected coordinate mapping:
  - internal scene: X lateral, Y longitudinal, Z vertical
  - OMSI O3D: X lateral, Y vertical, Z longitudinal
- Applied the same axis transform to normals and validated triangle winding.
- Added wheel-contact based ground calibration.
- Exported FL/FR/RL/RR wheel geometry as separate O3D files where possible.
- Added wheel rotation/suspension/steering blocks to `model.cfg`.
- Corrected common AI script/sound relative paths to `..\\..\\Scripts\\AI_Cars` and `..\\..\\Sounds\\AI_Cars`.
- Tightened texture resolution toward deterministic PIT/MAT/TOBJ/image relations.
- Added O3D readback/orientation/ground/wheel validation.

## V2.1.1 GLASS HOTFIX
- Fixed textureless legacy `glass_ex` materials being treated as fatal missing visible textures.
- Textureless glass can now become a neutral translucent OMSI glass material.
- Explicit referenced glass textures still remain meaningful dependencies.

## V2.1.2 EXPORT HOTFIX
- Fixed a real logic bug where unresolved visible materials could still force `FAILED` even though the V2 UI was using `StrictFidelity=false`.
- In normal SCS-only mode, successful 3D decode is prioritized:
  - exact textures are used when resolved;
  - unresolved glass/body/rubber/light/helper materials can receive safe semantic OMSI fallbacks;
  - optional ETS2 helper materials no longer block conversion.
- Structural failures such as model decode or O3D write/readback remain fatal.

## V2.2 MATERIAL + PHYSICS — CURRENT
- Fixed sparse PIM material indices so triangle material slots are never compressed/shifted.
  - Important for front/rear light materials and wheel materials.
- Prevented normal/mask/specular/reflection textures from being promoted to visible diffuse textures.
- Wheel models now use the same exact slot-safe material pipeline.
- Added OMSI AI physics profile estimation from vehicle name + measured 3D dimensions.
- Added estimated physics mass/profile to conversion results.
- Suppressed harmless optional ETS2 helper-material messages from user-facing warnings.
- Kept the V2 product boundary: **SCS-only / cars-only / exterior-only**.

## Current validation status
Build-environment checks currently pass:
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- frontend JavaScript syntax check
- sparse material-slot regression
- diffuse-only PIT texture selection regression
- F30/X6/dimension-based physics regression
- OVH formatting/physics regression

Real OMSI 2 rendering still must be validated with the user's real SCS packages after every output-affecting change.
