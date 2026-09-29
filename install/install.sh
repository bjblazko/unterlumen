#!/bin/sh
# Installs Unterlumen on macOS or Linux, with the helper programs it uses
# (ffmpeg, exiftool, cwebp, heif-convert), and starts it. Running it again
# updates it. The photo folder is chosen in the app afterwards.
#
#   curl -fsSL https://huepattl.de/unterlumen/install | sh
#
# UNTERLUMEN_VERSION=0.13.0   install that version instead of the newest
# UNTERLUMEN_SKIP_TOOLS=1     leave the helper programs alone
#
# Everything happens in main, so a download cut short runs nothing. POSIX sh
# has no local variables, so each function names its own.

set -eu

REPO="https://github.com/bjblazko/unterlumen"
WEBP_VERSION="1.6.0"

say() { printf '%s\n' "$*"; }
fail() { printf 'Unterlumen could not be installed: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# The newest release, read from where GitHub's "latest" link points.
latest_version() {
    url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$REPO/releases/latest") || fail "GitHub did not answer. Check the internet connection and try again."
    printf '%s\n' "${url##*/v}"
}

# The archive name for this computer, as the release names it.
archive_name() {
    version=$1
    case "$(uname -s)/$(uname -m)" in
        Darwin/arm64) printf 'unterlumen_%s_macos_apple_silicon.tar.gz\n' "$version" ;;
        Darwin/x86_64) printf 'unterlumen_%s_macos_intel.tar.gz\n' "$version" ;;
        Linux/x86_64) printf 'unterlumen_%s_linux_amd64.tar.gz\n' "$version" ;;
        Linux/aarch64 | Linux/arm64) printf 'unterlumen_%s_linux_arm64.tar.gz\n' "$version" ;;
        *) fail "there is no Unterlumen for $(uname -s) on $(uname -m)." ;;
    esac
}

sha256() {
    if have shasum; then shasum -a 256 "$1"; else sha256sum "$1"; fi | cut -d ' ' -f 1
}

# Downloads the release into $1 and checks it against the release's checksums.
download() {
    dir=$1 version=$2 name=$3
    say "Downloading Unterlumen ${version}…"
    curl -fsSL -o "$dir/$name" "$REPO/releases/download/v$version/$name" || fail "the download of $name failed."
    curl -fsSL -o "$dir/checksums.txt" "$REPO/releases/download/v$version/checksums.txt" || fail "the checksums of the release could not be downloaded."
    want=$(grep " $name\$" "$dir/checksums.txt" | cut -d ' ' -f 1)
    [ -n "$want" ] && [ "$want" = "$(sha256 "$dir/$name")" ] || fail "the download is not the file the release lists. Nothing was installed."
    tar -xzf "$dir/$name" -C "$dir"
}

# --- Helper programs ---

tools_macos() {
    if have brew; then
        say "Installing the helper programs with Homebrew…"
        for pair in ffmpeg:ffmpeg exiftool:exiftool cwebp:webp; do
            have "${pair%%:*}" || brew install "${pair#*:}"
        done
        return
    fi
    tools="$HOME/Library/Application Support/Unterlumen/tools"
    mkdir -p "$tools"
    fetched=$(mktemp -d)
    case "$(uname -m)" in arm64) ffarch=arm64 webparch=arm64 ;; *) ffarch=amd64 webparch=x86-64 ;; esac
    if ! have ffmpeg && [ ! -x "$tools/ffmpeg" ]; then
        say "Downloading ffmpeg (HEIF photos)…"
        curl -fsSL -o "$fetched/ffmpeg.zip" "https://ffmpeg.martin-riedl.de/redirect/latest/macos/$ffarch/release/ffmpeg.zip"
        unzip -qo "$fetched/ffmpeg.zip" -d "$tools"
    fi
    if ! have exiftool && [ ! -d "$tools/exiftool" ]; then
        say "Downloading exiftool (metadata, locations, renaming)…"
        v=$(curl -fsSL https://exiftool.org/ver.txt)
        curl -fsSL -o "$fetched/exiftool.tar.gz" "https://sourceforge.net/projects/exiftool/files/Image-ExifTool-$v.tar.gz/download"
        tar -xzf "$fetched/exiftool.tar.gz" -C "$fetched"
        mv "$fetched/Image-ExifTool-$v" "$tools/exiftool"
    fi
    if ! have cwebp && [ ! -x "$tools/cwebp" ]; then
        say "Downloading cwebp (WebP export)…"
        curl -fsSL -o "$fetched/webp.tar.gz" "https://storage.googleapis.com/downloads.webmproject.org/releases/webp/libwebp-$WEBP_VERSION-mac-$webparch.tar.gz"
        tar -xzf "$fetched/webp.tar.gz" -C "$fetched"
        cp "$fetched/libwebp-$WEBP_VERSION-mac-$webparch/bin/cwebp" "$tools/cwebp"
    fi
    rm -rf "$fetched"
}

tools_linux() {
    if have apt-get; then
        cmd="apt-get install -y ffmpeg libimage-exiftool-perl libheif-examples webp"
    elif have dnf; then
        cmd="dnf install -y ffmpeg-free perl-Image-ExifTool libheif-tools libwebp-tools"
    elif have pacman; then
        cmd="pacman -S --needed --noconfirm ffmpeg perl-image-exiftool libheif libwebp"
    else
        say "Install ffmpeg, exiftool, heif-convert and cwebp with your package manager; Unterlumen says in Settings which ones it finds."
        return
    fi
    if [ "$(id -u)" = 0 ]; then
        sh -c "$cmd"
    elif have sudo; then
        say "Installing the helper programs; sudo asks for your password."
        sudo sh -c "$cmd" || say "The helper programs were not installed. Later, run: sudo $cmd"
    else
        say "To install the helper programs, run as root: $cmd"
    fi
}

# --- Start ---

start_app() {
    [ -n "${CI:-}" ] && return
    case "$(uname -s)" in
        Darwin) open "$HOME/Applications/Unterlumen.app" ;;
        *) nohup "$HOME/.local/share/unterlumen/launch.sh" >/dev/null 2>&1 & ;;
    esac
}

main() {
    have curl || fail "curl is needed and was not found."
    version=${UNTERLUMEN_VERSION:-$(latest_version)}
    name=$(archive_name "$version")
    work=$(mktemp -d)
    trap 'rm -rf "$work"' EXIT

    download "$work" "$version" "$name"
    if [ "${UNTERLUMEN_SKIP_TOOLS:-}" != 1 ]; then
        case "$(uname -s)" in Darwin) tools_macos ;; *) tools_linux ;; esac
    fi
    "$work/unterlumen" -desktop-install </dev/null
    start_app
    say "Unterlumen $version is installed. It opens in a window; choose your photo folder there."
}

main "$@"
