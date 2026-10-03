# Changelog

## V2.2.2 — Pencere, gerçek 3D görüntüleyici, boya düzeltmesi

- Program artık tarayıcıda değil, kendi penceresinde açılıyor (Edge WebView2). Tarayıcı sadece yedek.
- 3D görüntüleyici baştan yazıldı: gerçek DDS texture'lar, güneş + gökyüzü ışığı, parlama/yansıma, saydam cam, zemin ve gölge; üçgen atlama kaldırıldı (delikler yok). Önizleme dönüşümle aynı texture ve fallback'leri kullanıyor.
- OMSI'de siyah kaput/çamurluk/tampon sorunu: base.scs'te kalan boya materyalleri artık neredeyse siyah RGB(48,48,52) yerine aracın kendi gövde texture'ındaki baskın renkle dolduruluyor.
- Yeni DDS okuyucu (BC1/BC2/BC3/BC4/BC5, sıkıştırmasız, DX10).

## V2.2.1 — Jazzycat analiz düzeltmeleri

- 3D önizleme: üçgen sarımı vertex normallerine göre düzeltiliyor (O3D çıktısıyla aynı kural); CULL_FACE kapalı, `0` normal değerleri artık doğru. Araçlar önizlemede siyah görünmüyor.
- Texture: seçilen paket artık doğrudan okunuyor. Adında boşluk olan (`tableau de bord.dds`), yanında `.tobj` olmayan (`cargocolor.dds`) ve PIT'te uzantısız yazılan texture'lar çıkarılıyor. Eşleşme sırası: tam yol → TOBJ içindeki yol → boşluk/alt çizgi duyarsız yol → paket genelinde tekil dosya adı (belirsizse reddedilir). OMSI'ye yazılan texture adlarında boşluk yok.
- Uyarılar: cam fallback'leri artık uyarı sayılmıyor (tasarım gereği, base.scs'teki paylaşılan cam); gövde+LOD tekrar eden uyarılar tekilleştirildi.
- EXE: görünür durum penceresi (URL + log yolu, "Evet = tarayıcıda aç / Hayır = kapat"), tek-örnek koruması (ikinci çift tıklama mevcut örneği açar), mutlak log yolu (`%LocalAppData%\ETS2OMSI\`), arayüzde ⏻ Kapat düğmesi + `/api/quit`, `--no-browser` ve `--no-window` seçenekleri.

## V2.2 MATERIAL + PHYSICS

- Fixed sparse PIM material indices so triangle material slots are never compressed or shifted.
- Prevented normal/mask/specular/reflection maps from being promoted to visible diffuse textures.
- Wheel models now inherit the same exact slot-safe material pipeline.
- Added automatic OMSI AI physics profiles from vehicle name + measured model dimensions.
- Added estimated mass/profile to conversion result UI.
- Suppressed harmless optional ETS2 helper-material messages from user-facing warnings.
- Kept SCS-only / cars-only / exterior-only scope.
