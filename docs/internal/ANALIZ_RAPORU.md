# ETS2OMSI V2.2 — Analiz Raporu (3 sorun)

> **Durum (V2.2.2):** Bu rapordaki 3 düzeltmenin hepsi yapıldı (3D görüntüleyici, texture kurtarma, exe). Ayrıntı: [CHANGELOG.md](../../CHANGELOG.md).

Test paketi: `ai_traffic_pack_by_Jazzycat_v2.9.scs` (687 MB, 20.045 giriş, 171 araç)
Referans: gerçek OMSI 2 araçları (`Opel_Manta_B`, `Peugeot 106`) + Go 1.27 kaynak kodu.

---

## 1) "EXE çalışmıyor" — bulundu

`ETS2OMSI_V2_2_MATERIAL_PHYSICS.exe` aslında **çalışıyor ama kullanıcıya hiçbir şey göstermiyor.**

| Ölçüm | Sonuç |
|---|---|
| PE subsystem | `1` (GUI) |
| `MainWindowHandle` (tüm örnekler) | `0` → **pencere açmıyor** |
| Yaptığı | `127.0.0.1:<rastgele port>` üzerinde HTTP sunucusu başlatır, 180 ms sonra `rundll32 url.dll,FileProtocolHandler` ile tarayıcıyı açar |
| Süreç davranışı | tek-örnek (single-instance) koruması **yok**, çıkış (exit) yolu **yok** |
| Gözlenen | **10 adet yetim (orphan) süreç** çalışıyor (PID 4576, 9184, 11252, 12424, 9136, 15176, 12472, 6948, 4876, 8776) |
| Log | **göreli** yol: `ETS2OMSI_V2_2_MATERIAL_PHYSICS.log` (çalışma dizinine göre yer değiştirir) |
| Defender | temiz, tespit yok |

**Sonuç:** "exe çalışmıyor" hissi, pencere + durum + çıkış yolu olmamasından geliyor. Tarayıcı açılmazsa
kullanıcıya URL de gösterilmiyor → hiçbir geri bildirim yok. Sunucu arka planda çalışıyor ama
her çift tıklamada **yeni bir süreç** daha bırakılıyor.

**Düzeltme:** görünür pencere/durum satırı, URL'i ekranda gösterme, tek-örnek kilidi, mutlak log yolu,
çıkış butonu/yolu, `--no-browser` seçeneği.

---

## 2) "Texture hataları" — 3 ayrı kök neden (sayısal kanıtlı)

171 aracın dönüşüm sonucu: **0 fail / 0 pass / 171 warn**.
Uyarı dağılımı: 970 cam fallback, 49 gövde fallback, 21 tekerlek modeli uyarı, 15 zemin/tekerlek teması.

### 2a) ConverterPIX, dosya adında **boşluk** olan texture'ları sessizce atlıyor (ANA NEDEN)

Paket içinde **tam olarak 10** adet boşluklu `.dds` var ve **10'u da** ConverterPIX önbelleğinde
(`%LocalAppData%\ETS2OMSI\pix`) **hiç yok** → 10/10 kayıp.

```
vehicle/ai/jazzycat/renault_megane_2/tableau de bord.dds   → mat_0006_tableaudebord
vehicle/ai/jazzycat/renault_megane_2/porte ar.dds          → mat_0013_portear
vehicle/ai/jazzycat/renault_megane_2/megane phare.dds      → mat_0014_meganephare
vehicle/wheel_jazzy/renault_megane_2/jante 512.dds         → mat_0023_jante512
vehicle/ai/jazzycat/vw_lt/dorr tap.dds                     → mat_0009_dorrtap / mat_0011_dorrtap
vehicle/ai/jazzycat/vw_lt/dzrwi tap.dds                    → mat_0011_dzrwitap / mat_0013_dzrwitap
vehicle/ai/jazzycat/vw_lt/itd plast.dds                    → mat_0007_itdplast
vehicle/ai/jazzycat/vw_lt/kolor wheel.dds                  → mat_0015_kolorwheel
vehicle/ai/jazzycat/jaguar_x350/panel 2.dds                → mat_0011_panel2
vehicle/ai/jazzycat/renault_megane_2/megane phare_l.dds
```

