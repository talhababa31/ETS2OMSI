# Build ETS2OMSI V2.2 MATERIAL + PHYSICS

Requirements for building ETS2OMSI itself: Go 1.23+.

```powershell
go test ./...
go vet ./...
go test -race ./...
$env:GOOS="windows"
$env:GOARCH="amd64"
go build -trimpath -ldflags "-s -w -H windowsgui" -o ETS2OMSI_V2_2_MATERIAL_PHYSICS.exe ./cmd/ets2omsi-alpha
go build -trimpath -ldflags "-s -w" -o ETS2OMSI_V2_2_MATERIAL_PHYSICS_CLI.exe ./cmd/ets2omsi-cli
```

End users do not need Go, Blender or ETS2. ConverterPIX is automatically downloaded at first 3D operation if it is not already present.
