# Sürüm notları

Biçim [Keep a Changelog](https://keepachangelog.com/tr-TR/1.1.0/) esas alınarak hazırlanmıştır.

## [2.5.2] — 2026-10-03

### Düzeltildi
- **OMSI'de bulunamayan texture'lar:** O3D texture adları ANSI karakter setinde yazılırken diskteki dosya adları orijinal (ör. Lehçe `ł`) kalıyordu. Tüm çıktı texture adları artık ASCII.
- **Bozuk araç adları:** `.ovh` / `model.cfg` UTF-8 yazılıyordu; OMSI ANSI okuduğu için "Kırmızı", "Minibüs", "·" bozuk görünüyordu. Metin dosyaları artık ASCII (Türkçe harfler sadeleştirilir).
- **Toplu dönüştürmede yarıda kesilme:** Tek istekte 30 dakika sınırı vardı; büyük paketler renk çeşitleriyle bunu aşabiliyordu. Sınır 8 saate çıkarıldı.
- **Lamba parlaması (flare) ve gölge yüzeyleri:** Texture'ları pakette olduğunda OMSI'de opak kareler olarak çiziliyordu; artık her zaman görünmez.
- **Çıkartmalar (decal):** Saydamlık kullanmıyordu; gövde üzerinde dikdörtgen görünebiliyordu.

### Değişti
- Yeni marka kimliği: neon yeşil çizgi ikon (disket · dişli · direksiyon) ve "ETS 2OMSI" yazı logosu; README banner'ı, GitHub paylaşım görseli (`assets/brand/social.png`), uygulama ve exe simgesi.

## [2.5.1] — 2026-10-03

### Düzeltildi
- **Renk çeşitleri OMSI'de görünmüyordu:** OMSI'de `[matl]` texture değiştirmez, O3D içindeki materyali texture adıyla seçer; farklı texture adı yazılan çeşitler bu yüzden orijinal renkte kalıyordu. Her renk artık kendi gövde ve LOD O3D dosyalarını (`body_<renk>.o3d`) alır; texture'lar O3D'nin içindedir. Tekerlekler ortak kalır.

## [2.5.0] — 2026-10-03

### Eklendi
- **Renk çeşitleri:** Her araç birden fazla renkte dışa aktarılır; her renk kendi `.ovh` dosyasını alır, 3D modeller ortaktır ve `ailists_snippet.txt` tüm renkleri listeler. Kaynaklar: (1) aracın ETS2'deki ek renkleri (PIT Look'ları), (2) renk paleti — beyaz/gümüş gövde boyası, gölgelendirme korunarak seçilen renge boyanır; trim, lamba ve cam değişmez.
- Ayarlarda **Renk çeşitleri** seçimi (13 renk, varsayılan: siyah, gümüş, füme, lacivert, kırmızı); 3D önizlemede renk kutucukları.

### Düzeltildi
- **OMSI'de lekeli / yer yer saydam gövde:** ETS2'nin opak texture'lardaki alfa kanalı (parlaklık maskesi) OMSI'de saydamlık olarak kullanılıyordu. Opak parçaların texture'ları artık alfa kanalı olmadan yazılır.

## [2.4.0] — 2026-10-03

### Düzeltildi
- **Gövdenin içinin görünmesi / eksik kaput-çamurluk:** ETS2'nin gövde efekti `eut2.dif.spec.add.env`, adındaki `.add` yüzünden saydam materyal sanılıyordu; texture alfa kanalı (aslında parlaklık maskesi) saydamlık olarak kullanılıyor, gövde yer yer görünmez oluyordu. Önizlemede ve OMSI'de (`[matl_alpha]`) düzeltildi; saydamlık yalnızca gerçek alfa/blend/cam efektlerinde.
- **Tekerleklerin zemine gömülmesi:** Tekerlek yarıçapı, `bb` locator'ı olmayan modellerde lastik genişliğinin yarısı (~0,1 m) olarak alınıyordu. Yarıçap artık tekerlek geometrisinden, zemin de yerleştirilmiş lastiklerin en alt noktasından hesaplanıyor.

