# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies before copying source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" -o /unterlumen .

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM debian:bookworm-slim

# Install external tools bundled in the image:
#   ffmpeg         — HEIF/HEIC display and WebP export
#   exiftool       — GPS metadata editing and EXIF stripping on export
#   ca-certificates — TLS roots (for future HTTPS or CDN map tiles)
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ffmpeg \
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
ENV UNTERLUMEN_BIND=0.0.0.0 \
    UNTERLUMEN_PORT=8080 \
    UNTERLUMEN_ROOT_PATH=/photos

VOLUME /photos
EXPOSE 8080

ENTRYPOINT ["/unterlumen"]
