#!/usr/bin/env bash
# run-podman.sh — Launch Unterlumen in Podman with the same photo libraries
# as the installed desktop app.
#
# Run in your own terminal (not via Claude Code — needs Keychain access for the NAS):
#
#   chmod +x run-podman.sh
#   ./run-podman.sh
#
# Stop:  podman stop unterlumen-dev
# Logs:  podman logs -f unterlumen-dev

set -euo pipefail

NAS_SERVER="192.168.178.148"
NAS_SHARE="nas"
NAS_USER="data"
NAS_MOUNT="$HOME/mnt/nas"            # virtiofs-visible path (under /Users)
PHOTOS_PATH="$NAS_MOUNT/Timo/Bilder/Fotos"
# Mount point inside the container must match the macOS path so that the
# library source_path values stored in the DB (/Volumes/nas/Timo/Bilder/Fotos/…)
# resolve correctly without any path translation.
CONTAINER_PHOTOS_PATH="/Volumes/nas/Timo/Bilder/Fotos"
LIB_DIR="$HOME/Library/Application Support/Unterlumen"
CACHE_DIR="$HOME/Library/Caches/unterlumen"
IMAGE="unterlumen:local"
CONTAINER="unterlumen-dev"
PORT=8091

# ── 1. Ensure Podman machine is running ──────────────────────────────────────
if ! podman machine list --format '{{.Running}}' 2>/dev/null | grep -q true; then
  echo "Starting Podman machine…"
  podman machine start
fi

# ── 2. Mount NAS inside the VM at the virtiofs-mapped path ───────────────────
#
# ~/mnt/nas is visible in the Podman VM at the same absolute path via virtiofs.
# We mount the SMB share ON TOP of that path inside the VM so Podman sees the
# NAS contents when resolving the /Users/… volume path.
#
VM_MOUNT="/Users/blazko/mnt/nas"

if ! podman machine ssh "mountpoint -q '$VM_MOUNT'" 2>/dev/null; then
  echo "Mounting NAS inside Podman VM…"

  # Read NAS password from the macOS Keychain (prompts Touch ID / macOS password).
  NAS_PASS=$(security find-internet-password -s "$NAS_SERVER" -a "$NAS_USER" -w)

  # Write a credentials file and pipe it directly into the VM via stdin to avoid
  # quoting issues with special characters in the password.
  CRED_TMP=$(mktemp /tmp/nas-cred-XXXXXX)
  printf 'username=%s\npassword=%s\n' "$NAS_USER" "$NAS_PASS" > "$CRED_TMP"
  unset NAS_PASS

  # Copy credentials file into VM, mount, and clean up credentials.
  podman machine ssh "sudo tee /tmp/nas-cred > /dev/null" < "$CRED_TMP"
  rm -f "$CRED_TMP"

  podman machine ssh "
    sudo mkdir -p '$VM_MOUNT'
    sudo mount.cifs '//$NAS_SERVER/$NAS_SHARE' '$VM_MOUNT' \
      -o credentials=/tmp/nas-cred,uid=0,gid=0,file_mode=0644,dir_mode=0755,ro
    sudo rm -f /tmp/nas-cred
  "

  if ! podman machine ssh "mountpoint -q '$VM_MOUNT'" 2>/dev/null; then
    echo "ERROR: NAS mount failed inside the VM. Check NAS connectivity and credentials."
    exit 1
  fi
  echo "NAS mounted at $VM_MOUNT in VM."
fi

# ── 3. Ensure local anchor dirs exist ────────────────────────────────────────
mkdir -p "$NAS_MOUNT"
mkdir -p "$CACHE_DIR"

# ── 4. Remove stale container and start fresh ────────────────────────────────
podman rm -f "$CONTAINER" 2>/dev/null || true

echo "Starting $IMAGE on http://localhost:$PORT …"
EXTRA_MOUNTS=()
# Mount any extra local photo paths used by libraries outside the NAS tree.
# Add entries here if you have libraries pointing to local folders.
EXAMPLES_DIR="$HOME/Development/unterlumen/src/examples"
[ -d "$EXAMPLES_DIR" ] && EXTRA_MOUNTS+=("-v" "${EXAMPLES_DIR}:${EXAMPLES_DIR}:ro")

podman run -d \
  --name "$CONTAINER" \
  --user "$(id -u):$(id -g)" \
  -p "${PORT}:8080" \
  -v "${PHOTOS_PATH}:${CONTAINER_PHOTOS_PATH}:ro" \
  "${EXTRA_MOUNTS[@]}" \
  -v "${LIB_DIR}:/data" \
  -v "${CACHE_DIR}:/cache" \
  -e UNTERLUMEN_BIND=0.0.0.0 \
  -e UNTERLUMEN_PORT=8080 \
  -e UNTERLUMEN_ROOT_PATH="${CONTAINER_PHOTOS_PATH}" \
  -e UNTERLUMEN_LIB_DIR=/data \
  -e UNTERLUMEN_CACHE_DIR=/cache \
  "$IMAGE"

echo ""
echo "Unterlumen (Podman) → http://localhost:$PORT"
echo "Desktop app          → http://localhost:8090"
echo ""
echo "Logs: podman logs -f $CONTAINER"
echo "Stop: podman stop $CONTAINER"
