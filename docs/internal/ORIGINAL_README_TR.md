# ETS2OMSI V2.2 MATERIAL + PHYSICS — SCS ONLY AI Car Converter

Bu sürüm V2'nin dar hedefini korur: **yalnız seçilen trafik `.scs` paketindeki AI otomobillerinin dış modelini OMSI 2 `.ovh` AI aracına dönüştürmek.** ETS2 kurulumu, base/DLC mount, player truck, bus, interior veya Blender zorunluluğu yoktur.

## Golden Reference düzeltmesi

V2.1, kullanıcı tarafından sağlanan çalışan bir OMSI 2 Volkswagen Passat AI aracı referans alınarak output tarafını yeniden düzenler. Referans aracın dosyaları dağıtıma dahil edilmez; yalnız doğrulanmış OMSI koordinat/material/teker/config davranışları test kuralı olarak kullanılır.

- Internal scene: `X = lateral`, `Y = longitudinal`, `Z = vertical`.
- OMSI O3D: `X = lateral`, `Y = vertical`, `Z = longitudinal`.
- O3D exportunda Y/Z eksenleri ve normal bileşenleri aynı şekilde çevrilir; handedness değişimi için triangle winding doğrulanır.
- Araç zemini tamponun en alt vertex'inden değil, mümkünse **wheel center - wheel radius** temas düzleminden kalibre edilir.
- Tekerler gövdeye gömülmek yerine FL/FR/RL/RR olarak ayrı O3D dosyalarıdır ve OMSI wheel/suspension/steering animasyon blokları üretir.
- OMSI ortak AI script/sound yolları `..\..\Scripts\AI_Cars` ve `..\..\Sounds\AI_Cars` olarak üretilir.

## Texture politikası

Final conversion artık görünür body texture'ında fuzzy eşleştirme veya pembe/siyah checker ile 'başarılı' sayılmaz.

1. ConverterPIX PIT içindeki `Material -> Texture(texture_base)` referansı okunur.
2. `.tobj` veya doğrudan image referansı **aynı seçili SCS paketinden** ConverterPIX ile çıkarılır.
3. Material alias yalnız deterministik/benzersiz exact path, basename/stem veya ConverterPIX `mat_0000_texture` alias kuralı ile çözülür.
4. Belirsiz veya çözülemeyen görünür texture varsa conversion açık hata ile durur.
5. `trucklight_*`, shadow/occlusion/reflection gibi basic exterior için opsiyonel ETS2 helper materialleri neutral OMSI materyaline düşebilir ve uyarı üretir.

## V2.1 akışı

1. Trafik `.scs` paketini seç.
2. **Arabaları Bul** ile storage-bound `car` araçlarını listele.
3. İstersen **Seçileni Ayır** ile o araca bağlı package-local kaynakları çıkar.
4. **3D Önizle** ile source/final geometriyi kontrol et.
5. **OMSI'ye Dönüştür** ile PMD/PMG -> PIM/PIT -> Internal Scene -> O3D dönüşümü yap.
6. Program body + varsa ETS2 LOD'ları + ayrı wheel O3D'leri, texture klasörü, `model.cfg` ve `.ovh` üretir.
7. Export edilen O3D tekrar okunur; yön, texture referansları, ground ve wheel sayısı validator'dan geçer.

## Çıktı

```text
ETS2OMSI_Audi_A6/
  Audi_A6.ovh
  model/
    model.cfg
    body.o3d
    wheel_fl.o3d
    wheel_fr.o3d
    wheel_rl.o3d
    wheel_rr.o3d
    lod_1.o3d
    ...
  texture/
    ...
  script/
    AI_constfile.txt
  conversion_report.txt
  conversion_report.json
  package_manifest.json
  ailists_snippet.txt
```

## Bilerek yok

- ETS2 installation / base / DLC zorunluluğu
- truck veya bus conversion
- player interior/dashboard
- yapay zekâ advisor
- manuel texture üzerinde far seçme
- gameplay physics/script dönüştürme
- görünür texture eksikken sahte checker output

Amaç tek cümleyle: **SCS içindeki düşük/orta poly trafik otomobilini doğru yönde, doğru ölçüde, doğru texture ve tekerlerle OMSI AI aracı olarak çıkarmak.**