`Ren Megan2` kanıtı (`conversion_report.json`):
`unresolved = [glass_ex ×3, tableaudebord ×2, portear, meganephare, jante512 ×2]` →
dosyaların hepsi SCS'te **mevcut**, ama boşluk yüzünden hiç çıkarılmamış.

### 2b) `.tobj`'siz `.dds`'ler hiç çıkarılmıyor (2. sınıf)

Paketin 1328 `.dds`'inden **25 tanesinin yanında `.tobj` yok**. ConverterPIX sadece `.tobj`
üzerinden çalıştığı için bunlar asla önbelleğe düşmüyor. Etkilenen görünür örnek:
`vehicle/ai/peugeot_boxer/cargocolor.dds` → `mat_0012_cargocolor`, `mat_0015_cargocolor`.

### 2c) `resolveTextureHints()` uzantısız referansları atlıyor (3. sınıf)

PIT dosyaları texture yolunu **uzantısız** yazıyor:
`Value: "/vehicle/ai/jazzycat/renault_megane_2/tableaudebord"`

`pipeline.go:1002-1012`:
```go
switch {
case strings.HasSuffix(l, ".tobj"):   ...
case strings.HasSuffix(l, ".dds"), ...: ...
default:
    continue          // ← uzantısız TÜM referanslar burada eleniyor
}
```
Rapor her zaman **"On-demand package textures: 0"** gösteriyor — bu yol fiilen ölü.
Yani 2a/2b'de kaybolan dosyalar için **hiçbir yedek çekme mekanizması çalışmıyor.**

### 2d) Cam (glass) uyarıları — tasarım gereği, gerçek hata değil

970 cam uyarısı kaynaklı: `/vehicle/truck/share/glass_ex` **base.scs** içinde, modda yok.
`fallback_glass.png` = RGBA(95,110,120,**120**) → doğru, yarı saydam.
171 aracın "warn" olmasının sebebi bu (≈5.7 uyarı/araç). Hata değil, sadece gürültü.

### 2e) Doğrulanan ve **dokunulmayacak** şeyler

