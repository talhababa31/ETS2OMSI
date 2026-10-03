# Mimari

## Kapsam

Girdi tek bir ETS2 trafik paketidir (`.scs`). Çıktı, OMSI 2'ye kopyalanabilen AI araç klasörleridir. ETS2 kurulumu, DLC veya Blender kullanılmaz.

## Dönüştürme hattı

| Aşama | Paket | Açıklama |
|---|---|---|
| Arşiv | `internal/archive` | ZIP `.scs` doğrudan, HashFS yardımcı araçla, klasör test için |
| Tarama | `internal/scanner`, `internal/sii` | `traffic_storage*.sii`, include zincirleri, eski/yeni trafik tanımları, şasi ve model bağları |
| Model çözme | `internal/pixbridge` | ConverterPIX ile PMD/PMG → PIM/PIT (önbellekli) |
| Sahne | `internal/pim`, `internal/pit` | Geometri, normal, UV (`_TEXCOORD0` kanalı, DirectX düzeni), materyal, varyant parçaları |
| Materyal | `internal/convert/pit_looks.go` | Seçili Look, `diffuse` rengi, texture referansları |
| Texture | `internal/convert/textures.go` | `.tobj` → resim → OMSI texture; teşhis kaydı |
| Eksik texture | `internal/convert/generated_textures.go` | Parça türüne göre üretilen texture'lar |
| Çıktı | `internal/o3d`, `internal/omsi` | Eksen dönüşümü, zemin, tekerlekler, süspansiyon, `model.cfg`, `.ovh` |

## Koordinatlar

- İç sahne: X yanal, Y boylamasına, Z dikey.
- OMSI O3D: X yanal, Y dikey, Z boylamasına; normaller aynı şekilde döndürülür, üçgen yönü normallere göre doğrulanır.
- UV: ETS2 ve OMSI aynı (DirectX) düzeni kullanır; çevirme yapılmaz.

## Texture çözümleme

1. Materyal adı (ConverterPIX alias) → PIT'teki seçili Look → `texture_base` referansı (`.tobj` yolu, uzantısız).
2. `.tobj` paketten okunur (40 baytlık başlık, ardından uzunluk + yol). Başlıktaki U/V adresleme alınır.
3. Resim tam yolundan ya da aynı klasörde boşluk/alt çizgi/büyük-küçük harf farkı gözetmeden aranır. Başka klasörlerden ad benzerliğiyle texture alınmaz.
4. Çıktı biçimi: uyumlu DXT/RGB DDS olduğu gibi kopyalanır; diğerleri (DX10 başlıklı DDS, TGA, PNG) mipmap'li sıkıştırmasız DDS'e çevrilir.
5. Ayna adresleme: texture aynasıyla birleştirilir ve UV yarıya ölçeklenir. Clamp: UV [0,1] aralığına sınırlanır.
6. `diffuse` rengi texture'a işlenir.
7. Bulunamayan texture'lar için parça türü (ad, efekt, referans ve konum) belirlenir ve uygun texture üretilir.

Her materyal için `conversion_report.json → materials` altında durum (`ok` / `missing` / `fallback`), sebep ve dosya yolları yazılır.

## Süspansiyon

Aks başına yay, sönüm ve azami kuvvet kütleden hesaplanır (yaklaşık 2,3 Hz, sönüm oranı 0,45, ön/arka yük dağılımı 56/44). 1,5 tonluk bir araçta statik çökme yaklaşık 4,5 cm'dir.

## Tekerlekler ve zemin

- Tekerlek konumu ETS2 `wheel_f*` / `wheel_r*` locator'larından, yoksa gövde ölçüsünden alınır.
- Tekerlek modeli paketteyse kullanılır; değilse sentetik lastik + jant üretilir (yarıçap locator yüksekliğinden).
- Yarıçap tekerlek geometrisinden; zemin yerleştirilmiş lastiklerin en alt noktalarının medyanından hesaplanır.

## Araç sınıfı

Önce ad (model adları ve anahtar kelimeler), sonra gövde biçimi: yükseklik (van/minibüs), alçak arka kasa (pickup), yükseklik+genişlik (SUV), arka %10'un tavana göre yüksekliği (hatchback/kombi), alçak gövde (coupe), aksi halde sedan. Elle seçim her zaman önceliklidir. Sınıf; kütle aralığı, süspansiyon frekansı, ön/arka yük dağılımı ve AI azami hızını belirler.

## Renk çeşitleri ve alfa

- Opak materyallerin texture'ları alfa kanalı olmadan yazılır (`*_opq.dds`): ETS2 alfa kanalında parlaklık maskesi tutar, OMSI bunu saydamlık sayar.
- Renk çeşitleri: PIT'teki diğer Look'lar ayrı ayrı çözülür; palet renkleri için gövde boyası (baskın rengi açık ve doygunluğu düşük texture'lar) piksel bazında, göreli parlaklık korunarak boyanır. Doygun (lamba) ve koyu (trim) pikseller değişmez.
- Her çeşit kendi gövde/LOD O3D dosyalarını (`body_<id>.o3d`, `lod_N_<id>.o3d`), `model/model_<id>.cfg` ve `<Araç>_<id>.ovh` dosyasını alır; tekerlek O3D'leri ortaktır. OMSI'de `[matl]` yalnız materyal seçer (O3D'deki texture adıyla), texture değiştirmez; bu yüzden texture'lar O3D'ye yazılır.
