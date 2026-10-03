# V2.2 Material + Physics

Bu sürüm SCS-only V2 hattındaki gerçek export blokajını düzeltir.

- UI conversion çağrısı `StrictFidelity=false` kullandığı halde V2.1.1 çözülemeyen görünür materyalde koşulsuz `FAILED` dönüyordu.
- V2.1.2 artık bu ayarı gerçekten uygular. Ana PMD/PMG başarıyla decode edildiyse çözülemeyen materyaller sınıfına göre nötr OMSI fallback alır ve dönüşüm devam eder.
- Glass fallback alpha ile, body/paint gri nötr, rubber koyu, light nötr olarak üretilir.
- Optional ETS2 helper material mesajı artık yalnız uyarıdır; exportu durdurmaz.
- Wheel texture çözümlemesi de aynı SCS-only safe export kuralına uyar.
- Orientation doğrulaması Strict Fidelity kapalıyken uyarıdır; O3D/OVH üretimini engellemez.
- Gerçek yapısal hatalar (3D decoder yok, model decode edilemiyor, O3D yazılamıyor/readback bozuk, output texture referansı fiziksel olarak kayıp) hâlâ fataldir.
