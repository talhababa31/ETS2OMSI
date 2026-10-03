# ETS2OMSI V2.2.1 — Nasıl Kullanılır

ETS2 trafik paketindeki (.scs) AI arabaları OMSI 2 AI aracına çevirir.

## 1. İndir

1. GitHub'da bu sayfanın üstündeki **Code → Download ZIP**'e bas (veya `release` klasörüne girip exe'yi tek tek indir).
2. ZIP'i bir klasöre çıkar (örnek: `C:\ETS2OMSI`).
3. Kullanacağın dosya: **`release\ETS2OMSI_V2_2_1.exe`**

> Go, Blender ya da ETS2 kurulu olmasına gerek yok. 3D motoru (ConverterPIX) ilk dönüşümde kendi kendine iner, bunun için internet lazım.

## 2. Çalıştır

1. Eski sürüm açık kaldıysa önce **Görev Yöneticisi**'nden `ETS2OMSI_V2_2_MATERIAL_PHYSICS.exe` süreçlerinin hepsini kapat.
2. `ETS2OMSI_V2_2_1.exe`'ye çift tıkla.
3. Küçük bir pencere çıkar, içinde adres yazar (ör. `http://127.0.0.1:53412/`). Tarayıcı kendisi açılır.
   - Tarayıcı açılmazsa: pencerede **Evet**'e bas ya da adresi tarayıcıya yaz.
   - Programı kapatmak için: pencerede **Hayır** veya arayüzde sağ üstteki **⏻** düğmesi.
4. Windows SmartScreen uyarı verirse: **Ek bilgi → Yine de çalıştır**.

## 3. Arabaları çevir

1. **SCS Dosyası Seç** → trafik paketini seç (ör. `ai_traffic_pack_by_Jazzycat_v2.9.scs`).
2. **Arabaları Bul**'a bas, liste gelsin.
3. **Çıktı klasörü** seç (boş bırakırsan exe'nin yanında `output` klasörüne yazar).
4. İstediğin arabaları işaretle → dönüştür.
5. İstersen önce 3D önizlemeye bakabilirsin.

## 4. OMSI 2'ye koy

1. Çıktıdaki her araba klasörünü `OMSI 2\Vehicles\` içine kopyala.
2. Her araç klasöründe `ailists_snippet.txt` var; içindeki satırı haritanın `ailists.cfg` dosyasındaki AI listesine ekle.
3. OMSI'yi aç, trafikte arabaları kontrol et.

## Sorun olursa

- Log dosyası: `%LocalAppData%\ETS2OMSI\ETS2OMSI_V2_2_MATERIAL_PHYSICS.log` (Windows tuşu + R → bu yolu yapıştır).
- Her aracın klasöründe `conversion_report.json` ve rapor metni var; hata/uyarılar orada yazar.
- Cam texture'ı ve bazı jantlar (golf_wheel, bmw_wheel...) ETS2'nin kendi base.scs dosyasında olduğu için yedek (fallback) texture ile gelir, bu normal.
