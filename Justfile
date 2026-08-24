set shell := ["powershell", "-Command"]

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
    just clean
    if (-not (Test-Path {{output_dir}})) { New-Item -ItemType Directory -Path {{output_dir}} -Force | Out-Null }
    go build -o {{output_dir}}/{{binary}} ./cmd/jukeboks
    Copy-Item -Path webroot -Destination {{output_dir}}/webroot -Recurse -Force

package-zip:
    just package
    if (Test-Path {{archive_name}}.zip) { Remove-Item {{archive_name}}.zip -Force }
    Compress-Archive -Path {{output_dir}}/* -DestinationPath {{archive_name}}.zip -Force

fmt:
    gofmt -w cmd internal
