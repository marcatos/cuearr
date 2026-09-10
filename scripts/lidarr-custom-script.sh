#!/usr/bin/env bash
set -euo pipefail
: "${CUEARR_URL:?}"
: "${CUEARR_API_KEY:?}"

first_added_track_path() {
  local raw="${1:-}"
  raw="${raw#"${raw%%[![:space:]]*}"}"
  raw="${raw%"${raw##*[![:space:]]}"}"
  [[ -z "$raw" ]] && return 0
  if command -v jq >/dev/null 2>&1 && [[ "$raw" == \[* ]]; then
    jq -r '.[0] // empty' <<<"$raw"
    return 0
  fi
  local IFS='|'
  read -r first _ <<<"$raw"
  first="${first#"${first%%[![:space:]]*}"}"
  first="${first%"${first##*[![:space:]]}"}"
  printf '%s' "$first"
}

PATH_TO_SCAN=""
if [[ -n "${Lidarr_AddedTrackPaths:-}" ]]; then
  PATH_TO_SCAN="$(first_added_track_path "$Lidarr_AddedTrackPaths")"
fi
if [[ -z "$PATH_TO_SCAN" && -n "${lidarr_trackfile_path:-}" ]]; then
  PATH_TO_SCAN="${lidarr_trackfile_path}"
fi
if [[ -z "$PATH_TO_SCAN" && -n "${lidarr_release_path:-}" ]]; then
  PATH_TO_SCAN="${lidarr_release_path}"
fi
if [[ -z "$PATH_TO_SCAN" ]]; then
  PATH_TO_SCAN="${1:-}"
fi
if [[ -z "$PATH_TO_SCAN" ]]; then
  echo "cuearr lidarr script: no path (set Lidarr_AddedTrackPaths, lidarr_trackfile_path, or pass path arg)" >&2
  exit 1
fi

if command -v jq >/dev/null 2>&1; then
  payload="$(jq -n --arg path "$PATH_TO_SCAN" '{path: $path}')"
elif command -v python3 >/dev/null 2>&1; then
  payload="$(python3 -c 'import json, sys; print(json.dumps({"path": sys.argv[1]}))' "$PATH_TO_SCAN")"
else
  echo "cuearr lidarr script: need jq or python3 to POST JSON safely" >&2
  exit 1
fi

curl -fsS -X POST "$CUEARR_URL/api/v1/hooks/lidarr" \
  -H "X-Api-Key: $CUEARR_API_KEY" \
  -H "Content-Type: application/json" \
  -d "$payload"
