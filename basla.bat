@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion
title Karecik - Baslatici
cd /d "%~dp0"
set "PROJE=%~dp0"

REM Windows komutlarini tam yolla cagiriyoruz: Git Bash gibi ortamlar
REM ayni isimde farkli komutlar tasiyabiliyor.
set "NETSTAT=%SystemRoot%\System32\netstat.exe"
set "BUL=%SystemRoot%\System32\findstr.exe"
REM Bekleme icin ping kullaniyoruz; timeout.exe stdin yonlendirilince hata verir.
set "BEKLE1=%SystemRoot%\System32\ping.exe -n 2 127.0.0.1"
set "BEKLE3=%SystemRoot%\System32\ping.exe -n 4 127.0.0.1"
set "BEKLE5=%SystemRoot%\System32\ping.exe -n 6 127.0.0.1"

echo.
echo  ==========================================
echo    KARECIK - QR Menu Platformu
echo  ==========================================
echo.

REM ---------------------------------------------------------------- Go
where go >nul 2>&1
if errorlevel 1 (
    if exist "C:\Program Files\Go\bin\go.exe" (
        set "PATH=C:\Program Files\Go\bin;!PATH!"
        echo  [+] Go bulundu ^(Program Files^)
    ) else (
        echo  [HATA] Go kurulu degil.
        echo         Indir: https://go.dev/dl/
        echo.
        pause
        exit /b 1
    )
) else (
    echo  [+] Go hazir
)

REM -------------------------------------------------------------- Node.js
where node >nul 2>&1
if errorlevel 1 (
    if exist "C:\Program Files\nodejs\node.exe" (
        set "PATH=C:\Program Files\nodejs;!PATH!"
        echo  [+] Node.js bulundu ^(Program Files^)
    ) else (
        echo  [HATA] Node.js kurulu degil.
        echo         Indir: https://nodejs.org/en/download
        echo.
        pause
        exit /b 1
    )
) else (
    echo  [+] Node.js hazir
)

REM ----------------------------------------------------------- PostgreSQL
%NETSTAT% -an | %BUL% ":5432 " | %BUL% "LISTENING" >nul
if errorlevel 1 (
    echo  [*] PostgreSQL calismiyor, baslatiliyor...
    net start postgresql-18 >nul 2>&1
    if errorlevel 1 net start postgresql-x64-18 >nul 2>&1
    if errorlevel 1 net start postgresql-17 >nul 2>&1
    if errorlevel 1 net start postgresql-x64-16 >nul 2>&1

    %BEKLE3% >nul
    %NETSTAT% -an | %BUL% ":5432 " | %BUL% "LISTENING" >nul
    if errorlevel 1 (
        echo.
        echo  [HATA] PostgreSQL baslatilamadi.
        echo         Windows arama ^> "Hizmetler" ^> postgresql-18 ^> sag tik ^> Baslat
        echo         ^(Bu dosyayi yonetici olarak calistirman gerekebilir.^)
        echo.
        pause
        exit /b 1
    )
    echo  [+] PostgreSQL baslatildi
) else (
    echo  [+] PostgreSQL hazir
)

REM ------------------------------------------------------------ .env kontrol
if not exist "%PROJE%backend\.env" (
    echo  [*] backend\.env yok, ornekten olusturuluyor...
    copy /y "%PROJE%backend\.env.example" "%PROJE%backend\.env" >nul
    echo.
    echo  ------------------------------------------------------------
    echo   ONEMLI: Acilan Not Defteri'nde su satirdaki sifreyi kendi
    echo   PostgreSQL sifrenle degistir, kaydet ve kapat:
    echo.
    echo     DATABASE_URL=postgres://postgres:SIFREN@localhost:5432/karecik
    echo  ------------------------------------------------------------
    echo.
    notepad "%PROJE%backend\.env"
)

REM -------------------------------------------------- npm paketleri kontrol
if not exist "%PROJE%frontend\node_modules" (
    echo  [*] Frontend paketleri eksik, kuruluyor ^(bir kereye mahsus, ~1 dk^)...
    pushd "%PROJE%frontend"
    call npm install
    popd
    if errorlevel 1 (
        echo  [HATA] npm install basarisiz oldu.
        pause
        exit /b 1
    )
    echo  [+] Paketler kuruldu
) else (
    echo  [+] Frontend paketleri hazir
)

REM ---------------------------------------------- zaten calisiyor mu kontrol
%NETSTAT% -an | %BUL% ":8080 " | %BUL% "LISTENING" >nul
if not errorlevel 1 (
    echo.
    echo  [*] Karecik zaten calisiyor. Tarayici aciliyor...
    echo      Yeniden baslatmak istersen once durdur.bat calistir.
    echo.
    start "" "http://localhost:5173"
    %BEKLE3% >nul
    exit /b 0
)

REM --------------------------------------------------- sunuculari baslat
echo.
echo  Sunucular baslatiliyor...
start "Karecik Backend" cmd /k "cd /d "%PROJE%backend" && go run ./cmd/api"
start "Karecik Frontend" cmd /k "cd /d "%PROJE%frontend" && npm run dev"

REM ------------------------------------------------ hazir olmasini bekle
echo  Hazir olmasi bekleniyor ^(ilk acilista derleme biraz surer^)...
set /a SAYAC=0

:BEKLEME_DONGUSU
%BEKLE1% >nul
set /a SAYAC+=1

%NETSTAT% -an | %BUL% ":5173 " | %BUL% "LISTENING" >nul
if errorlevel 1 goto DEVAM_ET
%NETSTAT% -an | %BUL% ":8080 " | %BUL% "LISTENING" >nul
if not errorlevel 1 goto HAZIR

:DEVAM_ET
if !SAYAC! lss 90 goto BEKLEME_DONGUSU

echo.
echo  [*] Sunucular 90 saniyede acilmadi.
echo      Acilan iki pencerede hata mesaji var mi kontrol et.
echo      En sik sebep: backend\.env icindeki veritabani sifresi yanlis.
echo.
pause
exit /b 1

:HAZIR
echo.
echo  ==========================================
echo    HAZIR
echo  ==========================================
echo.
echo    Site         : http://localhost:5173
echo    Demo giris   : demo@karecik.com  /  demo1234
echo    Musteri menu : http://demo-kafe.localhost:5173
echo.
echo    Durdurmak icin: durdur.bat
echo.

start "" "http://localhost:5173"
%BEKLE5% >nul
exit /b 0
