# Karecik — Sıfırdan Kurulum Rehberi

Bu rehber, bilgisayarında **hiçbir şey kurulu değilken** (Go, Node.js, PostgreSQL yok)
projeyi çalışır hale getirmen için gereken her adımı içerir.

Toplam süre: ~20-30 dakika.

---

## 0. Gereken Yazılımlar (özet)

| Yazılım | Sürüm | Ne işe yarar |
|---|---|---|
| Go | 1.22+ | Backend (API sunucusu) |
| Node.js | 20 LTS+ | Frontend (React derleyici) |
| PostgreSQL | 15+ | Veritabanı |
| Git | herhangi | Sürüm kontrolü (zaten kurulu) |

---

## 1. Go Kurulumu

### Windows

1. https://go.dev/dl/ adresine git.
2. **`go1.23.x.windows-amd64.msi`** dosyasını indir (MSI olan, ZIP olmayan).
3. Çift tıkla → İleri → İleri → Kur. Varsayılan konum `C:\Program Files\Go` kalsın.
4. **Kurulumdan sonra açık olan TÜM terminalleri kapat ve yeniden aç.** (PATH değişkeni
   ancak yeni terminalde görünür.)
5. Doğrula:

```powershell
go version
# çıktı: go version go1.23.5 windows/amd64
```

> `go: command not found` alıyorsan: Windows arama → "ortam değişkenleri" → *Sistem
> değişkenleri* → `Path` → *Düzenle* → *Yeni* → `C:\Program Files\Go\bin` ekle → OK.
> Terminali yeniden aç.

### macOS

```bash
# Homebrew yoksa önce onu kur:
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

brew install go
go version
```

---

## 2. Node.js Kurulumu

### Windows

1. https://nodejs.org/en/download adresine git.
2. **LTS** sürümünü (`node-v22.x.x-x64.msi`) indir.
3. Kur. Kurulum sırasında "Automatically install the necessary tools..." kutusunu
   *işaretleme* (gerekmiyor, kurulumu çok uzatır).
4. Terminali kapat/yeniden aç ve doğrula:

```powershell
node -v    # v22.x.x
npm -v     # 10.x.x
```

### macOS

```bash
brew install node@22
node -v
npm -v
```

---

## 3. PostgreSQL Kurulumu

### Windows

