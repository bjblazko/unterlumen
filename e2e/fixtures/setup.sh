#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EXAMPLES="$(cd "$SCRIPT_DIR/../../src/examples" && pwd)"
OUT="$SCRIPT_DIR"

echo "Copying example images to $OUT/photos..."

rm -rf "$OUT/photos"
cp -r "$EXAMPLES" "$OUT/photos"
# The photos' licence note is not a test photo.
rm -f "$OUT/photos/LICENSE"

# Add one image directly inside folder-a to create a mixed dir (subdirs + image at same level)
cp "$OUT/photos/folder-b/2018-10-20_17-46-50_Canon EOS 500D_IMG_3826.jpeg" \
   "$OUT/photos/folder-a/folder-a-sample.jpeg"

mkdir -p "$OUT/.unterlumen-test"

echo "Done."
echo "  photos/folder-a: $(find "$OUT/photos/folder-a" -maxdepth 1 -type f | wc -l | tr -d ' ') file(s), $(find "$OUT/photos/folder-a" -maxdepth 1 -type d | tail -n +2 | wc -l | tr -d ' ') subdir(s)"
echo "  photos/folder-b: $(find "$OUT/photos/folder-b" -maxdepth 1 -type f | wc -l | tr -d ' ') file(s)"

# People with real face recognition (face-recognition.spec.js): the photos
# of ../faces, served by a server of their own. Its data folder starts
# empty but for recognition/, ONNX Runtime and the models, which are
# downloaded once and kept.
echo "Copying face photos to $OUT/faces-photos..."
rm -rf "$OUT/faces-photos"
cp -r "$SCRIPT_DIR/../faces" "$OUT/faces-photos"
rm -f "$OUT/faces-photos/ABOUT.md" "$OUT/faces-photos/LICENSE"
mkdir -p "$OUT/.unterlumen-faces"
find "$OUT/.unterlumen-faces" -mindepth 1 -maxdepth 1 ! -name recognition -exec rm -rf {} +
rm -rf "$OUT/.unterlumen-faces-cache"
echo "  faces-photos: $(find "$OUT/faces-photos" -type f -name '*.jpg' | wc -l | tr -d ' ') photo(s)"
