# Golden Reference Notları

V2.1'de çalışan OMSI AI araç paketinin yapısı yalnız davranış referansı olarak incelendi; referans aracın model/texture dosyaları ETS2OMSI paketine kopyalanmaz.

Doğrulanan temel kurallar:

- OMSI O3D geometri eksenleri araçta X=genişlik, Y=yükseklik, Z=uzunluk düzenindedir.
- `model.cfg` animasyon/origin koordinatları araç mantığında X=lateral, Y=longitudinal, Z=height değerleriyle kullanılır.
- Dört teker ayrı O3D olabilir ve kendi `origin_trans` + wheel/suspension/steering animation bloklarını kullanabilir.
- Common AI sound/script referansları vehicle klasöründen iki seviye yukarıdaki `Sounds/AI_Cars` ve `Scripts/AI_Cars` dizinlerine gider.
- Cam/material davranışı O3D texture + `model.cfg` material override üzerinden kurulabilir.

V2.1 validator bu kurallardan ölçülebilir olanları otomatik test eder; tek bir referans aracın ölçülerini veya assetlerini başka araca kopyalamaz.
