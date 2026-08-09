set shell := ["powershell", "-Command"]

binary := "jukeboks.exe"
archive_name := "jukeboks-dist"
output_dir := "dist"

default:
    just --list

build:
    go build -o {{binary}} .

clean:
    if (Test-Path {{binary}}) { Remove-Item {{binary}} -Force }
    if (Test-Path {{output_dir}}) { Remove-Item {{output_dir}} -Recurse -Force }

run:
    go run .

test:
    go test ./...

package:
    just clean
    just build
    if (-not (Test-Path {{output_dir}})) { New-Item -ItemType Directory -Path {{output_dir}} -Force | Out-Null }
    Copy-Item {{binary}} {{output_dir}}/ -Force
    Copy-Item webroot {{output_dir}}/ -Recurse -Force
    if (Test-Path "{{output_dir}}/webroot") { Remove-Item "{{output_dir}}/webroot" -Recurse -Force }
    Move-Item "{{output_dir}}/webroot" "{{output_dir}}/webroot" -Force

package-zip:
    just package
    if (Test-Path {{archive_name}}.zip) { Remove-Item {{archive_name}}.zip -Force }
    Compress-Archive -Path {{output_dir}}/* -DestinationPath {{archive_name}}.zip -Force

fmt:
    gofmt -w *.go
