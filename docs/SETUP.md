# Karecik — Setup Guide

This guide covers everything needed to get the project running on a machine
where **nothing is installed yet** (no Go, no Node.js, no PostgreSQL).

Total time: roughly 20–30 minutes.

---

## 0. Required software

| Software | Version | Purpose |
|---|---|---|
| Go | 1.22+ | Backend (API server) |
| Node.js | 20 LTS+ | Frontend (React build tooling) |
| PostgreSQL | 15+ | Database |
| Git | any | Version control |

---

## 1. Installing Go

### Windows

1. Go to https://go.dev/dl/.
2. Download **`go1.23.x.windows-amd64.msi`** (the MSI, not the ZIP).
3. Double-click it → Next → Next → Install. Keep the default location
   `C:\Program Files\Go`.
4. **Close every open terminal and open a new one.** The PATH variable is only
   picked up by newly started terminals.
5. Verify:

```powershell
go version
# output: go version go1.23.5 windows/amd64
```

> Getting `go: command not found`? Windows search → "environment variables" →
> *System variables* → `Path` → *Edit* → *New* → add `C:\Program Files\Go\bin`
> → OK. Then reopen the terminal.

### macOS

```bash
# Install Homebrew first if you do not have it:
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

brew install go
go version
```

---

## 2. Installing Node.js

### Windows

1. Go to https://nodejs.org/en/download.
2. Download the **LTS** build (`node-v22.x.x-x64.msi`).
3. Install it. During setup, *leave unchecked* the
   "Automatically install the necessary tools..." box — it is not needed and
   makes the installation much longer.
4. Reopen the terminal and verify:

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

## 3. Installing PostgreSQL

### Windows

1. Go to https://www.postgresql.org/download/windows/ → "Download the installer" (EDB).
2. Download and run `postgresql-18.x-windows-x64.exe`.
3. In the wizard:
   - **Components**: keep `PostgreSQL Server`, `pgAdmin 4` and
     `Command Line Tools` checked. (Leave Stack Builder unchecked.)
   - **Password**: choose a password for the `postgres` user.
     **Write it down** — you will put it in `.env` shortly.
     (This guide assumes `postgres123`.)
   - **Port**: `5432` (the default, leave it alone).
   - **Locale**: `Turkish, Turkey` or `Default locale`, either is fine.
4. After installation, add `psql` to PATH:
   Environment Variables → `Path` → New → `C:\Program Files\PostgreSQL\18\bin`
   (If you installed a different version the folder differs too:
   `...\PostgreSQL\17\bin` and so on.)
5. Reopen the terminal and verify:

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

## 4. Creating the database

Karecik expects a database named `karecik`. It **creates the tables itself**
(migrations run when the backend starts), so you only need the empty database.

### Windows (PowerShell)

```powershell
# Connect as the postgres user (it will ask for the password you chose)
psql -U postgres -h localhost
```

At the `postgres=#` prompt:

```sql
CREATE DATABASE karecik;
\l
\q
```

Or as a one-liner:

```powershell
psql -U postgres -h localhost -c "CREATE DATABASE karecik;"
```

### macOS

```bash
createdb karecik
# or
psql postgres -c "CREATE DATABASE karecik;"
```

### With pgAdmin (if you prefer a GUI)

Open pgAdmin 4 → Servers → PostgreSQL 18 (enter the password) → right-click
**Databases** → *Create* → *Database...* → Database: `karecik` → Save.

---

## 5. Running the backend

```powershell
cd C:\Users\Arda\Desktop\karecik\backend
```

### 5.1 Create the `.env` file

The repository ships `.env.example`. Copy it:

```powershell
# Windows PowerShell
Copy-Item .env.example .env
```

```bash
# macOS / Git Bash
cp .env.example .env
```

Then open `.env` and replace the password in the `DATABASE_URL` line with the
one you chose during the PostgreSQL installation:

```env
DATABASE_URL=postgres://postgres:postgres123@localhost:5432/karecik?sslmode=disable
JWT_SECRET=change-me-to-a-long-random-secret-2026
PORT=8080
APP_DOMAIN=karecik.com
DEV_DOMAIN=localhost
CORS_ORIGINS=http://localhost:5173
UPLOAD_DIR=./uploads
SEED_DEMO=true
```

**Anatomy of the connection string** (`DATABASE_URL`):

```
postgres://USER:PASSWORD@HOST:PORT/DATABASE?sslmode=disable
          └──┬───┘ └──┬───┘ └───┬───┘ └─┬┘ └──┬───┘
        postgres    your     localhost 5432 karecik
                  password
```

> If your password contains `@ : / ? #`, **URL-encode** it:
> `@` → `%40`, `#` → `%23`, `/` → `%2F`.
> For example the password `pa@ss` becomes:
> `postgres://postgres:pa%40ss@localhost:5432/karecik?sslmode=disable`
>
> `sslmode=disable` is for local development. Switch it to `require` when you
> deploy to a server.