### Eklendi
- **Sentetik tekerlek:** Tekerlek modeli pakette olmayan (çoğu panelvan) veya hiç tanımlamayan araçlarda, ETS2 tekerlek konumlarına beş kollu jant ve lastik üretilir.
- **Araç sınıfı:** Sedan, hatchback, kombi, coupe, SUV, pickup, van/panelvan, minibüs. Önce araç adından, bulunamazsa gövde biçiminden (yükseklik, arka profil, kasa) otomatik bulunur; araç ayrıntılarında elle seçilebilir. Sınıf; kütle, süspansiyon, aks yük dağılımı, AI azami hızı ve OMSI araç adını belirler.

## [2.3.0] — 2026-10-03

### Eklendi
- Pakette bulunmayan texture'lar için parça türüne göre üretilen texture'lar: cam, far, stop lambası, sinyal, krom, jant, lastik, iç mekân, plaka, trim. Gölge/flare yardımcı yüzeyleri görünmez yapılır.
- Far ile stop lambası adla ayırt edilemediğinde aracın ön/arka yarısındaki konuma göre ayrılır.

### Değişti
- Depo düzeni: profesyonel README, mimari dokümanı, hata bildirme şablonu, `.editorconfig`, `.gitattributes`; iç geliştirme notları `docs/internal/` altına taşındı.

## [2.2.5] — 2026-10-03

### Değişti
- Texture çözümleme baştan yazıldı: `.tobj` paketten okunur ve ETS2'nin ikili biçimine göre çözülür; yalnızca tam yol ve aynı klasördeki eşleşmeler kabul edilir. Başka klasörlerden ad benzerliğiyle texture alınması kaldırıldı.
- DX10 başlıklı DDS, TGA ve PNG texture'lar OMSI uyumlu, mipmap'li DDS'e çevrilir.

### Eklendi
- Ayna ve clamp kenar davranışının OMSI'ye aktarılması.
- 3D önizlemede materyal teşhis listesi (durum, sebep, dosya yolu, parçayı vurgulama, JSON dışa aktarma); `conversion_report.json` içinde `materials`.

## [2.2.4] — 2026-10-03

### Düzeltildi
- UV koordinatlarının V ekseni yanlışlıkla ters çevriliyordu; texture'lar atlasın yanlış bölgesinden okunuyordu.
- Birden fazla UV kanalı olan parçalarda `_TEXCOORD0` kanalı seçilir.

## [2.2.3] — 2026-10-03

### Eklendi
- ETS2 materyal `diffuse` renginin texture'a işlenmesi; seçili renk varyasyonunun (Look) kullanılması.

### Düzeltildi
- Süspansiyon değerleri kütleden hesaplanır; araçların yaklaşık 18 cm çökmesi giderildi.
- Dört tekerleğin ortak model verisini değiştirmesi (konumların üst üste eklenmesi).

## [2.2.2] — 2026-10-03

### Eklendi
- Kendi penceresinde çalışan masaüstü uygulaması (Edge WebView2).
- Texture'lı, ışıklı 3D önizleme; DDS çözücü (BC1–BC5, sıkıştırmasız, DX10).

### Düzeltildi
- Çözülemeyen boya materyallerinin neredeyse siyah görünmesi.

## [2.2.1] — 2026-10-03

### Düzeltildi
- 3D önizlemede yüzlerin ters çizilmesi.
- Adında boşluk olan, `.tobj`'siz ve uzantısız referanslı texture'ların bulunamaması.
- Uygulamanın görünmez çalışması, birden fazla kopya açılması, göreli log yolu.

## [2.2.0]

- Seyrek materyal indeksleri, yalnızca diffuse texture seçimi, kütle ve ölçüye dayalı AI fizik profili.

Daha eski sürümler: [docs/history](docs/history/) · [docs/internal/FULL_CHANGELOG.md](docs/internal/FULL_CHANGELOG.md)
