# ETS2OMSI V2.1 GOLDEN — Technical foundation

## Product boundary

V2 converts **AI traffic cars already contained in one selected `.scs` package**. It does not resolve an ETS2 installation, DLC archives, player trucks, buses or interiors.

`traffic.scs -> storage-bound car -> package-local chassis/model set -> PMD/PMG -> OMSI AI vehicle`

## Archive layer

- ZIP `.scs`: native random-access reader.
- HashFS `.scs`: external archive-helper adapter.
- Extracted directory: supported for tests/debugging.

## Vehicle discovery

1. Discover all `traffic_storage*.sii` variants.
2. Traverse explicit `@include` chains.
3. Parse modern and legacy traffic vehicle units.
4. Keep only storage-bound `car` roots in the V2 app.
5. Follow explicit same-document and global unit-ID references.
6. Follow documented `<vehicle>/chassis.sii` when it exists.
7. If an old traffic root still has no model, use the V2 package-local bridge only when deterministic:
   - exact `traffic.foo` -> `chassis.foo` suffix, or
   - a unique chassis directory matching the root definition stem.
8. Preserve explicit PMD order as main + LOD set.

## Conversion boundary

ConverterPIX is used only as a binary Prism3D interoperability decoder. V2 mounts the selected SCS package itself as the model source. Generated PIM/PIT data is parsed into ETS2OMSI's own internal scene, then written as OMSI O3D.

## OMSI package

The generated package is deliberately minimal:

- exterior body O3D
- ETS2-provided LOD O3Ds when available
- textures/materials that can be resolved from the package/model conversion
- `model.cfg`
- `.ovh`
- minimal AI constfile
- validation/conversion report

ETS2 light/shadow helpers are not treated as a reason to require the full game install.

## V2.1 Golden output contract

The package scanner remains SCS-only. The output contract was tightened against a known-working OMSI AI vehicle structure:

- internal scene axes: X lateral, Y longitudinal, Z vertical;
- O3D axes: X lateral, Y vertical, Z longitudinal;
- visible textures must resolve deterministically from package-local PIT/MAT/TOBJ/image relations;
- wheel contact is preferred for ground calibration;
- FL/FR/RL/RR wheel geometry is exported separately when wheel models are resolved;
- shared OMSI AI scripts/sounds use two-level relative paths from `Vehicles/<vehicle>`;
- written O3D files are parsed again and validated before stage commit.

The converter still does not mount an ETS2 installation, base archives or DLC packages. Optional ETS2 runtime helpers may be ignored, but unresolved visible body textures are treated as a conversion error instead of being hidden by a checker texture.
