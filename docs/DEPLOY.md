# Karecik — Railway'e deploy ve CI/CD

Bu doküman kurulumun **neden** böyle olduğunu da anlatıyor, çünkü birkaç yerde
sessizce yanlış gidebilecek şeyler var.

---

## Mimari: tek servis

```
GitHub (main)  ──push──▶  GitHub Actions (test)      ← ücretsiz, deploy'u engellemez
       │
       └────────────────▶  Railway
                             ├── karecik      (Docker: Go + gömülü React)
                             └── Postgres     (Railway eklentisi)
```

Go binary'si `SERVE_STATIC=true` ile React bundle'ını kendisi servis ediyor, yani
**iki değil bir servis** çalışıyor. Bunun iki kazancı var:

- **Maliyet.** Railway kullanıma göre faturalandırıyor; ikinci bir web servisi
  ikinci bir konteyner demek. Ayrı bir frontend servisi burada hiçbir şey
  kazandırmıyor çünkü aynı Go süreci zaten statik dosya servis edebiliyor.
- **Basitlik.** API ve menü aynı origin'den cevap veriyor, yani production'da
  CORS ayarı gerekmiyor ve `{isletme}.karecik.com` alt alan adları tek bir
  sertifika ve tek bir router arkasında duruyor.

Frontend'i ayrıca Cloudflare Pages veya Vercel'e (ücretsiz katman) koymayı da
düşündüm. Daha ucuz **değil** — Railway tarafında yine aynı tek konteyner
kalıyor — ve karşılığında CORS, ayrı bir domain ve iki ayrı deploy hattı
getiriyor. Tek servis hem daha ucuz hem daha az parça.

---

## İlk kurulum (bir kere)

### 1. Repoyu GitHub'a it

```bash
git remote -v            # zaten bir remote var mı?
git push -u origin main
```

### 2. Railway'de proje ve veritabanı

1. Railway → **New Project** → **Deploy from GitHub repo** → bu repo.
2. Railway kökteki `railway.json` ve `Dockerfile`'ı kendisi bulur; Nixpacks'e
   gerek yok. (Nixpacks bu repoda zaten çalışmazdı: iki dilli bir monorepo.)
3. Aynı projede **New** → **Database** → **Add PostgreSQL**.

### 3. Kalıcı disk — bunu atlama

Konteyner dosya sistemi **her deploy'da sıfırlanıyor**. Yüklenen logolar ve ürün
görselleri `UPLOAD_DIR` altında duruyor, yani disk olmadan ilk deploy'da hepsi
kaybolur ve menülerde kırık görseller kalır.

Railway → servis → **Settings → Volumes → Add Volume**, mount yolu:

```
/data
```

Dockerfile `UPLOAD_DIR=/data/uploads` ile geliyor ve o dizini konteyner
kullanıcısına ait olacak şekilde oluşturuyor.

> Uzun vadede S3/Cloudflare R2 daha doğru olur (birden fazla replika, yedek),
> ama tek replikada disk hem yeterli hem daha ucuz.

### 4. Ortam değişkenleri

Railway → servis → **Variables**:

| Değişken | Değer | Not |
|---|---|---|
| `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` | Referans olarak yaz, kopyalama — Railway özel ağ üzerinden bağlar |
| `JWT_SECRET` | 32+ karakter rastgele | **Zorunlu.** `APP_ENV=production` iken yoksa uygulama açılmıyor |
| `APP_DOMAIN` | `karecik.com` | Menü alt alan adlarının kökü |

`openssl rand -base64 48` iyi bir secret üretir.

Dockerfile'ın verdiği ve dokunmana gerek olmayanlar: `HOST=0.0.0.0`,
`PORT=8080`, `APP_ENV=production`, `SERVE_STATIC=true`, `STATIC_DIR=/app/static`,
`UPLOAD_DIR=/data/uploads`, `SEED_DEMO=false`.

### 5. Alan adı

Railway → **Settings → Networking → Custom Domain**:

- `karecik.com` → pazarlama sayfası ve panel
- `*.karecik.com` → **wildcard, asıl önemli olan bu.** Müşteri menüleri
  `{isletme}.karecik.com/{menu}` adresinden açılıyor; wildcard olmadan hiçbir
  müşteri menüsü çözülmez.

DNS'te Railway'in verdiği hedefe iki CNAME:

```
@   CNAME  <railway-hedefi>
*   CNAME  <railway-hedefi>
```

> Wildcard domain Railway'in ücretli planlarında var. Plan seçerken buna dikkat:
> ürünün tamamı bu alt alan adı modeline dayanıyor.

---

## Sonrası: her push

`main`'e her push Railway'de yeni bir build tetikliyor. Migration'lar açılışta
kendiliğinden uygulanıyor (`database.Migrate`), yani ayrı bir migration adımı yok.

GitHub Actions (`.github/workflows/ci.yml`) aynı anda şunları çalıştırıyor:

- `go vet` + `go test` (gerçek bir PostgreSQL servis konteyneriyle)
- `npm run build`
- Docker imajının gerçekten derlendiğini doğrulama

**Bir ayrıntı:** çok kiracılı izolasyon testleri veritabanı bulamazsa kendini
`t.Skip` ediyor — yani veritabanı olmadan CI yeşil yanar ve hiçbir şey kanıtlamaz.
Workflow bu yüzden çıktıda `--- SKIP` görürse build'i bilerek kırıyor. Atlanmış
bir güvenlik testi, olmayan testten daha kötüdür.

### Actions deploy'u engellesin mi?

Railway ve Actions varsayılan olarak **paralel** çalışıyor: testler kırmızı olsa
bile deploy devam eder. İki seçenek:

1. **Basit (önerilen başlangıç):** böyle bırak. Kırmızı bir CI'yı görürsün,
   Railway'den tek tıkla önceki deploy'a dönersin.
2. **Sıkı:** Railway'de otomatik deploy'u kapat ve `railway up`'ı Actions'ın
   sonuna ekle (`RAILWAY_TOKEN` secret'ı ile). Testler geçmeden deploy olmaz,
   karşılığında bir secret yönetmen gerekir.

---

## Bilerek yapılmış üç seçim

**`HOST=0.0.0.0` sadece konteynerde.** Lokal varsayılan `127.0.0.1` ve öyle
kalmalı — Windows Firewall'un her yeniden derlemede izin sorması bu yüzden
bitti. Ama konteynerde loopback'e bağlanmak uygulamayı platformun router'ından
erişilemez yapar ve **her health check başarısız olur**. Dockerfile bunu
override ediyor; ikisi de doğru, ikisi de kendi yerinde.

**`SEED_DEMO=false` ve production'da seed'in reddi.** Seed gerçek bir işletme
için çalışan bir giriş yazıyor (`melly@karecik.com` / `melly1234`). `SeedDemo`
zaten `APP_ENV=production` görürse hiçbir şey yapmadan çıkıyor; Dockerfile
ayrıca bayrağı da kapatıyor. Production'a örnek veri sızmasın diye iki kat kilit.

**Migration'lar açılışta.** Ayrı bir migration servisi yok. Tek replikada bu
doğru ve en ucuz; replika sayısını artırırsan iki konteyner aynı anda migration
denemesin diye advisory lock gerekir.

---

## Sorun giderme

| Belirti | Sebep |
|---|---|
| Health check timeout | `HOST` `0.0.0.0` değil, ya da `PORT` override edilmiş |
| Açılışta `JWT_SECRET is required in production` | Değişken tanımlı değil |
| Deploy sonrası görseller kırık | `/data` volume'ü yok; yüklemeler imajda kalmış ve silinmiş |
| `{isletme}.karecik.com` 404 | Wildcard CNAME yok, ya da `APP_DOMAIN` yanlış |
| Menü açılıyor ama panel boş | Veritabanı boş — bu normal, kayıt olup ilk menünü oluştur |

Loglar: `railway logs`, veya Railway → servis → **Deployments → View Logs**.
