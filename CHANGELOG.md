# Changelog

## V2.2.5 — Texture mantığı baştan yazıldı

- **Kesin (tahminsiz) eşleşme:** Materyalin istediği `.tobj` paketten okunur, ETS2'nin ikili formatına göre çözülür (hangi resim, hangi kenar davranışı), resim de paketten okunur. Sadece tam yol ve aynı klasörde boşluk/alt çizgi farkı kabul edilir. Paketin başka bir yerinden "adı benzeyen" texture alınması tamamen kaldırıldı (başka araçların texture'ları gelebiliyordu).
- **Ayna/clamp kenar davranışı:** ETS2'nin aynalı texture'ları OMSI'de aynı görünecek şekilde dönüştürülüyor (texture aynasıyla birleştirilir, UV ayarlanır); clamp UV'leri sınırlanıyor.
- **OMSI uyumlu biçim:** DX10 başlıklı DDS, TGA, PNG vb. OMSI'nin kesin okuduğu sıkıştırmasız DDS'e (mipmap'li) çevriliyor; zaten uyumlu DXT dosyaları olduğu gibi kopyalanıyor.
- **Materyal teşhis listesi:** 3D önizlemenin altında her parça: ✔ paketten / ✖ pakette yok / ⚠ yedek, sebebi ve dosya yolu. Satıra tıklayınca parça 3D'de turuncu yanar. "Teşhis dosyasını kaydet" ile tek dosya halinde dışa aktarılır. Dönüşüm raporunda da (`conversion_report.json` → `materials`) aynı bilgi var.

## V2.2.4 — Texture'lar araca oturuyor (UV düzeltmesi)

- **Asıl texture hatası:** UV koordinatlarının V ekseni yanlışlıkla ters çevriliyordu (`1 - v`). ConverterPIX ETS2'nin DirectX düzenindeki UV'lerini olduğu gibi yazar, OMSI de aynı düzeni kullanır (ETS2'nin resmi Blender eklentisi çevirmeyi yalnızca Blender'a alırken yapar). Ters çevirme texture atlasının yanlış bölgesini okutuyordu: farlarda stop lambası, gövdede başka parçaların texture'ı. Hem 3D önizleme hem OMSI çıktısı düzeldi.
- **Doğru UV kanalı:** Bir parçada birden çok UV kanalı varsa artık base texture'ın kullandığı kanal (`_TEXCOORD0` etiketi) seçiliyor, körlemesine `_UV0` değil.

## V2.2.3 — Boya renkleri ve süspansiyon

- **Boya rengi:** ETS2 trafik arabalarında gövde texture'ı çoğunlukla gri tonludur, rengi materyalin `diffuse` değeri verir. Artık bu renk PIT'ten okunup texture'a işleniyor (gri araba sorunu). Texture bulunamazsa yedek boya da bu renkten yapılıyor.
- **Look (renk varyasyonu):** PIT'teki Look'lar karıştırılıyordu (son yazılan kazanıyordu). Artık aracın kendi Look'u, yoksa `default`, yoksa ilk Look seçiliyor.
- **Süspansiyon:** Yay/sönüm değerleri kütleden fiziksel olarak hesaplanıyor (≈2,3 Hz, sönüm oranı 0,45). Eski değerler (1,5 t için 40 kN/m) aracı ~18 cm çöktürüp tekerleri çamurluğa gömüyordu; şimdi ~4,5 cm.
- **Tekerlek kopyası hatası:** Aynı tekerlek modeli 4 tekerlekte paylaşılırken konumlar ortak veriyi değiştiriyordu; artık her tekerlek kendi kopyasını alıyor.

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
