# Karecik — deploy ve CI/CD

Her şey tek konteynerde: Go binary'si hem API'yi çalıştırıyor hem de derlenmiş
React paketini servis ediyor. Bu doküman **neden** böyle olduğunu da anlatıyor,
çünkü sessizce yanlış gidebilecek birkaç nokta var.

---

## Mimari

```
GitHub (main) ──push──▶ GitHub Actions (test)     ← ücretsiz, deploy'u engellemez
       │
       └──────────────▶ Railway
                          ├── tek konteyner
                          │     Go API  +  React dist
                          │     karecik.com
                          │     *.karecik.com
                          └── Postgres
```

Bir dönem frontend Cloudflare Pages'te, API ayrı duruyordu. O kurulum iptal
edildi. Tek konteynerin karşılığı şu: **tek origin**. Oturum çerezi için
cross-site ayarı, `SameSite=None`, ayrı bir CORS listesi — hiçbiri gerekmiyor.
Deploy edilecek tek bir şey var, dolayısıyla frontend ile API'nin sürümlerinin
birbirinden ayrı düşmesi de mümkün değil.

---

## İmaj nasıl kuruluyor (3 aşama)

```
1. node:20-alpine    → npm ci → npm run build → frontend/dist
2. golang:1.22-alpine→ statik Go binary'si
3. alpine:3.20       → /app/karecik  +  /app/frontend/dist
```

Üçüncü aşamada ne Node ne de Go araç zinciri var; sadece binary ve derlenmiş
dosyalar. Her iki aşama da çıktısını `test -f` ile doğruluyor: paket üretilmediyse
ya da `COPY` yolu yanlışsa **build kırılıyor**, imaj production'a ulaşmıyor.

### Yolların uyuşması gereken tek nokta

Sunucu `SERVE_STATIC` açıkken paketi servis ediyor ve `index.html`'i
`STATIC_DIR` altında arıyor. **Göreli** bir `STATIC_DIR`, sürecin çalışma
dizinine göre çözülüyor — yani `WORKDIR`'e:

| `STATIC_DIR` | `WORKDIR /app` ile çözülen yol | Sonuç |
|---|---|---|
| `/app/frontend/dist` | `/app/frontend/dist` | ✅ imajın varsayılanı |
| `frontend/dist` | `/app/frontend/dist` | ✅ |
| `./frontend/dist` | `/app/frontend/dist` | ✅ |
| `../frontend/dist` | `/frontend/dist` | ❌ |

Son satır `backend/.env` içindeki **yerel geliştirme** varsayılanı: binary'yi
`backend/` klasöründen çalıştırdığınızda doğru, konteynerin içinde yanlış. Daha
önce production'da alınan

```json
{"error":"sendfile: file frontend/dist/index.html not found","code":"HTTP_ERROR"}
```

hatası tam olarak buydu — paket imajda hiç yoktu. Dikkat çeken tarafı:
**Railway deploy'u başarılı gösteriyordu.** Health check `/api/health` adresine
bakıyor, o da statik servisten bağımsız çalıştığı için platform hiçbir sorun
görmüyordu.

---

## Build zamanı vs çalışma zamanı — en sık yapılan hata

| | Ne zaman okunur | Nasıl değiştirilir |
|---|---|---|
| `VITE_*` | **Build** sırasında JavaScript'in içine gömülür | Docker build ARG + yeniden build |
| Diğer hepsi | Süreç açılırken | Railway değişkeni + yeniden başlatma |

Bir Dockerfile build'i normalde platformun servis değişkenlerini görmez, **ama
Railway `ARG` ile adı eşleşen değişkeni build'e geçirir.** Yani Dockerfile'da
`ARG X` yazmak, panodaki `X` için bir anahtar açmak demek — eski bir kurulumdan
kalan değişken, build yeşil ve health check yeşil bir şekilde pakete gömülür.

Bu yüzden `VITE_API_URL` **ARG değil**: tek doğru değeri var (boş), o da
Dockerfile'da sabit `ENV` olarak duruyor. Panodan erişilemiyor.

Ayarlanabilir kalan ikisi:

| ARG | Varsayılan | Neden |
|---|---|---|
| `VITE_APP_DOMAIN` | `karecik.com` | Menü adreslerindeki kök alan adı |
| `VITE_DEMO_BUSINESS` | *(boş)* | Landing sayfasındaki telefonda gösterilecek kiracı; boşken sabit görsel çıkar |

