# ETS2OMSI V2.2.3 — ETS2 Trafik Arabalarını OMSI 2'ye Çevirici

ETS2 (Euro Truck Simulator 2) trafik paketlerindeki (`.scs`) AI arabaları alır, **OMSI 2 AI aracı** (`.ovh` + `.o3d` + texture) olarak çıkarır.

- ETS2 kurulu olması **gerekmez**, sadece trafik `.scs` dosyası yeter.
- Blender, Go veya başka program **gerekmez**.
- Sadece **otomobiller**, sadece **dış görünüş** (tır, otobüs, iç mekân yok).

## ⬇️ İndir ve kullan

1. Bu sayfada yeşil **Code → Download ZIP**'e bas ve ZIP'i bir klasöre çıkar.
2. `release\ETS2OMSI.exe`'ye çift tıkla → program **kendi penceresinde** açılır.
3. **SCS Dosyası Seç** → **Arabaları Bul** → bir arabaya tıkla → **3D önizleme**ye bak → arabaları işaretle → **dönüştür**.
4. Çıkan araba klasörlerini `OMSI 2\Vehicles\` içine kopyala; her klasördeki `ailists_snippet.txt` satırını haritanın `ailists.cfg` dosyasına ekle.

Ayrıntılı anlatım: **[KULLANIM.md](KULLANIM.md)** · Sürüm notları: **[CHANGELOG.md](CHANGELOG.md)**

> 3D motoru (ConverterPIX) ilk dönüşümde otomatik iner (internet gerekir). Pencere için Windows 10/11'de hazır gelen Edge WebView2 kullanılır; yoksa program tarayıcıda açılır.

## ✨ Son sürümde (V2.2.3) neler var

| | |
|---|---|
| 🎨 **Gerçek boya renkleri** | ETS2'nin materyal rengi (diffuse) okunup texture'a işleniyor — arabalar artık gri değil, kendi renginde. Doğru renk varyasyonu (Look) seçiliyor. |
| 🛞 **Süspansiyon düzeltildi** | Yay/amortisör değerleri araç kütlesinden hesaplanıyor; araç artık ~18 cm çökmüyor, tekerler çamurluğa gömülmüyor. |
| 🪟 **Kendi penceresi** | Exe artık tarayıcı yerine kendi program penceresinde açılır. Tek kopya çalışır, pencereyi kapatınca program kapanır. |
| 🚗 **Oyun gibi 3D önizleme** | Gerçek DDS texture'lar, güneş/gökyüzü ışığı, parlama ve yansıma, saydam cam, zemin ve gölge. Önizlemede gördüğün, OMSI'ye giden sonuçla aynı. |
| 🎨 **Siyah panel düzeltmesi** | ETS2'nin kendi dosyasında (base.scs) kalan boya texture'ları artık siyah değil, aracın kendi gövde renginde. |
| 🖼️ **Texture kurtarma** | Adında boşluk olan (`tableau de bord.dds`), `.tobj`'siz ve uzantısız yazılmış texture'lar artık paketten doğrudan çıkarılıyor. |
| 🧹 **Daha az gereksiz uyarı** | Cam yedekleri uyarı sayılmıyor, tekrar eden uyarılar tekleştirildi. |

## 📁 Çıktı yapısı

```text
ETS2OMSI_<Araç>/
  <Araç>.ovh
  model/  model.cfg, body.o3d, wheel_fl/fr/rl/rr.o3d, lod_*.o3d
  texture/
  script/ AI_constfile.txt
  conversion_report.txt / .json
  ailists_snippet.txt
```

## 🔧 Sorun olursa

- Log: `%LocalAppData%\ETS2OMSI\ETS2OMSI.log`
- Her aracın klasöründe `conversion_report.json` var; hangi texture'ın neden çözülmediği orada yazar.
- Cam ve bazı jantlar (golf_wheel, bmw_wheel…) ETS2'nin base.scs dosyasında olduğu için yedek texture ile gelir — bu beklenen durum.

## 🛠️ Geliştiriciler için

- Kaynaktan derleme: [BUILD.md](BUILD.md)
- Teknik altyapı: [docs/TECHNICAL_FOUNDATION.md](docs/TECHNICAL_FOUNDATION.md)
- Jazzycat paketi analiz raporu: [docs/ANALIZ_RAPORU.md](docs/ANALIZ_RAPORU.md)
- Eski sürüm notları: [docs/history/](docs/history/) · Geliştirme devir notları: [docs/handoff/](docs/handoff/)

Akış: `.scs → tarayıcı (scanner) → araba → PMD/PMG → ConverterPIX → PIM/PIT → iç sahne → OMSI O3D/model.cfg/OVH`

---

<sub>English: ETS2OMSI converts ETS2 traffic-car exteriors from a single `.scs` package into OMSI 2 AI vehicles. Download the ZIP and run `release/ETS2OMSI.exe`. No ETS2 install, base/DLC archives or Blender needed.</sub>
