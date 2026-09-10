#!/usr/bin/env bash
set -euo pipefail
: "${CUEARR_URL:?}"
: "${CUEARR_API_KEY:?}"
PATH_TO_SCAN="${lidarr_episodefile_path:-${1:-}}"
curl -fsS -X POST "$CUEARR_URL/api/v1/hooks/lidarr" \
  -H "X-Api-Key: $CUEARR_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"path\":\"$PATH_TO_SCAN\"}"