`VITE_ROOT_DOMAIN` de bilerek ARG değil — yerel geliştirme ayarı, gerçek bir
alan adı gömmek kiracı çözümlemesine ikinci bir kök alan adı sokardı.

> `VITE_API_URL` ayrı bir API adresine ayarlanırsa **iki şey birden** bozulur.
> Panel her istekte 401 alır (çerez cross-origin gönderilmez, ama giriş 200
> döndüğü için sorun giriş gibi görünmez) ve **müşteri menülerinin hiçbiri
> yüklenmez**: sayfa `{kiracı}.karecik.com`'dan açılıyor, istek başka bir
> origin'e gidiyor ve `isAllowedOrigin` artık `*.karecik.com`'u kabul etmediği
> için tarayıcı isteği CORS'ta durduruyor.

---

## Railway kurulumu

1. **New Project** → **Deploy from GitHub repo** → bu repo. Kökteki
   `railway.json` ve `Dockerfile` bulunur.
2. Aynı projede **New** → **Database** → **PostgreSQL**.
3. **Volume**, mount yolu `/data` — bunu atlama. Konteyner dosya sistemi her
   deploy'da sıfırlanıyor; yüklenen logolar ve görseller `UPLOAD_DIR` altında
   duruyor ve disk olmadan ilk deploy'da kaybolur.

   > ⚠️ **Volume ekliyorsan `RAILWAY_RUN_UID=0` da ekle.** Railway volume'leri
   > **root** olarak mount ediyor, bu imaj ise güvenlik için root olmayan bir
   > kullanıcıyla (uid 10001) çalışıyor. İmajın içindeki `chown`, mount
   > tarafından üzeri örtüldüğü için işe yaramıyor: uygulama açılışta
   > `/data/uploads`'ı oluşturamıyor ve **konteyner hiç ayağa kalkmıyor**.
   > Tuzağın kötü tarafı sırası: volume yokken her şey çalışıyor, "deploy sonrası
   > görseller kayboluyor" diye volume ekliyorsun ve geçici bir sorunu tam bir
   > kesintiye çeviriyorsun. Railway'in kendi çözümü bu değişken.

4. Variables:

   | Değişken | Değer |
   |---|---|
   | `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` — referans olarak yaz, kopyalama |
   | `APP_DOMAIN` | `karecik.com` |
   | `CORS_ORIGINS` | `https://karecik.com` |
   | `RAILWAY_RUN_UID` | `0` — volume varsa **zorunlu** (yukarı bak). Sonradan "artık kalmış" diye silme |

   Dockerfile'ın verdiği ve **dokunmana gerek olmayanlar**: `HOST=0.0.0.0`,
   `PORT=8080`, `APP_ENV=production`, `SERVE_STATIC=true`,
   `STATIC_DIR=/app/frontend/dist`, `UPLOAD_DIR=/data/uploads`,
   `COOKIE_SECURE=true`, `COOKIE_SAMESITE=Lax`, `COOKIE_DOMAIN=""`.

5. Custom domain: `karecik.com` **ve** `*.karecik.com`. Wildcard şart — müşteri
   menüleri `{isletme}.karecik.com/{menu}` adresinden açılıyor.

   Railway her alan adı için bir **CNAME** ve bir **TXT** doğrulama kaydı
   veriyor; TXT eksikse alan adı CNAME doğru olsa bile 404 döner. Wildcard
   ayrıca sertifika için bir `_acme-challenge` CNAME'i istiyor ve o kaydın
   **proxy'lenmemesi** gerekiyor (Cloudflare DNS kullanıyorsanız bulut turuncu
   değil gri olmalı). Apex (`karecik.com`) için sağlayıcınızın CNAME flattening
   ya da ALIAS desteklemesi lazım.

6. **Replika sayısı 1 kalmalı.** Oturumlar süreç belleğinde; ikinci bir kopya
   birincisinin oturumlarını görmez.

### Panodan temizlenecekler

Pano değeri Dockerfile'daki `ENV`'i **ezer**, yani ayrılık döneminden kalan bir
değişken düzeltmeyi sessizce geçersiz kılabilir. Şunlar varsa silin:

