# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.27-alpine AS builder

WORKDIR /build/src

ARG VERSION=dev

# Cache dependencies before copying source
COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w -X main.Version=${VERSION}" -o /unterlumen .

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM debian:trixie-slim

# Install external tools bundled in the image:
#   ffmpeg            — HEIF/HEIC embedded preview extraction, WebP export (built with libwebp)
#   libheif-examples  — heif-convert; primary HEIC decoder on Linux; handles Fujifilm HEIC
#                       files that have no embedded JPEG stream ffmpeg can probe.
#   libheif-plugin-libde265 — libheif's HEVC decoder. Since libheif 1.16 (trixie) decoders
#                       are plugins of their own; without it heif-convert reports
#                       "Unsupported codec" for every HEIC file.
#   webp              — cwebp; fallback WebP encoder used when ffmpeg lacks libwebp (rare on
#                       Debian, but present in minimal/custom ffmpeg builds or arm64 variants)
#   exiftool          — GPS metadata editing and EXIF stripping on export
#   ca-certificates   — TLS roots (for future HTTPS or CDN map tiles)
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ffmpeg \
        libheif-examples \
        libheif-plugin-libde265 \
        webp \
        libimage-exiftool-perl \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Non-root user
RUN useradd -u 1000 -m unterlumen
USER unterlumen

COPY --from=builder /unterlumen /unterlumen

# Defaults suitable for container use:
#   UNTERLUMEN_BIND=0.0.0.0      — listen on all interfaces
#   UNTERLUMEN_PORT=8080         — standard port
#   UNTERLUMEN_ROOT_PATH=/photos — server mode, navigation locked to mount
#   UNTERLUMEN_CACHE_DIR=/cache  — persistent cache volume; mount /cache for reuse across restarts
ENV UNTERLUMEN_BIND=0.0.0.0 \
    UNTERLUMEN_PORT=8080 \
    UNTERLUMEN_ROOT_PATH=/photos \
    UNTERLUMEN_CACHE_DIR=/cache

VOLUME /photos
VOLUME /cache
EXPOSE 8080

ENTRYPOINT ["/unterlumen"]
