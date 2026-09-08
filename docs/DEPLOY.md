# Karecik — deploy ve CI/CD

Frontend Cloudflare Pages'te, API Railway'de. Bu doküman **neden** böyle
olduğunu da anlatıyor, çünkü ayrı origin'e geçmenin sessizce yanlış gidebilecek
birkaç noktası var.

---

## Mimari

```
GitHub (main) ──push──▶ GitHub Actions (test)     ← ücretsiz, deploy'u engellemez
       │
       ├──────────────▶ Cloudflare Pages          ← React SPA, ücretsiz
       │                  karecik.com
       │                  *.karecik.com
       │
       └──────────────▶ Railway                   ← Go API
                          api.karecik.com
                          └── Postgres
```

Önceki kurulum tek konteynerdi: Go binary'si React bundle'ını da servis
ediyordu. Ayırmanın karşılığı şu: statik dosyalar Cloudflare'in ücretsiz
katmanında ve dünya genelinde CDN'de, Railway konteyneri de sadece API
çalıştırdığı için daha küçük. Bedeli, aşağıdaki bütün bölümün konusu olan **iki
farklı origin** ve aralarındaki oturum çerezi.

---

## Kimlik doğrulama: bellek içi oturumlar

JWT kaldırıldı. Oturum artık tarayıcıda HttpOnly bir çerez, sunucuda ise **API
sürecinin kendi belleğinde** bir kayıt (`backend/internal/session`). Veritabanı
oturum yolunda hiç yok.

Neden JWT'den vazgeçtik — konfigürasyonun yarısı bundan geliyor:

- **JWT iptal edilemiyordu.** Çıkış yapmak token'ı localStorage'dan silmekti;
  daha önce kopyalanmış bir token süresi dolana kadar çalışmaya devam ederdi.
  Kaydı silmek oturumu gerçekten bitiriyor — "diğer cihazlardan çıkış yap" ve
  "şifre değişince öteki oturumları kapat" ancak böyle gerçek oluyor.
- **localStorage'ı her script okuyabiliyordu.** HttpOnly çerezi JavaScript'ten
  okunamıyor, yani bir XSS açığı artık oturumu çalamıyor.
- **Ham token hiçbir yerde tutulmuyor.** Saklanan şey SHA-256 özeti; ne bir
  veritabanı yedeği ne de bir bellek dökümü kullanılabilir kimlik bilgisi verir.

### Belleğe taşımanın üç sonucu — üçü de operasyonel

**1. API tek instance çalışmak zorunda.** Yük dengeleyici arkasındaki iki kopya
bu haritayı paylaşmaz: A'da açılan oturum B için yoktur, kullanıcı isteklerinin
kabaca yarısında giriş ekranına düşer. Railway'de servis **1 replika** kalmalı;
ölçeklenmesi gerektiği gün oturumlar paylaşılan bir yere (Redis ya da yeniden
veritabanı) taşınmadan replika sayısı artırılamaz.

**2. Her deploy herkesi çıkış yaptırır.** Süreç yeniden başladığında harita
boştur. `main`'e her push bir deploy tetiklediği için bu nadir bir olay değil —
kullanıcılar "sürekli çıkış atıyor" derse sebebi genelde budur, bir hata değil.
Tercih bilerek yapıldı; API açılışta bunu log'a da yazıyor.

**3. `cmd/resetpw` artık oturum kapatamıyor.** Ayrı bir süreç, çalışan
sunucunun belleğine uzanamaz: şifreyi değiştirir ama o hesapta açık duran
oturumlar açık kalır. Şifre sıfırlama genelde "hesap ele geçirildi" şüphesiyle
yapıldığı için **komuttan sonra API servisini yeniden başlatın** — açık olan
her oturumu, saldırganınki dahil, sonlandıran şey budur. Komut bitince bunu
kendisi de hatırlatıyor.

