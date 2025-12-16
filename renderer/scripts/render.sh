#!/usr/bin/env bash
set -euo pipefail

SPEC_PATH="${1:?missing spec path}"

# Helpers Python inline (evita jq)
py() {
  python3 - <<EOF
import json
with open("$SPEC_PATH","r",encoding="utf-8") as f:
    d=json.load(f)
print($1)
EOF
}

JOB_ID=$(py 'd["job_id"]')
TEXT=$(py 'd.get("params",{}).get("text","GALA")')
VIDEO_KEY=$(py 'd["output"]["video_object_key"]')
THUMB_KEY=$(py 'd["output"]["thumb_object_key"]')

ROOT="${STORAGE_LOCAL_ROOT:-/data}"
VIDEO_OUT="${ROOT}/${VIDEO_KEY}"
THUMB_OUT="${ROOT}/${THUMB_KEY}"

mkdir -p "$(dirname "$VIDEO_OUT")"
mkdir -p "$(dirname "$THUMB_OUT")"

echo "Rendering job ${JOB_ID}"
echo "Video: ${VIDEO_OUT}"
echo "Thumb: ${THUMB_OUT}"

# Video vertical 1080x1920, 7s, texto centrado
ffmpeg -y \
  -f lavfi -i color=c=black:s=1080x1920:d=7:r=30 \
  -vf "drawtext=fontcolor=white:fontsize=72:text='${TEXT}':x=(w-text_w)/2:y=(h-text_h)/2" \
  -c:v libx264 -pix_fmt yuv420p -t 7 \
  "${VIDEO_OUT}"

# Thumbnail (primer frame)
ffmpeg -y \
  -i "${VIDEO_OUT}" \
  -frames:v 1 \
  -q:v 2 \
  "${THUMB_OUT}"

echo "Render complete"
