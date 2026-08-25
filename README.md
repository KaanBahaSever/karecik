# Karecik

**İşletmeniz için QR arayüzü.** Kafeler ve restoranlar için çok kiracılı QR menü
platformu: işletme kaydolur, menüsünü sürükle-bırak ile hazırlar, müşteriler
`isletmeadi.karecik.com` adresinden menüyü görür.

```
Go + Fiber   ·   PostgreSQL   ·   React + Vite + Tailwind   ·   @dnd-kit
```

---

## Hızlı Başlangıç

### En kolay yol (Windows)

Kurulum bir kez yapıldıktan sonra proje klasöründeki dosyaya **çift tıkla**:

| Dosya | Ne yapar |
|---|---|
| **`basla.bat`** | Go / Node.js / PostgreSQL kontrol eder, eksik paketleri kurar, iki sunucuyu ayrı pencerelerde başlatır, hazır olunca tarayıcıyı açar |
| **`durdur.bat`** | Sunucuları kapatır (yalnızca 8080 ve 5173 portlarını — başka uygulamalara dokunmaz) |

`basla.bat` ilk çalıştırmada `.env` yoksa oluşturup Not Defteri'nde açar; oradaki
`DATABASE_URL` satırına PostgreSQL şifreni yazman yeterlidir.

### Elle çalıştırmak istersen

> Bilgisayarında Go, Node.js veya PostgreSQL kurulu değilse
> **[docs/KURULUM.md](docs/KURULUM.md)** dosyasını baştan sona takip et — indirme
> bağlantıları, kurulum adımları ve bağlantı ayarları orada adım adım anlatılıyor.

```powershell
# 1. Veritabanını oluştur (bir kez)
psql -U postgres -c "CREATE DATABASE karecik;"

# 2. Backend  (1. terminal)
cd backend
Copy-Item .env.example .env      # DATABASE_URL içindeki şifreyi düzenle
go mod tidy
go run ./cmd/api                 # http://localhost:8080

# 3. Frontend (2. terminal)
cd frontend
npm install
npm run dev                      # http://localhost:5173
```

Tablolar sunucu açılışında otomatik oluşturulur; ayrıca migration çalıştırman gerekmez.

**Demo hesap:** `demo@karecik.com` / `demo1234` — örnek kafe menüsü:
[localhost:5173/m/demo-kafe](http://localhost:5173/m/demo-kafe)

---

## Özellikler

### Açılış sayfası
- Sade, temiz, **tamamen animasyonsuz** karşılama ekranı (bilinçli tasarım kararı —
  hiçbir animasyon kütüphanesi bağımlılığa eklenmemiştir).
- Sağda iPhone çerçevesi içinde `<iframe>` ile çalışan **gerçek** örnek kafe menüsü.
- Modal üzerinden kayıt: İşletme Adı · E-posta · Şifre.

### Yönetim paneli
- Kategori ve ürün ekleme / silme / düzenleme.
- **Sürükle-bırak** ile kategori ve ürün sıralama (@dnd-kit, klavye desteğiyle).
- Satır içi **hızlı fiyat düzenleme**.
- Ürün detayları: görsel, açıklama, içindekiler, **alerjen uyarıları**, öne çıkarma.
- **Her dilde ürün ekleme** — Türkçe, İngilizce, Almanca, Rusça, Arapça, Fransızca.
- **Para birimi** seçimi: varsayılan ₺, ayrıca $ € £ ₼ ₽ ﷼ د.إ.
- **Toplu fiyat güncelleme**: tek tuşla yüzde bazlı zam/indirim + fiyat yuvarlama
  (tam sayı, 5'in katı, 10'un katı, 0,50 · …,95 · …,99).
- **Tasarım**: 6 tema, 8 yazı tipi, özel ana renk.
- **Canlı önizleme**: yaptığın değişiklik cihaz ekranında anında görünür.
- **QR kod**: PNG indirme, yazdırma, adres kopyalama.

### Müşteri menüsü
- `isletmeadi.karecik.com` — wildcard subdomain, menü PostgreSQL'den dinamik gelir.
- Panelden **açılıp kapatılabilen ve özelleştirilebilen** kısa karşılama ekranı
  (logo, arka plan rengi, süre, metin).
- Logo en üst solda, altında kategori kartları.
- Ürün detay ekranı, arama, dil değiştirme.
- **Otomatik yasal ibareler**:
  "Fiyatlarımız 24.08.2026 tarihinden itibaren geçerlidir." ·
  "Fiyatlarımıza KDV dahildir."
  (Tarih, toplu fiyat güncellemesinde kendiliğinden yenilenir.)

---

## Belgeler

| Dosya | İçerik |
|---|---|
| [docs/KURULUM.md](docs/KURULUM.md) | Go, Node.js ve PostgreSQL kurulumu; `.env` ve bağlantı dizesi; subdomain'i yerelde test etme |
| [docs/API.md](docs/API.md) | Tüm API uçları, istek/yanıt gövdeleri, hata kodları |
| [docs/MIMARI.md](docs/MIMARI.md) | Klasör yapısı, çok kiracılılık, subdomain çözümleme, veri modeli, teknoloji kararları |
| [docs/FRONTEND-SOZLESME.md](docs/FRONTEND-SOZLESME.md) | Ortak frontend modüllerinin imzaları |

---

## Proje Yapısı

```
backend/     Go + Fiber API      → cmd/api, internal/{config,database,models,
                                     repository,handlers,middleware,router,utils}
frontend/    React + Vite        → src/{lib,themes,locales,components,pages}
docs/        Belgeler
```

---

## Üretime Alma

```powershell
cd frontend && npm run build          # frontend/dist
cd ../backend && go build -o karecik.exe ./cmd/api
```

`.env` içinde `SERVE_STATIC=true` yapılırsa backend derlenmiş frontend'i de sunar;
tek bir çalıştırılabilir dosya yeterlidir. DNS'te `*.karecik.com` wildcard A kaydı
ve wildcard SSL sertifikası gerekir. Ayrıntılar: [docs/KURULUM.md](docs/KURULUM.md) §10.
