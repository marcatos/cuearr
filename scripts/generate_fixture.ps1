# Synthetic album (FLAC or PCM WAV image + CUE) for local smoke tests and e2e.
param(
    [string]$OutDir = (Join-Path (Split-Path $PSScriptRoot -Parent) "testdata\e2e_album"),
    [ValidateSet("flac", "wav")]
    [string]$Mode = "flac"
)

$ErrorActionPreference = "Stop"
$stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
$logLevel = if ($env:CUEARR_LOG_LEVEL) { $env:CUEARR_LOG_LEVEL.ToUpperInvariant() } else { "INFO" }
function Write-FixtureLog {
    param([string]$Level, [string]$Message)
    if ($Level -ne "INFO" -or $logLevel -ne "ERROR") {
        Write-Host "$Level generate_fixture: $Message"
    }
}
trap {
    Write-FixtureLog "ERROR" "failed mode=$Mode out=$OutDir total_ms=$($stopwatch.ElapsedMilliseconds) error=$($_.Exception.Message)"
    break
}
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$cuePath = Join-Path $OutDir "album.cue"
$imagePath = Join-Path $OutDir "album.$Mode"
Write-FixtureLog "INFO" "start mode=$Mode out=$OutDir"

@"
PERFORMER "Cuearr Test"
TITLE "Fixture Album"
FILE "album.$Mode" WAVE
  TRACK 01 AUDIO
    TITLE "Track One"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Track Two"
    INDEX 01 00:03:00
"@ | Set-Content -Path $cuePath -Encoding utf8NoBOM

if (Test-Path $imagePath) {
    Write-FixtureLog "INFO" "keeping existing image=$imagePath duration_ms=$($stopwatch.ElapsedMilliseconds)"
    exit 0
}

$sox = Get-Command sox -ErrorAction SilentlyContinue
$flac = Get-Command flac -ErrorAction SilentlyContinue
$ffmpeg = Get-Command ffmpeg -ErrorAction SilentlyContinue
$temporaryWavPath = Join-Path $OutDir "album.fixture.wav"

if ($Mode -eq "wav" -and $sox) {
    & sox -n -r 44100 -c 2 -b 16 $imagePath trim 0 6
} elseif ($Mode -eq "wav" -and $ffmpeg) {
    & ffmpeg -hide_banner -loglevel error -y `
        -f lavfi -i "anullsrc=r=44100:cl=2" -t 6 `
        -c:a pcm_s16le $imagePath
} elseif ($Mode -eq "flac" -and $sox -and $flac) {
    & sox -n -r 44100 -c 2 -b 16 $temporaryWavPath trim 0 6
    & flac -f -o $imagePath $temporaryWavPath
    Remove-Item -Force $temporaryWavPath -ErrorAction SilentlyContinue
} elseif ($Mode -eq "flac" -and $ffmpeg -and $flac) {
    & ffmpeg -hide_banner -loglevel error -y `
        -f lavfi -i "anullsrc=r=44100:cl=2" -t 6 `
        -c:a pcm_s16le $temporaryWavPath
    & flac -f -o $imagePath $temporaryWavPath
    Remove-Item -Force $temporaryWavPath -ErrorAction SilentlyContinue
} else {
    throw "need sox or ffmpeg (plus flac for FLAC mode) to create $imagePath"
}

Write-FixtureLog "INFO" "finished cue=$cuePath image=$imagePath files=2 duration_ms=$($stopwatch.ElapsedMilliseconds)"
