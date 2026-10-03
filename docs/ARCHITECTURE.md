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