- Texture yol şeması (bare isim, `model\` + `texture\`) → gerçek OMSI araçlarıyla **birebir aynı** ✓
- `[matl]` dizin indeksleri (0–32, ardışık) → gerçek OMSI'da da aynı semantik ✓
- Tekerlek mesh'lerinde `[matl]` olmaması → gerçek OMSI'da 76 mesh'te aynı ✓
- O3D çıktı geometrisi: winding/normal **tutarlı** (agree=%100, oppose=%0) ✓

---

## 3) "3D görüntüleyici hataları" — kök neden tam izole

- `/api/preview` → 171/171 araç **başarısızlık yok** (backend sağlam).
- `traffic.alfa_155` önizleme JSON'u: **18.177 / 18.184 üçgen (%99.96) normal yönüne ters** winding'e sahip.
- Render edilen canvas piksel analizi: ortalama nesne parlaklığı **66/255**, piksellerin **%74'ü 34–50
  aralığında** → shader'daki `max(.22, dot(...))` klatmanına oturuyor. Yani **neredeyse siyah**.

**Neden OMSI çıktısı doğru, görüntüleyici yanlış?**

| Fonksiyon | `faceOpposesNormals()` düzeltmesi | Sonuç |
|---|---|---|
| `toO3D()` (pipeline.go:1259) → dosyaya yazılan `body.o3d` | **var** | oyun içi geometri doğru |
| `previewFromScene()` (preview.go) → izleyiciye gönderilen JSON | **yok** | izleyici siyah/neredeyse görünmez |

Ek olarak `app.js` içindeki `createRenderer`:
- `gl.enable(gl.CULL_FACE)` açık → ters yüzler **siliniyor**
- normal dizilerinde `||`-falsy varsayılan (`n[0] || 0` gibi) → `0` geçerli bir normal değeri ama
  falsy sayıldığı için varsayılana düşüyor
- `window.mouseup` → `destroy()` içinde `removeEventListener` zaten var ve `loadPreview()` yeni
  renderer yaratmadan önce eskisini `destroy()` ediyor → **dinleyici sızıntısı YOK, dokunma**

---

## Önerilen düzeltme sırası

1. **3D görüntüleyici** ✅ TAMAM (build/vet/test geçti, davranış doğrulaması beklemede): `previewFromScene()` içine `toO3D()` ile aynı
   `faceOpposesNormals()` winding düzeltmesi; `CULL_FACE` kapat; normals `||1` → `??1`.
   (`window.mouseup` sızıntısı yanlış teşhis — `destroy()` zaten temizliyor, dokunulmadı.)
2. **Texture:**
   - `resolveTextureHints()`: uzantısız referansları `.tobj` + `.dds`/`.tga` olarak dene.
   - Uzantısız hint'ler için doğrudan arşivden (`archive.Open`) kendi Go kodunla çekme — ConverterPIX'e
     bırakma (boşluk sorununu da çözer).
   - `lookupTexture`/`collectTextures`: `tableaudebord` ↔ `tableau de bord` eşleşmesi için
     boşluk/alt tire duyarlı (tight) normalize anahtar ekle; çakışma durumunda `ambiguousTexture` ile
     güvenli biçimde reddet.
   - `.tobj`'siz `.dds`'leri arşiv indeksine dahil et.
   - Uyarıları tekrarsız (body + LOD aynı mesajı iki kez yazıyor).
   - Cam fallback'lerini uyarı sayma (tasarım gereği).
3. **EXE:** görünür pencere/durum + URL gösterimi, tek-örnek kilidi, mutlak log yolu, çıkış yolu,
   `--no-browser`.

---
# ⏸️ CHECKPOINT — 2026-10-02 (burada kaldık)

## Tamamlanan: Fix #1 — 3D görüntüleyici (KOD YAZILDI + `go build` ✅ / `go vet` ✅ / `go test ./...` ✅ HEPSİ GEÇTİ)

| Dosya | Değişiklik | Durum |
|---|---|---|
| `internal/convert/pipeline.go` | `faceOpposesNormals`'ın yanına `sceneFaceOpposesNormals(sc, a,b,c)` eklendi (scene.Vertex tabanlı, aynı winding kuralı) | ✅ yazıldı |
| `internal/convert/preview.go` | `previewFromScene` içinde `add(t.A), add(t.B), add(t.C)` yerine `ia,ib,ic` + `sceneFaceOpposesNormals` kontrolü ve `ib,ic = ic,ib` flip'i | ✅ yazıldı |
| `cmd/ets2omsi-alpha/web/app.js` | `gl.enable(gl.CULL_FACE)` → `gl.disable(gl.CULL_FACE)` | ✅ yazıldı |
| `cmd/ets2omsi-alpha/web/app.js` | `d.normals[vi*3+2]??1` (eski: `\|\|1` → Z=0 olan normal'ler 1'e çevriliyordu) | ✅ yazıldı |

**NOT:** `window.mouseup` dinleyici sızıntısı aslında **SORUN DEĞİL** — `destroy()` içinde
`removeEventListener('mouseup', up)` zaten var ve `loadPreview()` yeni renderer yaratmadan önce
öncekini `destroy()` ediyor. Buna dokunmadım.

### ⚠️ Henüz yapılmayan (yarın yapılacak)
- `go build ./...` → ✅ exit 0 · `go vet ./...` → ✅ exit 0 · `go test ./...` → ✅ HEPSİ `ok`
- **`go test -race ./...` çalıştırılmadı** (yarın yapılacak)
- Fix #1 için 171 araçlık dönüşümle/preview ile **davranış doğrulaması yapılmadı**

## Sıradaki: Fix #2 — texture (TASARIM HAZIR, KOD YAZILMADI)

Yeni eklenecek: `resolvePackageTextures(mounts, hints, out) (int, map[string]bool, []string)`
- `archive.Open` ile mount'ları **high→low** öncelikte aç (MountPaths low→high yazılı, ters sırada gez)
- Sadece `.dds/.tga/.png/.jpg/.jpeg/.bmp` girişlerini indeksle
- `textureTightKey(p)`: lowercase, uzantıyı at, ` / _ - . ` karakterlerini sil, `/` koru
  → `vehicle/ai/jazzycat/renault_megane_2/tableau de bord` ≡ `.../tableaudebord`
- İki indeks: **path-tight** (birincil, güvenli) ve **base-tight** (yedek, `baseAmbiguous` guard'ı ile)
- Başarılıysa `handled[key]=true` işaretle (ref sahibi native extractor)
- Yazılacak dosya adı: `filepath.Base(entry)`; boşluk **içeriyorsa** `filepath.Base(ref)+ext` kullan
- Çakışma guard'ı: hedef varsa ve içerik farklıysa `shortHash(entryPath)+"_"+name`
- `archive.Open` hatası olursa sessizce geç (HashFS paketlerinde HelperPaths yok → eski davranış korunur)

`resolveTextureHints` değişiklikleri:
- en başa: `native, handled, nativeWarnings := resolvePackageTextures(...)`
- döngüde: `if handled[key] { continue }`
- `default:` branch'i artık extensionless için `pixbridge.ConvertTextureObject(ref+".tobj")` denesin
  ama **hatayı sessiz yut** (uyarı üretme — yoksa uyarı sayısı artar, hedef azaltmak)
- dönüş: `native + okCount`

Uyarı temizliği:
- `applySafeMaterialFallbacks` (pipeline.go:1201) → body + LOD aynı mesajı iki kez yazıyor → tekrarsızlaştır
- `class == "glass"` fallback'i için **uyarı üretme** (970 cam uyarısının tamamı tasarım gereği)

Beklenen kazanım: `mat_0006_tableaudebord`, `mat_0013_portear`, `mat_0014_meganephare`,
`mat_0023_jante512`, `mat_0011_panel2`, `mat_0015_kolorwheel`, `mat_0007_itdplast`,
`mat_0009/0011_dorrtap`, `mat_0011/0013_dzrwitap`, `mat_0012/0015_cargocolor`,
`mat_0008_white`, `mat_0009/0014_black` → hepsi çözülmeli.
**Kalan fallback'ler** (`golf_wheel`, `bmw_wheel`, `jaguar_wheel`, `peugeot_wheel_color`) →
`.tobj`'leri `/vehicle/wheel/16in/golf_wheel.dds` gibi **paket dışına (base.scs)** işaret ediyor,
SCS-only kapsamda gerçeğe doğru fallback kalacak.

## Sonra: Fix #3 — exe (`cmd/ets2omsi-alpha/main.go`)
- görünür pencere/durum + URL'i ekranda göster
- tek-örnek (single-instance) kilidi
- log yolunu **mutlak** yap (şu an göreli: `ETS2OMSI_V2_2_MATERIAL_PHYSICS.log`)
- çıkış (exit) yolu + `--no-browser`

## Son: Doğrulama
1. `go build ./...` → `go vet ./...` → `go test ./...` → `go test -race ./...`
2. rebuild
3. 171 araçlık dönüşümü yeniden çalıştır:
   `POST /api/convert {"ids":[...],"output_root":"..."}`
   - `output_all\*\conversion_report.json` → `textures.unresolved` içindeki boşluklu isimlerin **kaybolması**
   - gövde fallback sayısının **49'dan düşmesi**
   - cam fallback'lerinin uyarı **saymaması** → 171 `warn` düşmeli
4. `GET /api/preview?id=<id>&mode=source` → winding'in artık normal ile **uyumlu** olduğunu doğrula

## Ortam notları
- Go: `C:\Program Files\Go\bin\go.exe` (yeni shell'de PATH'e yeniden ekle)
- Test sunucusu PID 8776 → `http://127.0.0.1:63010/` (`runtest\`), yeniden başlatmak gerekebilir
- API: `POST /api/scan` `{"path":"..."}` · `POST /api/convert` `{"ids":[...],"output_root":"..."}`
  · `GET /api/preview?id=<vehicleID>&mode=source|final` · scan'de JSON anahtarı **`path`** (`package_path` değil → 400)
- Kullanıcı error metni/ekran görüntüsü vermedi → "şimdilik yok, devam et" dedi

---

## Doğrulama planı

`go test ./...` → `go test -race ./...` → `go vet ./...` → rebuild → 171 araçlık dönüşümü yeniden çalıştır
→ gövde fallback sayısının 49'dan düşmesini, `unresolved` listesindeki boşluklu isimlerin kaybolmasını bekle.
