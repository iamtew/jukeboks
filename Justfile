set shell := ["powershell.exe", "-NoProfile", "-Command"]

binary := "jukeboks.exe"
archive_name := "jukeboks-dist"
output_dir := "dist"

default:
    just --list

build:
    go build -o {{binary}} ./cmd/jukeboks

clean:
    if (Test-Path {{binary}}) { Remove-Item {{binary}} -Force }
    if (Test-Path {{output_dir}}) { Remove-Item {{output_dir}} -Recurse -Force }

run:
    go run ./cmd/jukeboks

test:
    go test ./...

package:
    if (Test-Path {{output_dir}}) { Remove-Item {{output_dir}} -Recurse -Force }
    New-Item -ItemType Directory -Path {{output_dir}} -Force | Out-Null
    go build -o {{output_dir}}/{{binary}} ./cmd/jukeboks
    if (-not (Test-Path {{output_dir}}/{{binary}})) { Write-Error "go build failed"; exit 1 }
    Copy-Item -Path webroot -Destination {{output_dir}}/webroot -Recurse -Force

verify-package:
    @just package
    just _verify-dist
    if (Test-Path {{archive_name}}.zip) { just _verify-zip }

package-zip:
    just package
    if (Test-Path {{archive_name}}.zip) { Remove-Item {{archive_name}}.zip -Force }
    tar -a -c -f {{archive_name}}.zip -C {{output_dir}} .
    if (-not (Test-Path {{archive_name}}.zip)) { Write-Error "tar failed to create zip"; exit 1 }
    just _verify-dist
    just _verify-zip

_verify-dist:
    if (-not (Test-Path {{output_dir}}/{{binary}})) { Write-Error "missing {{output_dir}}/{{binary}}"; exit 1 }
    if (-not (Test-Path {{output_dir}}/webroot/index.html)) { Write-Error "missing webroot/index.html"; exit 1 }
    if (-not (Test-Path {{output_dir}}/webroot/admin/index.html)) { Write-Error "missing webroot/admin/index.html"; exit 1 }
    if (-not (Test-Path {{output_dir}}/webroot/overlay/index.html)) { Write-Error "missing webroot/overlay/index.html"; exit 1 }

_verify-zip:
    cmd /c "tar -t -f {{archive_name}}.zip | findstr /C:jukeboks.exe >nul || exit /b 1"
    cmd /c "tar -t -f {{archive_name}}.zip | findstr /C:webroot/ >nul || exit /b 1"

fmt:
    gofmt -w cmd internal