### 5.2 Download the Go packages

`go.mod` is ready. Fetch the dependencies with:

```powershell
go mod tidy
```

That downloads:

| Package | Role |
|---|---|
| `github.com/gofiber/fiber/v2` | HTTP framework (router, middleware) |
| `github.com/jackc/pgx/v5` | PostgreSQL driver and connection pool |
| `github.com/golang-jwt/jwt/v5` | JWT issuing and verification |
| `golang.org/x/crypto` | Password hashing with bcrypt |
| `github.com/joho/godotenv` | Reading the `.env` file |
| `github.com/google/uuid` | UUID generation |

An internet connection is required; the packages land in
`C:\Users\<user>\go\pkg\mod`.

### 5.3 Start the server

```powershell
go run ./cmd/api
```

Expected output:

```
[karecik] loaded .env (.env)
[karecik] connected to the database (localhost:5432/karecik)
[karecik] migration applied: 001_init.sql
[karecik] migration applied: 002_brand_color.sql
[karecik] demo menu created -> demo@karecik.com / demo1234 (slug: demo-kafe)
[karecik] server listening   -> http://localhost:8080
```

Test it: open http://localhost:8080/api/health →
`{"status":"ok","database":"up"}`

> **Error: `dial tcp [::1]:5432: connectex: No connection could be made`**
> The PostgreSQL service is not running. Windows search → "Services" →
> `postgresql-18` → right click → Start.
>
> **Error: `password authentication failed for user "postgres"`**
> The password in `.env` is wrong. Check the one you set during installation.
>
> **Error: `database "karecik" does not exist`**
> You skipped step 4: `psql -U postgres -c "CREATE DATABASE karecik;"`

---

## 6. Running the frontend

**Open a second terminal** (leave the backend running in the first one):

```powershell
cd C:\Users\Arda\Desktop\karecik\frontend
npm install
npm run dev
```

`npm install` pulls in `react`, `react-router-dom`, `vite`, `tailwindcss`,
`@dnd-kit/core`, `@dnd-kit/sortable`, `lucide-react` (icons) and `qrcode`.

> **Note:** the landing page contains no animation library at all
> (framer-motion, gsap, aos and friends are deliberately absent).

Output:

```
  VITE v5.4.x  ready in 420 ms
  ➜  Local:   http://localhost:5173/
```

Open http://localhost:5173 in the browser and the landing page appears.

---

## 7. Testing subdomains locally

In production menus are served from `businessname.karecik.com`. Locally there
are three ways to reach the same thing:

### Option A — path based (simplest, needs no configuration)

```
http://localhost:5173/m/demo-kafe
```

### Option B — a real subdomain (Chrome, Edge and Firefox support `*.localhost`)

```
http://demo-kafe.localhost:5173
```

Chrome and Edge resolve `*.localhost` subdomains to `127.0.0.1` automatically,
so **no hosts file entry is required**. Safari does not.

### Option C — the hosts file (for Safari or a custom domain)

Windows: open Notepad **as administrator** → open
`C:\Windows\System32\drivers\etc\hosts` → append:

```
127.0.0.1 karecik.local
127.0.0.1 demo-kafe.karecik.local
```

macOS: add the same lines through `sudo nano /etc/hosts`.

Then put `VITE_ROOT_DOMAIN=karecik.local` in `frontend/.env` and
`DEV_DOMAIN=karecik.local` in `backend/.env`, and open
`http://demo-kafe.karecik.local:5173`.

---

## 8. Demo account

The seeder creates a sample café menu (this is what the iPhone iframe on the
landing page shows):

```
Email: demo@karecik.com
Password: demo1234
Menu: http://localhost:5173/m/demo-kafe
```

---

## 9. Everyday use

```powershell
# Terminal 1 — backend
cd C:\Users\Arda\Desktop\karecik\backend
go run ./cmd/api

# Terminal 2 — frontend
cd C:\Users\Arda\Desktop\karecik\frontend
npm run dev
```

Or simply double-click `start.bat` in the project root, which does both.

PostgreSQL runs in the background as a Windows service, so you never have to
start it yourself.

---

## 10. Going to production

```powershell
# Build the frontend → creates frontend/dist
cd frontend
npm run build

# Build the backend into a single .exe
cd ../backend
go build -o karecik.exe ./cmd/api
```

- Add a wildcard A record in DNS: `*.karecik.com` → the server IP.
- Obtain a wildcard SSL certificate (Let's Encrypt via the DNS-01 challenge for
  `*.karecik.com`).
- `.env`: `APP_DOMAIN=karecik.com`, `SERVE_STATIC=true`,
  `STATIC_DIR=../frontend/dist`, `sslmode=require`, a strong `JWT_SECRET`,
  `SEED_DEMO=false`.
- With `SERVE_STATIC=true` the backend also serves the built frontend, so a
  single binary is enough.
