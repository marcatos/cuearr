#!/usr/bin/env bash
# Synthetic album (FLAC or PCM WAV image + CUE) for local smoke tests and e2e.
set -euo pipefail

START_SECONDS=$SECONDS
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-${ROOT}/testdata/e2e_album}"
MODE="${2:-flac}"
LOG_LEVEL="${CUEARR_LOG_LEVEL:-INFO}"
log() {
  local level="$1"
  shift
  if [[ "$level" != "INFO" || "$LOG_LEVEL" != "ERROR" ]]; then
    printf '%s generate_fixture: %s\n' "$level" "$*"
  fi
}
trap 'status=$?; if [[ $status -ne 0 ]]; then log ERROR "failed mode=$MODE out=$OUT status=$status total_seconds=$((SECONDS - START_SECONDS))"; fi' EXIT
mkdir -p "$OUT"

CUE="${OUT}/album.cue"
case "$MODE" in
  flac|wav) ;;
  *)
    log ERROR "mode must be flac or wav (got $MODE)" >&2
    exit 2
    ;;
esac
IMAGE="${OUT}/album.${MODE}"
log INFO "start mode=$MODE out=$OUT"

cat >"$CUE" <<EOF
PERFORMER "Cuearr Test"
TITLE "Fixture Album"
FILE "album.${MODE}" WAVE
  TRACK 01 AUDIO
    TITLE "Track One"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Track Two"
    INDEX 01 00:03:00
EOF

if [[ -f "$IMAGE" ]]; then
  log INFO "keeping existing image=$IMAGE duration_seconds=$((SECONDS - START_SECONDS))"
  exit 0
fi

generate_via_sox_flac() {
  local temporary_wav="${OUT}/album.fixture.wav"
  sox -n -r 44100 -c 2 -b 16 "$temporary_wav" trim 0 6
  flac -f -o "$IMAGE" "$temporary_wav"
  rm -f "$temporary_wav"
}

generate_via_ffmpeg_pcm_flac() {
  local temporary_wav="${OUT}/album.fixture.wav"
  ffmpeg -hide_banner -loglevel error -y \
    -f lavfi -i anullsrc=r=44100:cl=2 -t 6 \
    -c:a pcm_s16le "$temporary_wav"
  flac -f -o "$IMAGE" "$temporary_wav"
  rm -f "$temporary_wav"
}

if [[ "$MODE" == "wav" ]] && command -v sox >/dev/null 2>&1; then
  sox -n -r 44100 -c 2 -b 16 "$IMAGE" trim 0 6
elif [[ "$MODE" == "wav" ]] && command -v ffmpeg >/dev/null 2>&1; then
  ffmpeg -hide_banner -loglevel error -y \
    -f lavfi -i anullsrc=r=44100:cl=2 -t 6 \
    -c:a pcm_s16le "$IMAGE"
elif [[ "$MODE" == "flac" ]] && command -v sox >/dev/null 2>&1 && command -v flac >/dev/null 2>&1; then
  generate_via_sox_flac
elif [[ "$MODE" == "flac" ]] && command -v ffmpeg >/dev/null 2>&1 && command -v flac >/dev/null 2>&1; then
  generate_via_ffmpeg_pcm_flac
else
  log ERROR "need sox or ffmpeg (plus flac for FLAC mode) to create $IMAGE" >&2
  exit 1
fi

log INFO "finished cue=$CUE image=$IMAGE files=2 duration_seconds=$((SECONDS - START_SECONDS))"