1. https://www.postgresql.org/download/windows/ → "Download the installer" (EDB).
2. `postgresql-18.x-windows-x64.exe` indir ve çalıştır.
3. Kurulum sihirbazında:
   - **Components**: `PostgreSQL Server`, `pgAdmin 4`, `Command Line Tools` işaretli olsun.
     (Stack Builder'ı işaretleme.)
   - **Password**: `postgres` kullanıcısı için bir şifre belirle.
     **Bu şifreyi bir yere not et**, birazdan `.env` dosyasına yazacağız.
     (Rehberde `postgres123` varsayıyoruz.)
   - **Port**: `5432` (varsayılan, değiştirme).
   - **Locale**: `Turkish, Turkey` ya da `Default locale` — ikisi de olur.
4. Kurulum bitince `psql`'i PATH'e eklemek için:
   Ortam Değişkenleri → `Path` → Yeni → `C:\Program Files\PostgreSQL\18\bin`
   (kurduğun sürüm farklıysa klasör adı da farklı olur: `...\PostgreSQL\17\bin` vb.)
5. Terminali yeniden aç, doğrula:

```powershell
psql --version    # psql (PostgreSQL) 18.x
```

### macOS

```bash
brew install postgresql@17
brew services start postgresql@17
psql --version
```

---

## 4. Veritabanını Oluştur

Karecik `karecik` adlı bir veritabanı bekliyor. Tabloları **kendisi otomatik oluşturur**
(migration'lar backend açılışında çalışır), senin sadece boş veritabanını açman yeterli.

### Windows (PowerShell)

```powershell
# psql'e postgres kullanıcısıyla bağlan (kurulumda verdiğin şifreyi soracak)
psql -U postgres -h localhost
```

Açılan `postgres=#` isteminde:

```sql
CREATE DATABASE karecik;
\l
\q
```

Tek satırda yapmak istersen:

```powershell
psql -U postgres -h localhost -c "CREATE DATABASE karecik;"
```

### macOS

```bash
createdb karecik
# ya da
psql postgres -c "CREATE DATABASE karecik;"
```

### pgAdmin ile (grafik arayüz tercih edersen)

pgAdmin 4'ü aç → Servers → PostgreSQL 18 (şifreyi gir) → **Databases** üzerine sağ tık →
*Create* → *Database...* → Database: `karecik` → Save.

---

## 5. Backend'i Çalıştır

```powershell
cd C:\Users\Arda\Desktop\karecik\backend
```

### 5.1 `.env` dosyasını oluştur

Depoda `.env.example` var. Kopyala:

```powershell
# Windows PowerShell
Copy-Item .env.example .env
```

```bash
# macOS / Git Bash
cp .env.example .env
```

Sonra `.env` dosyasını aç ve `DATABASE_URL` satırındaki şifreyi PostgreSQL kurulumunda
belirlediğin şifreyle değiştir:

```env
DATABASE_URL=postgres://postgres:postgres123@localhost:5432/karecik?sslmode=disable
JWT_SECRET=bu-degeri-mutlaka-degistir-en-az-32-karakter-olsun
PORT=8080
APP_DOMAIN=karecik.com
DEV_DOMAIN=localhost
CORS_ORIGINS=http://localhost:5173
UPLOAD_DIR=./uploads
SEED_DEMO=true
```

**Connection string yapısı** (`DATABASE_URL`):

```
postgres://KULLANICI:ŞİFRE@HOST:PORT/VERİTABANI?sslmode=disable
          └───┬────┘ └─┬──┘ └───┬───┘ └─┬┘ └──┬───┘
          postgres   senin   localhost 5432 karecik
                     şifren
```

> Şifrende `@ : / ? #` gibi karakterler varsa **URL-encode** et:
> `@` → `%40`, `#` → `%23`, `/` → `%2F`.
> Örn. şifre `pa@ss` ise: `postgres://postgres:pa%40ss@localhost:5432/karecik?sslmode=disable`
>
> `sslmode=disable` yerel geliştirme içindir. Sunucuya (production) alırken `require` yap.

### 5.2 Go paketlerini indir

`go.mod` dosyası hazır. Bağımlılıkları indirmek için:

```powershell
go mod tidy
```

Bu komut şunları indirir:

| Paket | Görev |
|---|---|
| `github.com/gofiber/fiber/v2` | HTTP framework (router, middleware) |
| `github.com/jackc/pgx/v5` | PostgreSQL sürücüsü + connection pool |
| `github.com/golang-jwt/jwt/v5` | JWT token üretimi/doğrulaması |
| `golang.org/x/crypto` | bcrypt ile şifre hashleme |
| `github.com/joho/godotenv` | `.env` dosyasını okuma |
| `github.com/google/uuid` | UUID üretimi |

İnternet bağlantısı gerekir; paketler `C:\Users\<kullanıcı>\go\pkg\mod` altına iner.

### 5.3 Sunucuyu başlat

```powershell
go run ./cmd/api
```

Beklenen çıktı:

```
[karecik] .env yuklendi (.env)
[karecik] veritabanina baglanildi (localhost:5432/karecik)
[karecik] migration uygulandi: 001_init.sql
[karecik] demo menu olusturuldu -> demo@karecik.com / demo1234 (slug: demo-kafe)
[karecik] sunucu calisiyor -> http://localhost:8080
```

Test et: tarayıcıda http://localhost:8080/api/health → `{"status":"ok","database":"up"}`

> **Hata: `dial tcp [::1]:5432: connectex: No connection could be made`**
> PostgreSQL servisi çalışmıyor. Windows arama → "Hizmetler" (Services) →
> `postgresql-18` → sağ tık → Başlat.
>
> **Hata: `password authentication failed for user "postgres"`**
> `.env` içindeki şifre yanlış. Kurulumda girdiğin şifreyi kontrol et.
>
> **Hata: `database "karecik" does not exist`**
> 4. adımı atlamışsın: `psql -U postgres -c "CREATE DATABASE karecik;"`

---

## 6. Frontend'i Çalıştır

**Yeni bir terminal aç** (backend'in çalıştığı terminali kapatma):

```powershell
cd C:\Users\Arda\Desktop\karecik\frontend
npm install
npm run dev
```

`npm install` şunları kurar: `react`, `react-router-dom`, `vite`, `tailwindcss`,
`@dnd-kit/core`, `@dnd-kit/sortable`, `lucide-react` (ikonlar), `qrcode` (QR üretimi).

> **Not:** Landing page'de hiçbir animasyon kütüphanesi yoktur (framer-motion, gsap,
> aos vb. bilerek kurulmamıştır).

Çıktı:

```
  VITE v5.4.x  ready in 420 ms
  ➜  Local:   http://localhost:5173/
```

Tarayıcıda http://localhost:5173 → Landing page açılır.

---

## 7. Subdomain'i Yerelde Test Etme

Canlıda menüler `isletmeadi.karecik.com` adresinden açılır. Yerelde bunu üç yolla test edersin:

### Yol A — Yol tabanlı (en kolay, hiçbir ayar gerekmez)

```
http://localhost:5173/m/demo-kafe
```

### Yol B — Gerçek subdomain (Chrome/Edge/Firefox `*.localhost` destekler)

```
http://demo-kafe.localhost:5173
```

Chrome ve Edge `*.localhost` alt alan adlarını otomatik olarak `127.0.0.1`'e çözer,
**hosts dosyasını düzenlemeye gerek yoktur.** Safari desteklemez.

### Yol C — hosts dosyası (Safari veya özel alan adı istersen)

Windows: Not Defteri'ni **yönetici olarak** aç →
`C:\Windows\System32\drivers\etc\hosts` dosyasını aç → en alta ekle:

```
127.0.0.1 karecik.local
127.0.0.1 demo-kafe.karecik.local
```

macOS: `sudo nano /etc/hosts` ile aynı satırları ekle.

Sonra `frontend/.env` içine `VITE_ROOT_DOMAIN=karecik.local` yaz, `backend/.env` içine
`DEV_DOMAIN=karecik.local` yaz ve `http://demo-kafe.karecik.local:5173` adresini aç.

---

## 8. Demo Hesap

Migration'lar örnek bir kafe menüsü oluşturur (landing page'deki iPhone iframe'i bunu gösterir):

```
E-posta: demo@karecik.com
Şifre:   demo1234
Menü:    http://localhost:5173/m/demo-kafe
```

---

## 9. Günlük Kullanım (her seferinde)

```powershell
# 1. Terminal — backend
cd C:\Users\Arda\Desktop\karecik\backend
go run ./cmd/api

# 2. Terminal — frontend
cd C:\Users\Arda\Desktop\karecik\frontend
npm run dev
```

PostgreSQL Windows'ta servis olarak arka planda otomatik çalışır, ayrıca başlatman gerekmez.

---

## 10. Üretime Alırken (özet)

```powershell
# Frontend'i derle → frontend/dist klasörü oluşur
cd frontend
npm run build

# Backend'i tek bir .exe olarak derle
cd ../backend
go build -o karecik.exe ./cmd/api
```

- DNS'te wildcard A kaydı aç: `*.karecik.com` → sunucu IP'si.
- Wildcard SSL sertifikası al (Let's Encrypt DNS-01 challenge ile `*.karecik.com`).
- `.env`: `APP_DOMAIN=karecik.com`, `SERVE_STATIC=true`, `STATIC_DIR=../frontend/dist`,
  `sslmode=require`, güçlü bir `JWT_SECRET`, `SEED_DEMO=false`.
- Backend `SERVE_STATIC=true` iken frontend derlemesini de kendisi sunar; tek binary yeterlidir.
