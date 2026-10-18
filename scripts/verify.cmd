@echo off
setlocal

echo === SubForge verify (Windows) ===

echo [1/9] go build ./...
go build ./...
if errorlevel 1 exit /b 1

echo [2/9] go vet ./...
go vet ./...
if errorlevel 1 exit /b 1

echo [3/9] go test ./... -count=1
go test ./... -count=1
if errorlevel 1 exit /b 1

echo [4/9] go test -coverprofile coverage.out -covermode=atomic ./internal/... -count=1
go test -coverprofile coverage.out -covermode=atomic ./internal/... -count=1
if errorlevel 1 exit /b 1

echo [5/9] coveragegate (-min 80 -min-func 100)
go run ./internal/tools/coveragegate -profile coverage.out -min 80 -min-func 100
if errorlevel 1 exit /b 1

echo [6/9] condgate (profile, -min 80)
go run ./internal/tools/condgate -engine=profile -profile coverage.out -min 80
if errorlevel 1 exit /b 1

echo [7/9] frontend: check web/node_modules
if not exist web\node_modules (
  echo ERROR: web/node_modules not found. Run: cd web ^&^& pnpm install --frozen-lockfile
  exit /b 1
)

echo [8/9] pnpm -C web run typecheck
pnpm -C web run typecheck
if errorlevel 1 exit /b 1

echo [8/9] pnpm -C web run test
pnpm -C web run test
if errorlevel 1 exit /b 1

echo [9/9] pnpm -C web run build
pnpm -C web run build
if errorlevel 1 exit /b 1

echo === verify: all green ===
exit /b 0
