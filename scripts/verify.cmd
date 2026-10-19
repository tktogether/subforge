@echo off
setlocal

echo === SubForge verify (Windows) ===

rem Frontend runner: prefer pnpm; fall back to web\node_modules\.bin when pnpm.exe is absent.
set "HAVE_PNPM="
where pnpm >nul 2>nul
if not errorlevel 1 set "HAVE_PNPM=1"

echo [1/10] go build ./...
go build ./...
if errorlevel 1 exit /b 1

echo [2/10] go vet ./...
go vet ./...
if errorlevel 1 exit /b 1

echo [3/10] go test ./... -count=1
go test ./... -count=1
if errorlevel 1 exit /b 1

echo [4/10] go test -coverprofile coverage.out -covermode=atomic ./internal/... -count=1
go test -coverprofile coverage.out -covermode=atomic ./internal/... -count=1
if errorlevel 1 exit /b 1

echo [5/10] coveragegate (-min 80 -min-func 100)
go run ./internal/tools/coveragegate -profile coverage.out -min 80 -min-func 100
if errorlevel 1 exit /b 1

echo [6/10] condgate (profile, -min 80)
go run ./internal/tools/condgate -engine=profile -profile coverage.out -min 80
if errorlevel 1 exit /b 1

echo [7/10] frontend: check web\node_modules
if not exist web\node_modules (
  echo ERROR: web\node_modules not found. Run: cd web ^&^& pnpm install --frozen-lockfile
  exit /b 1
)

echo [8/10] frontend typecheck
if defined HAVE_PNPM (
  pnpm -C web run typecheck
) else (
  cmd /c "cd web && node_modules\.bin\vue-tsc.cmd --noEmit"
)
if errorlevel 1 exit /b 1

echo [9/10] frontend coverage gate (vitest run --coverage; thresholds in web/vitest.config.ts)
if defined HAVE_PNPM (
  pnpm -C web run coverage
) else (
  cmd /c "cd web && node_modules\.bin\vitest.cmd run --coverage"
)
if errorlevel 1 exit /b 1

echo [10/10] frontend build
if defined HAVE_PNPM (
  pnpm -C web run build
) else (
  cmd /c "cd web && node_modules\.bin\vue-tsc.cmd --noEmit && node_modules\.bin\vite.cmd build && node scripts\ensure-dist-placeholder.mjs"
)
if errorlevel 1 exit /b 1

echo === verify: all green ===
exit /b 0
