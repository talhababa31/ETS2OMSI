<div align="center">

<img src="assets/brand/banner.png" alt="ETS2OMSI" width="100%">

**Euro Truck Simulator 2 trafik araçlarını OMSI 2 AI araçlarına dönüştürür.**

[![Sürüm](https://img.shields.io/badge/s%C3%BCr%C3%BCm-2.6.0-8CF03C?style=for-the-badge&labelColor=202328)](CHANGELOG.md)
[![Platform](https://img.shields.io/badge/platform-Windows%2010%2F11-8CF03C?style=for-the-badge&labelColor=202328)](#indirme)
[![Hedef](https://img.shields.io/badge/hedef-OMSI%202-8CF03C?style=for-the-badge&labelColor=202328)](#nasıl-çalışır)
[![Go](https://img.shields.io/badge/Go-1.23%2B-8CF03C?style=for-the-badge&logo=go&logoColor=8CF03C&labelColor=202328)](BUILD.md)

[**⬇ İndir**](#indirme) &nbsp;·&nbsp; [Kullanım](#kullanım) &nbsp;·&nbsp; [Nasıl çalışır](#nasıl-çalışır) &nbsp;·&nbsp; [Sorun giderme](#sorun-giderme) &nbsp;·&nbsp; [Geliştirme](#geliştirme) &nbsp;·&nbsp; [Sürüm notları](CHANGELOG.md)

</div>

---

## Genel bakış

ETS2OMSI, tek bir ETS2 trafik paketindeki (`.scs`) otomobilleri tarar ve her birini OMSI 2'ye kopyalanmaya hazır bir AI aracı olarak dışa aktarır: gövde ve ayrı tekerlek modelleri (`.o3d`), texture'lar, `model.cfg`, `.ovh` ve AI ayarları.

| Gerekmez | Gerekir |
|---|---|
| ETS2 kurulumu, DLC, Blender, Go | Windows 10/11 ve bir ETS2 trafik paketi (`.scs`) |

> Kapsam: yalnızca otomobiller, yalnızca dış görünüş. Tır, otobüs ve iç mekân dönüştürülmez.

## Özellikler

- **Tek pencerede çalışma** — paket tarama, araç listesi, 3D önizleme ve toplu dönüştürme.
- **Oyun benzeri 3D önizleme** — gerçek texture'lar, ışık, yansıma, saydam cam; önizleme OMSI çıktısıyla birebir aynı veriyi kullanır.
- **Deterministik texture çözümleme** — her materyal için `.tobj` dosyası paketten okunur; tahmin yapılmaz.
- **Eksik texture üretimi** — ETS2'nin kendi dosyalarında kalan cam, far, stop, sinyal, krom, jant ve lastik texture'ları otomatik üretilir.
- **ETS2 boya renkleri** — materyal `diffuse` rengi ve seçili renk varyasyonu (Look) texture'a işlenir.
- **Materyal teşhis listesi** — her parçanın texture'ının nereden geldiği ya da neden bulunamadığı gösterilir ve dışa aktarılabilir.
- **Renk çeşitleri** — her araç ETS2'deki ek renkleri ve seçilen palet renkleriyle ayrı OMSI araçları olarak çıkar; trafik tek renk olmaz.
- **Araç sınıfı** — sedan, hatchback, kombi, coupe, SUV, pickup, van, minibüs otomatik bulunur ya da elle seçilir; fizik ve AI hızı sınıfa göre ayarlanır.
- **Eksik tekerlek üretimi** — tekerlek modeli olmayan araçlara (ör. panelvanlar) jant ve lastik üretilir.
- **OMSI uyumlu çıktı** — doğru eksenler, tekerlek temas noktasına göre zemin, ayrı ve animasyonlu tekerlekler, kütleden hesaplanan süspansiyon.

## İndirme

1. **Code → Download ZIP** ile depoyu indirin ve bir klasöre çıkarın.
2. Program: [`release/ETS2OMSI.exe`](release/ETS2OMSI.exe)

İlk dönüşümde 3D model çözücü (ConverterPIX) otomatik indirilir; bunun için internet bağlantısı gerekir.

## Kullanım

1. `ETS2OMSI.exe` dosyasını çalıştırın. Program kendi penceresinde açılır.
2. **SCS Dosyası Seç** ile trafik paketini seçin ve **Arabaları Bul**'a tıklayın.
3. Bir araç seçip **3D önizleme**yi açın; alttaki **Materyaller** listesinden texture durumunu kontrol edin. Gerekirse **Araç sınıfı**nı elle seçin.
4. Araçları işaretleyip **OMSI'ye Dönüştür**'e tıklayın.
5. Dönüşüm ekranında **Tüm ailists satırlarını kopyala**'ya basın; araç klasörlerini `OMSI 2\Vehicles\` içine kopyalayıp satırları haritanın `ailists.cfg` dosyasına yapıştırın.

Ayrıntılı anlatım: [KULLANIM.md](KULLANIM.md)

### Çıktı yapısı

```text
ETS2OMSI_<Araç>/
├── <Araç>.ovh
├── <Araç>_<renk>.ovh        (renk çeşitleri)
├── model/
│   ├── model.cfg · model_<renk>.cfg
│   ├── body.o3d · body_<renk>.o3d
│   ├── lod_*.o3d
│   └── wheel_fl.o3d · wheel_fr.o3d · wheel_rl.o3d · wheel_rr.o3d
├── texture/
├── script/AI_constfile.txt
├── conversion_report.json · conversion_report.txt
└── ailists_snippet.txt
```

## Nasıl çalışır

```mermaid
flowchart LR
    A[Trafik .scs] --> B[Tarayıcı<br/>traffic_storage · SII]
    B --> C[Araç<br/>PMD/PMG · Look · tekerlekler]
    C --> D[ConverterPIX<br/>PIM/PIT]
    D --> E[İç sahne<br/>geometri · UV · materyal]
    E --> F[Texture çözümleme<br/>.tobj → resim]
    F --> G[OMSI çıktısı<br/>O3D · model.cfg · OVH]
```

**Texture çözümleme kuralları**

| Adım | Kural |
|---|---|
| 1 | Materyal → PIT'teki seçili Look → texture nesnesi (`.tobj`) |
| 2 | `.tobj` paketten okunur: resim yolu ve kenar davranışı (tekrar / clamp / ayna) |
| 3 | Resim yalnızca tam yolundan ya da aynı klasörde boşluk/alt çizgi farkıyla aranır |
| 4 | Ayna texture'lar OMSI için dönüştürülür; DX10 DDS ve TGA, OMSI uyumlu DDS'e çevrilir |
| 5 | Pakette olmayan texture'lar parça türüne göre üretilir (cam, far, stop, sinyal, krom, jant, lastik, iç mekân) |

Ayrıntılar: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)

## Sorun giderme

| Belirti | Çözüm |
|---|---|
| Texture yanlış ya da eksik | Önizlemedeki **Materyaller** listesine bakın; **Teşhis dosyasını kaydet** ile oluşan `.json` dosyasını bir hata kaydına ekleyin. |
| Program açılmıyor | Log dosyası: `%LocalAppData%\ETS2OMSI\ETS2OMSI.log` |
| Pencere yerine tarayıcı açılıyor | Edge WebView2 yüklü değil; program tarayıcı moduna geçer, işlev aynıdır. |
| Araç OMSI'de görünmüyor | `ailists.cfg` satırını ve klasör adının `Vehicles` altında olduğunu kontrol edin. |

Hata bildirmek için: [yeni hata kaydı](https://github.com/talhababa31/ETS2OMSI/issues/new/choose)

## Geliştirme

```powershell
go vet ./...
go test -race ./...
$env:GOOS="windows"; $env:GOARCH="amd64"
go build -trimpath -ldflags "-s -w -H windowsgui" -o release/ETS2OMSI.exe ./cmd/ets2omsi-alpha
```

| Klasör | İçerik |
|---|---|
| `cmd/ets2omsi-alpha` | Masaüstü uygulaması (pencere, HTTP API, web arayüzü) |
| `cmd/ets2omsi-cli` | Komut satırı aracı |
| `internal/scanner` · `internal/sii` | Paket tarama, SII ayrıştırma |
| `internal/convert` | Dönüştürme hattı, texture çözümleme, önizleme |
| `internal/dds` | DDS çözücü (BC1–BC5, sıkıştırmasız, DX10) |
| `internal/o3d` · `internal/omsi` | OMSI O3D yazıcı, `model.cfg` / `.ovh` üretimi |

Derleme ayrıntıları: [BUILD.md](BUILD.md)

## Üçüncü taraf

- [ConverterPIX](https://github.com/mwl4/ConverterPIX) (LGPL-3.0) — ETS2 ikili modellerini çözmek için ayrı bir araç olarak kullanılır.
- Kütüphaneler: [go-webview2](https://github.com/jchv/go-webview2).
- Logo ve marka dosyaları: [`assets/brand`](assets/brand) (SVG, PNG, ICO, paylaşım görseli). Yazı tipi: [Anton](https://fonts.google.com/specimen/Anton) (SIL OFL 1.1).

ETS2OMSI; ETS2, DLC, mod araç dosyaları veya OMSI içeriği dağıtmaz. Ayrıntı: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
