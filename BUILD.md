# ETS2OMSI V2.6.0 — Derleme

Gereken: Go 1.23+ (Windows exe'leri Linux/macOS'ta da çapraz derlenebilir).

```powershell
go vet ./...
go test ./...
go test -race ./...
$env:GOOS="windows"; $env:GOARCH="amd64"
go build -trimpath -ldflags "-s -w -H windowsgui" -o release/ETS2OMSI.exe ./cmd/ets2omsi-alpha
go build -trimpath -ldflags "-s -w" -o release/ETS2OMSI_CLI.exe ./cmd/ets2omsi-cli
```

Program seçenekleri: `--no-window` (pencere yerine tarayıcı), `--no-browser` (hiçbir şey açma, adresi yazdır).

Son kullanıcının Go, Blender veya ETS2'ye ihtiyacı yoktur. ConverterPIX yoksa ilk 3D işleminde otomatik indirilir.