| Değişken | Neden |
|---|---|
| **`VITE_*` (hepsi)** | Railway bunları adı eşleşen `ARG`'a geçiriyor, yani panoda kalan bir değer **pakete gömülüyor**. `VITE_API_URL` artık ARG olmadığı için erişilemez, ama `VITE_APP_DOMAIN` ve `VITE_DEMO_BUSINESS` erişilebilir — bilerek ayarlamadıysanız silin |
| `SERVE_STATIC`, `STATIC_DIR` | Artık imajın kendisi doğru değeri veriyor; iki yerde tutmak ileride sessizce bozar. `SERVE_STATIC=false` kalırsa API çalışır, site tamamen ölür |
| `COOKIE_SAMESITE=None` | Ayrılık döneminden kalır ve **sessizce hayatta kalır** — açılıştaki kontrol yalnızca `Secure=false` ile eşleştiğinde hata veriyor. Artık olmayan bir topoloji için CSRF yüzeyi açar |
| `CORS_ORIGINS` içindeki `*.pages.dev` | İptal edilen bir Pages projesinin adı serbest kalır; başkası aynı adı alırsa duran bir yetki olur |
| `COOKIE_DOMAIN=.karecik.com` | Oturumu her kiracının subdomain'ine de gönderir; oradaki hiçbir sayfa oturum istemiyor |
| `HOST`, `PORT`, `APP_ENV`, `COOKIE_SECURE`, `UPLOAD_DIR` | İmajın verdiği değerler doğru; panoda kopyası olması konteyneri bir web formundan bozma imkânı demek |

> ⚠️ **Boşaltmayın, SİLİN.** Konfigürasyon boş bir değeri "tanımsız" sayıp
> **geliştirme varsayılanına** düşüyor. `HOST`'u boşaltmak `127.0.0.1` demek —
> konteyner Railway'in yönlendiricisinden erişilemez olur ve her health check
> başarısız olur. `UPLOAD_DIR`'i boşaltmak `./uploads` demek, yani volume'ün
> dışı: her deploy'da bütün görseller gider. `CORS_ORIGINS`'i boşaltmak da
> `http://localhost:5173` demek — bu yüzden uygulama production'da loopback
> origin'lerini açılışta atıp log'a yazıyor.
>
> **`RAILWAY_RUN_UID` bu listede değil.** O silinmeyecek olan.

---

## Kimlik doğrulama: bellek içi oturumlar

Oturum, tarayıcıda HttpOnly bir çerez; sunucuda ise **API sürecinin kendi
belleğinde** bir kayıt (`backend/internal/session`). Veritabanı oturum yolunda
hiç yok. Ham token hiçbir yerde tutulmuyor — saklanan şey SHA-256 özeti.

Tek origin olduğu için çerez ayarları artık basit:

| Ayar | Değer | Neden |
|---|---|---|
| `COOKIE_SAMESITE` | `Lax` | Sayfa ve API aynı origin; `None` sadece gerçekten farklı siteler için |
| `COOKIE_SECURE` | `true` | Çerez halka açık internetten geçiyor |
| `COOKIE_DOMAIN` | *(boş)* | Host-only. `.karecik.com` yazmak oturumu her kiracının subdomain'ine de gönderirdi |

Belleğe taşımanın üç sonucu — üçü de operasyonel:

1. **Her deploy herkesi çıkış yaptırır.** Süreç yeniden başladığında harita boş.
   `main`'e her push bir deploy tetiklediği için bu nadir değil. Kullanıcılar
   "sürekli çıkış atıyor" derse sebebi genelde budur, bir hata değil.
2. **API tek instance çalışmak zorunda.** İkinci kopya birincisinin oturumlarını
   tanımaz; kullanıcı isteklerinin yarısında giriş ekranına düşer.
3. **`cmd/resetpw` oturum kapatamıyor.** Ayrı bir süreç, çalışan sunucunun
   belleğine uzanamaz. Sıfırlamadan sonra API servisini yeniden başlatın.

---

## Örnek veri ve şifreler

Otomatik seed **kaldırıldı**. Eskiden bir ortam değişkeni sunucuya çalışan bir
giriş yazdırabiliyordu; yanlış ayarlanmış tek bir değişken production'a örnek
hesap koyabilirdi. Artık ikisi de elle çalıştırılan komutlar:

```bash
cd backend
go run ./cmd/seed                 # geliştirme verisi (Melly Coffee)
go run ./cmd/seed -refresh        # seed menülerini kaynaktan yeniden yaz (yıkıcı)
go run ./cmd/resetpw -email sahip@ornek.com
```

`cmd/seed` `APP_ENV=production` görürse çalışmayı reddediyor.

### Şifre sıfırlama neden HTTP ucu değil

"Şifremi unuttum" akışı, şifreyi değiştirmeden önce adresin sahibi olduğunuzu
kanıtlar — ve o kanıt e-postadır. Ortada e-posta gönderimi olmadığı için
kanıtlayacak bir şey yok: kimlik doğrulaması olmayan bir sıfırlama ucu, bir
adresi bilen herkesin hesabı ele geçirmesi demek olurdu. Teslim edilemeyen bir
token üreteci de aynı açığın kılık değiştirmiş hali.

