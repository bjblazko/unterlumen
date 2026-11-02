#!/bin/sh
# Builds Unterlumen.dmg: one app for Apple Silicon and Intel, with a link to
# Applications to drag it onto. Unsigned apart from an ad-hoc signature, so
# macOS asks once to "Open Anyway" (ADR-0042, stage 1b).
#
#   packaging/macos/build-dmg.sh 0.15.0 dist
#
# Runs on macOS from the repository root; needs Go, lipo, codesign, hdiutil.

set -eu

version=$1
out=$2
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

for arch in arm64 amd64; do
    (cd src && CGO_ENABLED=0 GOOS=darwin GOARCH=$arch go build \
        -ldflags "-s -w -X main.Version=$version" -o "$work/unterlumen-$arch" .)
done
lipo -create -output "$work/unterlumen" "$work/unterlumen-arm64" "$work/unterlumen-amd64"

mkdir -p "$work/dmg"
"$work/unterlumen" -macos-bundle "$work/dmg/Unterlumen.app"
# Without any signature an app on Apple Silicon reads as damaged; an ad-hoc
# one makes it an app from an unidentified developer, which the user can allow.
codesign --force --deep --sign - "$work/dmg/Unterlumen.app"
ln -s /Applications "$work/dmg/Applications"

mkdir -p "$out"
# hdiutil on GitHub's macOS runners fails now and then with "Resource
# busy" (a scanner holding the new image); it is tried again, and what it
# says is shown — the release of 0.23.0 lost its .dmg to a silent failure.
for try in 1 2 3; do
    if hdiutil create -volname Unterlumen -srcfolder "$work/dmg" -ov -format UDZO "$out/Unterlumen.dmg"; then
        break
    fi
    [ "$try" = 3 ] && exit 1
    echo "hdiutil create failed (try $try), trying again" >&2
    sleep $((try * 10))
done
hdiutil verify "$out/Unterlumen.dmg"
echo "$out/Unterlumen.dmg"
