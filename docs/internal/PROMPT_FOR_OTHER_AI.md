# Paste this into another AI together with the current source ZIP

You are continuing development of **ETS2OMSI V2.2 MATERIAL + PHYSICS**.

First read these files before changing code:
1. `AI_HANDOFF.md`
2. `PROJECT_STRUCTURE.md`
3. `FULL_CHANGELOG.md`
4. `CURRENT_STATE_AND_NEXT_STEPS.md`
5. the project's `README_TR.md`, `docs/TECHNICAL_FOUNDATION.md`, and tests

Hard product boundary:
- Input is ONE traffic `.scs` package.
- Convert only storage-bound ETS2 AI **cars**.
- Output minimal OMSI 2 AI car packages.
- Do NOT require ETS2 installation, base/DLC mounts, Blender, player trucks, buses, interiors, gameplay systems or AI-provider features.

Current data flow:
`.scs -> scanner -> car -> PMD/PMG -> ConverterPIX -> PIM/PIT -> internal scene -> OMSI O3D/model.cfg/OVH`

Critical invariants:
- internal axes = X lateral, Y longitudinal, Z vertical
- O3D axes = X lateral, Y vertical, Z longitudinal
- preserve sparse PIM material slot indices
- never promote normal/mask/specular/reflection maps to visible diffuse
- common OMSI AI paths are `..\\..\\Scripts\\AI_Cars` and `..\\..\\Sounds\\AI_Cars`
- use package-local assets only in the default V2 flow
- optional ETS2 helper materials must not fail basic exterior conversion
- current GUI conversion runs with `StrictFidelity=false`

The known-working Passat reference was used only to derive OMSI behavior. Do not copy/distribute its assets.

Before implementing anything new, run the existing tests and inspect the current code. The latest fix set (V2.2) targeted front/rear light texture swaps, wheel/jant texture mismatches, and vehicle physics profiling. Validate those fixes in OMSI before expanding scope.
