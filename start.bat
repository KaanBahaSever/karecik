@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion
title Karecik - Launcher
cd /d "%~dp0"
set "PROJECT=%~dp0"

REM Windows commands are called by their full path: environments such as
REM Git Bash may shadow them with different tools of the same name.
set "NETSTAT=%SystemRoot%\System32\netstat.exe"
set "FIND=%SystemRoot%\System32\findstr.exe"
REM Waiting is done with ping; timeout.exe fails when stdin is redirected.
set "WAIT1=%SystemRoot%\System32\ping.exe -n 2 127.0.0.1"
set "WAIT3=%SystemRoot%\System32\ping.exe -n 4 127.0.0.1"
set "WAIT5=%SystemRoot%\System32\ping.exe -n 6 127.0.0.1"

echo.
echo  ==========================================
echo    KARECIK - QR Menu Platform
echo  ==========================================
echo.

REM ---------------------------------------------------------------- Go
where go >nul 2>&1
if errorlevel 1 (
    if exist "C:\Program Files\Go\bin\go.exe" (
        set "PATH=C:\Program Files\Go\bin;!PATH!"
        echo  [+] Go found ^(Program Files^)
    ) else (
        echo  [ERROR] Go is not installed.
        echo          Download: https://go.dev/dl/
        echo.
        pause
        exit /b 1
    )
) else (
    echo  [+] Go ready
)

REM -------------------------------------------------------------- Node.js
where node >nul 2>&1
if errorlevel 1 (
    if exist "C:\Program Files\nodejs\node.exe" (
        set "PATH=C:\Program Files\nodejs;!PATH!"
        echo  [+] Node.js found ^(Program Files^)
    ) else (
        echo  [ERROR] Node.js is not installed.
        echo          Download: https://nodejs.org/en/download
        echo.
        pause
        exit /b 1
    )
) else (
    echo  [+] Node.js ready
)

REM ----------------------------------------------------------- PostgreSQL
%NETSTAT% -an | %FIND% ":5432 " | %FIND% "LISTENING" >nul
if errorlevel 1 (
    echo  [*] PostgreSQL is not running, starting it...
    net start postgresql-18 >nul 2>&1
    if errorlevel 1 net start postgresql-x64-18 >nul 2>&1
    if errorlevel 1 net start postgresql-17 >nul 2>&1
    if errorlevel 1 net start postgresql-x64-16 >nul 2>&1

    %WAIT3% >nul
    %NETSTAT% -an | %FIND% ":5432 " | %FIND% "LISTENING" >nul
    if errorlevel 1 (
        echo.
        echo  [ERROR] PostgreSQL could not be started.
        echo          Windows search ^> "Services" ^> postgresql-18 ^> right click ^> Start
        echo          ^(You may need to run this file as administrator.^)
        echo.
        pause
        exit /b 1
    )
    echo  [+] PostgreSQL started
) else (
    echo  [+] PostgreSQL ready
)

REM ------------------------------------------------------------ .env check
if not exist "%PROJECT%backend\.env" (
    echo  [*] backend\.env is missing, creating it from the example...
    copy /y "%PROJECT%backend\.env.example" "%PROJECT%backend\.env" >nul
    echo.
    echo  ------------------------------------------------------------
    echo   IMPORTANT: in the Notepad window that opens, replace the
    echo   password on this line with your own PostgreSQL password,
    echo   then save and close:
    echo.
    echo     DATABASE_URL=postgres://postgres:YOURPASSWORD@localhost:5432/karecik
    echo  ------------------------------------------------------------
    echo.
    notepad "%PROJECT%backend\.env"
)

REM -------------------------------------------------- npm packages check
if not exist "%PROJECT%frontend\node_modules" (
    echo  [*] Frontend packages are missing, installing ^(one time, ~1 min^)...
    pushd "%PROJECT%frontend"
    call npm install
    popd
    if errorlevel 1 (
        echo  [ERROR] npm install failed.
        pause
        exit /b 1
    )
    echo  [+] Packages installed
) else (
    echo  [+] Frontend packages ready
)

REM ------------------------------------------------ already running check
%NETSTAT% -an | %FIND% ":8080 " | %FIND% "LISTENING" >nul
if not errorlevel 1 (
    echo.
    echo  [*] Karecik is already running. Opening the browser...
    echo      Run stop.bat first if you want to restart it.
    echo.
    start "" "http://localhost:5173"
    %WAIT3% >nul
    exit /b 0
)

REM ------------------------------------------------------ start the servers
echo.
echo  Starting the servers...
start "Karecik Backend" cmd /k "cd /d "%PROJECT%backend" && go run ./cmd/api"
start "Karecik Frontend" cmd /k "cd /d "%PROJECT%frontend" && npm run dev"

REM ------------------------------------------------- wait until they are up
echo  Waiting for them to become ready ^(the first build takes a moment^)...
set /a COUNTER=0

:WAIT_LOOP
%WAIT1% >nul
set /a COUNTER+=1

%NETSTAT% -an | %FIND% ":5173 " | %FIND% "LISTENING" >nul
if errorlevel 1 goto KEEP_WAITING
%NETSTAT% -an | %FIND% ":8080 " | %FIND% "LISTENING" >nul
if not errorlevel 1 goto READY

:KEEP_WAITING
if !COUNTER! lss 90 goto WAIT_LOOP

echo.
echo  [*] The servers did not come up within 90 seconds.
echo      Check the two windows that opened for an error message.
echo      Most common cause: a wrong database password in backend\.env
echo.
pause
exit /b 1

:READY
echo.
echo  ==========================================
echo    READY
echo  ==========================================
echo.
echo    Site          : http://localhost:5173
echo    Demo sign-in  : demo@karecik.com  /  demo1234
echo    Customer menu : http://demo-kafe.localhost:5173
echo.
echo    To stop it: stop.bat
echo.

start "" "http://localhost:5173"
%WAIT5% >nul
exit /b 0