Bu yüzden sıfırlama, güvenin zaten bulunduğu yerde: veritabanı erişimi olan
kişinin sunucuda çalıştırdığı bir komutta. Komut şifreyi değiştirir ama açık
oturumları kapatamaz; sonrasında API'yi yeniden başlatın. SMTP geldiğinde
e-postalı akış eklenir, bu komut da acil durum yolu olarak kalır.

---

## Her push'ta

`main`'e push → Railway build alır. GitHub Actions aynı anda `go vet`,
`go test` (gerçek bir PostgreSQL servis konteyneriyle), oturum deposu için
`go test -race`, `npm run build` ve **imajın gerçekten derlendiğini** çalıştırır.

İmaj job'ı ne Go ne de React job'ının yakalayabileceği şeyleri yakalıyor: bozuk
bir `COPY` yolu, Alpine altında patlayan bir `npm ci`, `STATIC_DIR`'in
gösterdiği yere düşmeyen bir `dist`. Ama **imajı yalnızca derliyor,
çalıştırmıyor** — dolayısıyla ancak konteyner ayağa kalkınca ortaya çıkan
sorunları (volume izinleri gibi) göremez.

Asıl ağ artık health check'te: `SERVE_STATIC` açıkken `/api/health`,
`STATIC_DIR/index.html` yoksa **503** dönüyor ve gövdede baktığı yolu yazıyor.
Bu olayda deploy en baştan sona "başarılı" göründü, çünkü health check bir `/api`
ucuydu ve statik dosyalardan haberi yoktu. Göremediği şey için yeşil yanan bir
health check, hiç olmamasından kötü — çünkü ona inanılıyor.

Çok kiracılı izolasyon testleri veritabanı bulamazsa kendini atlar — yani CI
yeşil yanıp hiçbir şey kanıtlamayabilirdi. Workflow çıktıda `--- SKIP` görürse
build'i bilerek kırıyor.

---

## Sorun giderme

| Belirti | Sebep |
|---|---|
| Konteyner hiç ayağa kalkmıyor, log'da `could not create the upload directory ... permission denied` | Volume root olarak mount edilmiş, imaj root değil → `RAILWAY_RUN_UID=0` ekleyin |
| `/api/health` 503, gövdede `frontend bundle is missing: ...` | `STATIC_DIR` gövdede yazan yeri gösteriyor ama paket orada değil |
| `sendfile: file .../index.html not found` | Paket imajda yok ya da `STATIC_DIR` yanlış yeri gösteriyor (yukarıdaki tabloya bak) |
| Panele hiç giriş yapılamıyor, şifre doğru | Tarayıcıda eski `.karecik.com` çerezi kalmış olabilir. Çerez adı `karecik_sid` olarak değiştirildiği için bu artık olmamalı; olursa site verilerini temizleyin |
| Site açılıyor ama tamamen stilsiz | Tailwind `content` glob'ları `process.cwd()`'ye göre çözülüyor — Node aşaması `frontend/` içinden build almalı |
| Deploy sonrası beyaz ekran, konsolda `Unexpected token '<'` | Tarayıcı eski `index.html`'i tutuyor; yenile. Eksik asset artık 404 dönüyor, HTML değil |
| Her istek 401 | `VITE_API_URL` ayrı bir origin'e ayarlanmış — çerez gönderilmiyor |
| Giriş 200 dönüyor ama panele girmiyor | Çerez saklanmadı: API host'una ait olmayan bir `COOKIE_DOMAIN` |
| Her deploy'dan sonra herkes çıkış yapmış | Beklenen: oturumlar bellekte, süreçle birlikte gidiyor |
| Kullanıcılar rastgele çıkış atıyor (deploy yokken) | Birden fazla replika çalışıyor |
| Şifre sıfırlandı ama saldırganın oturumu duruyor | `cmd/resetpw` belleğe uzanamıyor — API'yi yeniden başlatın |
| Health check timeout | `HOST` `0.0.0.0` değil |
| Deploy sonrası görseller kırık | `/data` volume'ü yok |
| `{isletme}.karecik.com` 404 | Railway'de wildcard alan adı tanımlı değil |
| Railway panosunda `VITE_*` değiştirdim, bir şey olmadı | Beklenen: build zamanı değerler. `ARG` ile geçirip yeniden build alın |
