# ETS2OMSI V2.2 MATERIAL + PHYSICS — Test report

Validation completed in the build environment:

- `go test ./...` — PASS
- `go test -race ./...` — PASS
- `go vet ./...` — PASS
- frontend JavaScript syntax check — PASS
- sparse PIM material-slot regression — PASS
- diffuse-only PIT texture selection regression — PASS
- F30/X6/dimension-based OMSI AI physics profile regression — PASS
- OVH formatting/physics emission regression — PASS

Real OMSI 2 rendering must still be validated on Windows with the user's real traffic SCS package.
