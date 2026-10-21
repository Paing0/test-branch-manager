@echo off
REM Build script for Windows
REM Usage: scripts\build.bat

set APP_NAME=test-branch-manager
set CMD=.\cmd\test-branch-manager
set DIST=dist

if not defined VERSION set VERSION=dev

if not exist %DIST% mkdir %DIST%

echo === Building Git Branch Manager v%VERSION% ===
echo.

echo Building Windows (amd64)...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w -X main.version=%VERSION%" -o %DIST%\%APP_NAME%-windows-amd64.exe %CMD%
echo   -^> %DIST%\%APP_NAME%-windows-amd64.exe

echo Building macOS (amd64)...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags "-s -w -X main.version=%VERSION%" -o %DIST%\%APP_NAME%-darwin-amd64 %CMD%
echo   -^> %DIST%\%APP_NAME%-darwin-amd64

echo Building macOS (arm64)...
set GOOS=darwin
set GOARCH=arm64
go build -ldflags "-s -w -X main.version=%VERSION%" -o %DIST%\%APP_NAME%-darwin-arm64 %CMD%
echo   -^> %DIST%\%APP_NAME%-darwin-arm64

echo Building Linux (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-s -w -X main.version=%VERSION%" -o %DIST%\%APP_NAME%-linux-amd64 %CMD%
echo   -^> %DIST%\%APP_NAME%-linux-amd64

echo.
echo === Build complete ===
dir %DIST%
