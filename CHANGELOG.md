# Sürüm notları

Biçim [Keep a Changelog](https://keepachangelog.com/tr-TR/1.1.0/) esas alınarak hazırlanmıştır.

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
