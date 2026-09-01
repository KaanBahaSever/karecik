@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion
title Karecik - Stopping

REM Windows commands are called by their full path: environments such as
REM Git Bash may shadow them with different tools of the same name.
set "NETSTAT=%SystemRoot%\System32\netstat.exe"
set "FIND=%SystemRoot%\System32\findstr.exe"
set "KILL=%SystemRoot%\System32\taskkill.exe"
set "WAIT=%SystemRoot%\System32\ping.exe -n 4 127.0.0.1"

echo.
echo  Stopping the Karecik servers...
echo.

set /a FOUND=0

REM Kill whatever holds the backend (8080) and frontend (5173) ports.
REM Matching by port leaves other node/go applications untouched.
for %%P in (8080 5173) do (
    for /f "tokens=5" %%I in ('%NETSTAT% -ano ^| %FIND% ":%%P " ^| %FIND% "LISTENING"') do (
        %KILL% /F /PID %%I >nul 2>&1
        if not errorlevel 1 (
            echo  [+] Port %%P closed ^(PID %%I^)
            set /a FOUND+=1
        )
    )
)

REM Close the command windows that were opened as well
%KILL% /F /FI "WINDOWTITLE eq Karecik Backend*" >nul 2>&1
%KILL% /F /FI "WINDOWTITLE eq Karecik Frontend*" >nul 2>&1

echo.
if !FOUND! equ 0 (
    echo  No Karecik server was running.
) else (
    echo  Stopped.
)
echo.
echo  Note: the PostgreSQL service keeps running ^(it is a Windows service^).
echo.
%WAIT% >nul
exit /b 0
