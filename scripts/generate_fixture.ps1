# Synthetic album (image FLAC + CUE) for local smoke tests and e2e. No copyrighted audio.
param(
    [string]$OutDir = (Join-Path (Split-Path $PSScriptRoot -Parent) "testdata\e2e_album")
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$cuePath = Join-Path $OutDir "album.cue"
$flacPath = Join-Path $OutDir "album.flac"

@'
PERFORMER "Cuearr Test"
TITLE "Fixture Album"
FILE "album.flac" WAVE
  TRACK 01 AUDIO
    TITLE "Track One"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Track Two"
    INDEX 01 00:03:00
'@ | Set-Content -Path $cuePath -Encoding utf8NoBOM

if (Test-Path $flacPath) {
    Write-Host "Keeping existing $flacPath (delete to regenerate)"
    exit 0
}

$sox = Get-Command sox -ErrorAction SilentlyContinue
$flac = Get-Command flac -ErrorAction SilentlyContinue
$ffmpeg = Get-Command ffmpeg -ErrorAction SilentlyContinue
$wavPath = Join-Path $OutDir "album.wav"

if ($sox -and $flac) {
    & sox -n -r 44100 -c 2 -b 16 $wavPath trim 0 6
    & flac -f -o $flacPath $wavPath
    Remove-Item -Force $wavPath -ErrorAction SilentlyContinue
    Write-Host "Wrote $cuePath and $flacPath"
    exit 0
}

if ($ffmpeg -and $flac) {
    & ffmpeg -hide_banner -loglevel error -y `
        -f lavfi -i "anullsrc=r=44100:cl=2" -t 6 `
        -c:a pcm_s16le $wavPath
    & flac -f -o $flacPath $wavPath
    Remove-Item -Force $wavPath -ErrorAction SilentlyContinue
    Write-Host "Wrote $cuePath and $flacPath"
    exit 0
}

Write-Error "generate_fixture: install sox+flac or ffmpeg+flac to create $flacPath"