Panel içinden yapılan şifre değişikliği (`/panel/hesap`) etkilenmedi: onu
işleyen süreç oturumları tutan süreçle aynı, diğer oturumları anında düşürüyor.

### Çerez ayarları — burası kritik

| Kurulum | COOKIE_DOMAIN | COOKIE_SAMESITE | COOKIE_SECURE |
|---|---|---|---|
| Yerel geliştirme (Vite proxy) | boş | `Lax` | `false` |
| **karecik.com + api.karecik.com** | `.karecik.com` | `Lax` | `true` |
| pages.dev + up.railway.app | boş | `None` | `true` |

Ortadaki satır hedeflenen production şekli. `karecik.com` ile
`api.karecik.com` **aynı site** sayılır (aynı tescil edilebilir alan adı), o
yüzden `Lax` yeterli — `None` sadece iki taraf gerçekten farklı sitelerdeyken
gerekiyor.

Üç tuzak:

1. **`SameSite=None` + `Secure=false` kombinasyonu tarayıcı tarafından sessizce
   atılır.** Giriş başarılı görünür, hiçbir çerez saklanmaz. Uygulama bu ikiliyi
   açılışta reddediyor, çünkü belirtisi ("giriş çalışmıyor ama hata yok")
   sebebinden çok uzak.
2. **`COOKIE_DOMAIN`'i cevabı veren host'a ait olmayan bir değere ayarlamak da
   sessizce başarısız olur.** `.karecik.com` yalnızca API gerçekten o alan adı
   altındaysa doğru. Geçici Railway alan adında (`*.up.railway.app`) boş bırak.
3. **`credentials: 'include'` olmadan tarayıcı çerezi ne gönderir ne saklar.**
   Frontend'te ayarlı; API tarafında da CORS `AllowCredentials: true` gerekiyor
   ve bu, `Access-Control-Allow-Origin: *` ile birlikte kullanılamaz — bu yüzden
   izin verilen origin bir fonksiyonla tek tek yansıtılıyor.

---

## Cloudflare Pages (frontend)

1. Cloudflare → **Workers & Pages** → **Create** → **Pages** → GitHub reposunu bağla.
2. Build ayarları:

   | Alan | Değer |
   |---|---|
   | Framework preset | None |
   | Build command | `npm ci && npm run build` |
   | Build output directory | `dist` |
   | Root directory | `frontend` |

3. Environment variables (Production **ve** Preview):

   | Değişken | Değer |
   |---|---|
   | `VITE_API_URL` | `https://api.karecik.com` |
   | `VITE_APP_DOMAIN` | `karecik.com` |
   | `VITE_DEMO_BUSINESS` | *(opsiyonel)* landing sayfasındaki telefonda gösterilecek kiracı slug'ı |

   Bunlar **build zamanında** bundle'a gömülüyor: değiştirince yeniden build
   gerekiyor, yeniden başlatmak yetmiyor.

4. Alan adları: `karecik.com` ve **`*.karecik.com`**. Wildcard şart — müşteri
   menüleri `{isletme}.karecik.com/{menu}` adresinden açılıyor.

`frontend/public/_redirects` içindeki `/* /index.html 200` kuralı SPA
yönlendirmesini ayakta tutuyor; o olmadan ana sayfa dışındaki her adres
Cloudflare'in 404'ünü döner.

`VITE_DEMO_BUSINESS` boşsa landing sayfasındaki telefon canlı menü yerine sabit
bir görsel gösteriyor. Otomatik seed kaldırıldığı için varsayılan bu: var
olmayan bir kiracıya bakan iframe, ürünü satan sayfada "menü bulunamadı" yazardı.

---

## Railway (API + veritabanı)

1. **New Project** → **Deploy from GitHub repo** → bu repo. Kökteki
   `railway.json` ve `Dockerfile` bulunur.
2. Aynı projede **New** → **Database** → **PostgreSQL**.
3. **Volume**, mount yolu `/data` — bunu atlama. Konteyner dosya sistemi her
   deploy'da sıfırlanıyor; yüklenen logolar ve görseller `UPLOAD_DIR` altında
   duruyor ve disk olmadan ilk deploy'da kaybolur.
