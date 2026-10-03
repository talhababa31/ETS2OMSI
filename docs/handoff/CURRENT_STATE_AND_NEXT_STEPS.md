# ETS2OMSI V2.2 — Current State / Next Steps

## Current state

- Scanner: working on modern + legacy/Jazzycat-style traffic packages.
- V2 UI: cars-only, browser-based local app.
- Extraction: selected vehicle source extraction exists.
- Model decoder: ConverterPIX is auto-installed/downloaded when needed.
- Geometry: PMD/PMG -> PIM -> internal scene -> O3D works.
- Texture/material pipeline: package-local PIT/material hints with safe fallback behavior.
- Golden coordinate/output rules: implemented and covered by tests.
- Wheels: separate wheel visuals/config supported when resolved.
- LODs: ETS2-provided LOD model roles are preserved.
- Physics: automatic OMSI AI profile estimate from name + dimensions.
- Validation: O3D readback, orientation, ground, wheel and texture reference checks exist.

## Highest-priority next step

**Do not add features yet. Test V2.2 in OMSI 2 with several real cars.**

Recommended validation set:
- Alfa 155: front/rear light material assignment
- one car that previously had odd wheel texture
- BMW F30-like sedan: physics/stance
- BMW X6/Q7-like large SUV: physics/stance
- one car with multiple package LODs

Record for each:
- source vehicle ID
- body texture result
- glass result
- front/rear light result
- wheel texture result
- wheel placement
- orientation
- ride height/ground contact
- OMSI loading/log errors

## If front/rear lights are still swapped

Inspect in this order:
1. PIM triangle material index -> material slot identity.
2. PIT material alias -> visible texture hint.
3. `applyMaterials` / `resolveTextureHints` / `lookupTexture` in `internal/convert/pipeline.go`.
4. Confirm front/rear geometry is not being merged/reindexed incorrectly.

Do not fix it by swapping filenames globally; the mapping must remain per material slot.

## If wheel/jant texture is still wrong

Inspect:
1. wheel PMD/PIT is resolved independently from body PMD/PIT;
2. wheel material slot indices remain sparse-safe;
3. wheel texture hint does not inherit a body PIT hint;
4. only visible diffuse/base texture tags participate.

## If vehicle still looks too low/compressed

Separate visual ground/origin from physics:
- visual stance: inspect `groundPlane`, wheel radius/placement, `scene.Translate`, and generated model.cfg/OVH height-related values;
- physics softness/weight: inspect `EstimateAIPhysics` and suspension/force values in `internal/omsi/config.go`.

Do not try to fix visual ride height only by increasing mass.

## Documentation inconsistency to clean later

`README_TR.md` contains older V2.1 wording that unresolved visible textures always fail conversion. V2.1.2 changed normal GUI conversion to safe fallback (`StrictFidelity=false`). Update README wording when doing the next documentation cleanup.
