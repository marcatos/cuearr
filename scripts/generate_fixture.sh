#!/usr/bin/env bash
# Synthetic album (image FLAC + CUE) for local smoke tests and e2e. No copyrighted audio.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-${ROOT}/testdata/e2e_album}"
mkdir -p "$OUT"

CUE="${OUT}/album.cue"
FLAC="${OUT}/album.flac"

cat >"$CUE" <<'EOF'
PERFORMER "Cuearr Test"
TITLE "Fixture Album"
FILE "album.flac" WAVE
  TRACK 01 AUDIO
    TITLE "Track One"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Track Two"
    INDEX 01 00:00:03
EOF

if [[ -f "$FLAC" ]]; then
  echo "Keeping existing $FLAC (delete to regenerate)"
  exit 0
fi

if command -v ffmpeg >/dev/null 2>&1; then
  ffmpeg -hide_banner -loglevel error -y \
    -f lavfi -i anullsrc=r=44100:cl=2 -t 6 \
    -c:a flac "$FLAC"
elif command -v sox >/dev/null 2>&1 && command -v flac >/dev/null 2>&1; then
  WAV="${OUT}/album.wav"
  sox -n -r 44100 -c 2 -b 16 "$WAV" trim 0 6
  flac -f -o "$FLAC" "$WAV"
  rm -f "$WAV"
else
  echo "generate_fixture: need ffmpeg or (sox + flac) to create $FLAC" >&2
  exit 1
fi

echo "Wrote $CUE and $FLAC"
