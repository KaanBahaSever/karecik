@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion
title Karecik - Durduruluyor

REM Windows komutlarini tam yolla cagiriyoruz: Git Bash gibi ortamlar
REM ayni isimde farkli komutlar tasiyabiliyor.
set "NETSTAT=%SystemRoot%\System32\netstat.exe"
set "BUL=%SystemRoot%\System32\findstr.exe"
set "OLDUR=%SystemRoot%\System32\taskkill.exe"
set "BEKLE=%SystemRoot%\System32\ping.exe -n 4 127.0.0.1"

echo.
echo  Karecik sunuculari durduruluyor...
echo.

set /a BULUNAN=0

REM Backend (8080) ve frontend (5173) portlarini tutan surecleri kapat.
REM Porta gore kapatiyoruz: diger node/go uygulamalarina dokunmaz.
for %%P in (8080 5173) do (
    for /f "tokens=5" %%I in ('%NETSTAT% -ano ^| %BUL% ":%%P " ^| %BUL% "LISTENING"') do (
        %OLDUR% /F /PID %%I >nul 2>&1
        if not errorlevel 1 (
            echo  [+] Port %%P kapatildi ^(PID %%I^)
            set /a BULUNAN+=1
        )
    )
)

REM Acilan komut pencerelerini de kapat
%OLDUR% /F /FI "WINDOWTITLE eq Karecik Backend*" >nul 2>&1
%OLDUR% /F /FI "WINDOWTITLE eq Karecik Frontend*" >nul 2>&1

echo.
if !BULUNAN! equ 0 (
    echo  Zaten calisan bir Karecik sunucusu yoktu.
) else (
    echo  Durduruldu.
)
echo.
echo  Not: PostgreSQL servisi calismaya devam eder ^(Windows servisi^).
echo.
%BEKLE% >nul
exit /b 0
