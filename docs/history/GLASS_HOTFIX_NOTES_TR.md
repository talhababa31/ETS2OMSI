# V2.2 Material + Physics

Bu hotfix, `mat_0001_glass_ex` benzeri legacy ETS2 trafik aracı cam materyallerini düzeltir.

Bazı eski ETS2 AI modellerinde cam shader materyali geçerlidir ancak ayrı bir `texture_base`/diffuse texture referansı yoktur. V2.1 bunu eksik görünür texture sanıp dönüşümü durduruyordu.

V2.1.1 davranışı:

- Cam materyalinin **açık bir texture referansı yoksa** OMSI için nötr, yarı şeffaf bir cam materyali oluşturulur ve dönüşüm devam eder.
- Cam materyali **açıkça bir texture dosyasına referans veriyorsa** ve o dosya bulunamazsa dönüşüm yine durur. Gerçek eksik texture gizlenmez.
- `trucklight`, shadow/occlusion gibi ETS2 yardımcı materyalleri önceki gibi opsiyoneldir.

Bu değişiklik özellikle Alfa Giulietta hata ekranındaki `mat_0001_glass_ex ... mat_0004_glass_ex` durumunu hedefler.
