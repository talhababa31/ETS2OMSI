# ETS2OMSI V2.2 — Material + Physics

- Seyrek PIM material slot indeksleri artık birebir korunur. Bu düzeltme ön/arka ışık veya jant materyallerinin başka slota kaymasını engeller.
- PIT normal/mask/specular/reflection texture'ları artık diffuse texture olarak kullanmaz; yalnız görünür base/diffuse/albedo bağları seçilir.
- Jant modelleri aynı exact resolver'dan geçer ve kendi PIT bağlarını kullanır.
- Araç adı + gerçek 3D ölçülerden OMSI AI fizik profili ve yaklaşık fizik kütlesi hesaplanır. Bu değer gerçek üretici curb-weight iddiası değildir; OMSI trafik fiziği içindir.
- Zararsız ETS2 helper material fallbackleri artık kullanıcıya hata/uyarı gibi gösterilmez.
