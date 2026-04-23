#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT="$SCRIPT_DIR"

echo "Downloading test fixtures to $OUT..."

# JPEG with GPS EXIF (MIT licensed)
# Source: https://github.com/ianare/exif-samples
curl -fL -o "$OUT/gps-jpeg.jpg" \
  "https://raw.githubusercontent.com/ianare/exif-samples/master/jpg/gps/DSCN0010.jpg"

# Plain JPEG without GPS (MIT licensed)
curl -fL -o "$OUT/no-gps-jpeg.jpg" \
  "https://raw.githubusercontent.com/ianare/exif-samples/master/jpg/Canon_40D.jpg"

# HEIC sample (Apache 2.0 licensed)
# Source: https://github.com/strukturag/libheif
curl -fL -o "$OUT/heic-sample.heic" \
  "https://github.com/strukturag/libheif/raw/master/examples/example.heic"

# Sub-directory for navigation tests
mkdir -p "$OUT/subdir"
cp "$OUT/no-gps-jpeg.jpg" "$OUT/subdir/nested.jpg"

echo "Done."
ls -lh "$OUT"/*.jpg "$OUT"/*.heic "$OUT"/subdir/*.jpg
