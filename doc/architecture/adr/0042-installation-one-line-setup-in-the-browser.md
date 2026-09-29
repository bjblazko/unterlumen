# ADR-0042: Installation — one line, setup in the browser, sharing by convention

*Last modified: 2026-09-29*

## Status

Accepted.

## Context

Installing Unterlumen took a terminal: remove the quarantine attribute,
make the file executable, run `-desktop-install`, and answer three questions
on standard input. ffmpeg, exiftool, heif-convert and cwebp had to be found
and installed separately. Every setting was baked into the launcher as a flag,
so changing the photo folder meant installing again, and the installer never
wrote `-channels-dir` at all. Two installations that show the same photos
(ADR-0023, ADR-0035) had to be given the same `-channels-dir` by hand.

The owner asked on 2026-09-29 how someone without a terminal habit could
install it: on one computer at least, and ideally on a NAS or in a container,
including the hybrid setup of a NAS and a Mac on one share. Decisions taken
then:

- No code signing (no Apple Developer ID, no Windows certificate).
- The installer fetches the helper programs from their makers or the system's
  package manager; Unterlumen does not redistribute them.
- huepattl.de serves the installers and, in a later stage, a wizard for NAS
  and container setups.
- A second installation finds the first through the shared folder, not by a
  pairing code or a network service.

## Decision

1. **config.json instead of launcher flags.** `internal/installation` keeps
   the photo folder, the data folder (`-lib-dir`), the destinations folder
   (`-channels-dir`) and the port in `config.json` in the user's configuration
   folder (`~/Library/Application Support/Unterlumen`, `%AppData%\Unterlumen`,
   `~/.config/Unterlumen`). The launchers pass only `-desktop`.
   Precedence: flag, then environment, then config.json, then default.
   config.json is read only when neither a folder argument nor
   `UNTERLUMEN_ROOT_PATH` is given, which is exactly how the launcher starts
   the app. Development runs, the e2e tests and the container all name a
   folder, so they never read or write it.

2. **`-desktop-install` asks nothing.** Before it replaces a launcher it
   copies that launcher's flags into config.json once, so an update keeps the
   photo folder, the data folder and a `-channels-dir`. A first install writes
   only the port (8090).

3. **Setup in the browser (`#setup`).** Without a photo folder, the app opens
   on a setup place: the photo folder (chosen with the folder picker over the
   whole disk, `/api/setup/dirs`), whether destinations are shared, the data
   folder under "Data folder", and which helper programs were found. Saving
   builds the new configuration first and saves it only if it starts; the
   server then swaps its handler (`atomic.Pointer[http.Handler]` in
   `src/server.go`), so no restart is needed. Settings links here. A saved
   photo folder that is missing at start (a NAS not mounted) opens the setup
   with a sentence saying so instead of stopping the app.

4. **Sharing by convention.** `.unterlumen-shared` inside the photo folder is
   the shared folder. Without `-channels-dir`, an installation whose photo
   folder contains one uses it — on a desk and in a container alike. The setup
   offers to join a folder that has one, and to start sharing one that does
   not: that makes the folder and copies `channels.json` and the album
   register there unless the shared folder already has its own, which always
   wins.

5. **One-line installers.** `install/install.sh` (macOS, Linux) and
   `install/install.ps1` (Windows) download the newest release, check it
   against `checksums.txt`, install the helper programs, run
   `-desktop-install` and start the app. A file `curl` downloads carries no
   quarantine attribute, so an unsigned app opens without Gatekeeper's
   warning. Helper programs come from Homebrew when present; otherwise, on
   macOS, from their makers' builds (ffmpeg by Martin Riedl, exiftool from
   SourceForge, cwebp from Google) into a `tools` folder beside config.json,
   which the app puts in front of `PATH` at start. On Linux they come from
   apt, dnf or pacman; on Windows from winget, with cwebp into the tools
   folder. Both scripts are attached to every release, and huepattl.de
   forwards `/unterlumen/install` and `/unterlumen/install.ps1` to
   `releases/latest/download/`.

## Consequences

- Nobody needs a terminal on Windows; on macOS and Linux one pasted line.
- Settings are changeable in the app. An installation from before this
  change keeps working: its launcher still passes a folder, so it does not
  read config.json until `-desktop-install` runs again.
- A `.unterlumen-shared` folder in a photo folder is now used without being
  configured. Where both installations already pointed `-channels-dir` there,
  nothing changes.
- The setup lists folders anywhere on the disk. It exists only when the app
  is configured by config.json, which is the installed app bound to
  localhost; the rest of the API already had the same reach there.
- Downloading helper programs depends on third-party URLs. The installers
  are checked in CI after every release (`.github/workflows/install.yml`).

## Stages

1. This ADR: one-line installers, setup in the browser, sharing by convention.
2. Unsigned download packages built with free tools: a `.dmg` with a ready
   `Unterlumen.app`, a Windows setup made with Inno Setup or NSIS. One click
   through Gatekeeper's or SmartScreen's warning remains.
3. A wizard on huepattl.de that writes a `compose.yml`, a Synology Container
   Manager project or a Portainer stack; the image gets `UNTERLUMEN_LIB_DIR`
   and a volume for it; the setup in server mode (sharing only).
4. Store entries: Unraid Community Applications, CasaOS, Umbrel, a Portainer
   template URL.