4. Variables:

   | Değişken | Değer |
   |---|---|
   | `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` — referans olarak yaz, kopyalama |
   | `APP_DOMAIN` | `karecik.com` |
   | `CORS_ORIGINS` | `https://karecik.com` |
   | `COOKIE_DOMAIN` | `.karecik.com` *(kendi alan adına geçtikten sonra)* |
   | `COOKIE_SAMESITE` | `Lax` |

   Dockerfile'ın verdiği ve dokunmana gerek olmayanlar: `HOST=0.0.0.0`,
   `PORT=8080`, `APP_ENV=production`, `COOKIE_SECURE=true`,
   `UPLOAD_DIR=/data/uploads`.

5. Custom domain: `api.karecik.com`.
6. **Replika sayısı 1 kalmalı.** Oturumlar süreç belleğinde; ikinci bir kopya
   birincisinin oturumlarını görmez. Trafik arttığı için ölçeklemek gerekirse
   önce oturumları paylaşılan bir yere taşımak lazım — replikayı artırmak tek
   başına giriş akışını bozar.

Migration'lar açılışta kendiliğinden uygulanıyor; ayrı bir adım yok.

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
token üreteci de aynı açığın kılık değiştirmiş hali — token ya bir yerden
okunabilir olurdu ya da hiç kullanılamazdı.

Bu yüzden sıfırlama, güvenin zaten bulunduğu yerde: veritabanı erişimi olan
kişinin sunucuda çalıştırdığı bir komutta. SMTP geldiğinde e-postalı akış
eklenir, bu komut da acil durum yolu olarak kalır.

Komut şifreyi değiştirir ama **açık oturumları kapatamaz** — oturumlar API
sürecinin belleğinde, komut ise ayrı bir süreç. Sıfırlamadan sonra API
servisini yeniden başlatın.

---

## Her push'ta

`main`'e push → Cloudflare Pages ve Railway paralel build alır. GitHub Actions
aynı anda `go vet`, `go test` (gerçek bir PostgreSQL servis konteyneriyle),
`npm run build` ve API imajının derlendiğini çalıştırır.

Çok kiracılı izolasyon testleri veritabanı bulamazsa kendini atlar — yani CI
yeşil yanıp hiçbir şey kanıtlamayabilirdi. Workflow çıktıda `--- SKIP` görürse
build'i bilerek kırıyor.

---

## Sorun giderme

| Belirti | Sebep |
|---|---|
| Giriş 200 dönüyor ama panele girmiyor | Çerez saklanmadı: `SameSite=None` + `Secure=false`, ya da API host'una ait olmayan bir `COOKIE_DOMAIN` |
| Her istek 401 | Frontend `credentials:'include'` göndermiyor, ya da API'de `AllowCredentials` kapalı |
| Her deploy'dan sonra herkes çıkış yapmış | Beklenen davranış: oturumlar bellekte, süreçle birlikte gidiyor |
| Kullanıcılar rastgele çıkış atıyor (deploy yokken) | API birden fazla replika ile çalışıyor; oturum yalnızca onu açan kopyada var |
| Şifre sıfırlandı ama saldırganın oturumu duruyor | `cmd/resetpw` belleğe uzanamıyor — API'yi yeniden başlatın |
| CORS hatası: wildcard + credentials | `CORS_ORIGINS` tam origin'i içermeli; `*` bu modda geçersiz |
| Ana sayfa dışındaki her adres 404 | `_redirects` dosyası `dist`'e kopyalanmamış (Root directory `frontend` mi?) |
| Health check timeout | `HOST` `0.0.0.0` değil |
| Deploy sonrası görseller kırık | `/data` volume'ü yok |
| `{isletme}.karecik.com` 404 | Cloudflare'de wildcard alan adı yok |
