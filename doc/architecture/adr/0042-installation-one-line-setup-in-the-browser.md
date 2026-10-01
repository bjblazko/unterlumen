# ADR-0042: Installation — one line, setup in the browser, sharing by convention

*Last modified: 2026-10-01*

## Status

Accepted. The photo folder (§3, and §4 for the installed app) is superseded by
[ADR-0047](0047-independent-libraries-shared-per-library.md): the installed app
has none, the setup chooses a shared folder, and libraries are shared one by
one.

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
   only the port (8090). The installed app does the same on its first start
   (`desktop.AdoptOlderSettings`), so an app from the .dmg or the Windows
   setup, installed beside an older one, starts with that one's settings
   instead of the setup.

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
   wins. A disk root (`/`, `C:\`) is never shared from
   (`installation.IsDiskRoot`): no other installation sees it, and the top of
   a macOS system disk cannot be written. A shared folder that is not the
   photo folder's own — a `-channels-dir` taken over from an older
   installation — is kept.

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

6. **The system's folder dialog.** In the installed app (configured by
   config.json, so never in server mode, dev runs or the e2e tests)
   `FolderPicker.open` first asks `/api/folder-dialog`, which opens the
   system's own dialog — `osascript choose folder`, the Windows
   `FolderBrowserDialog`, zenity or kdialog — and returns the absolute path.
   Only a request from the same computer (loopback) may open it; a phone
   that reaches the app over the network gets the dialog in the page. A
   folder outside the photo folder opens the dialog in the page with a
   sentence saying why, instead of being dropped. Added on 2026-09-29 after the
   owner found that New library… wanted a typed path.

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
2. Unsigned download packages built with free tools (decided 2026-09-29):
   one universal `Unterlumen.dmg` (lipo, ad-hoc signed — unsigned arm64 code
   reads as damaged — made by `-macos-bundle`, the same bundle
   `-desktop-install` writes) and `Unterlumen-Setup.exe` made with Inno Setup,
   per user without admin rights. One click through Gatekeeper's or
   SmartScreen's warning remains. A package cannot run the helper-program
   installation, so `internal/toolinstall` does it from the app ("Install
   the missing ones"), with the same sources as `install.sh`; on Linux it
   shows the command, since it needs root. `.github/workflows/packages.yml`
   builds both and the release workflow attaches them under fixed names, so
   `releases/latest/download/Unterlumen.dmg` always is the newest.
   The bundle's executable is the program itself (`CFBundleExecutable`
   `unterlumen`), which opens its window when it runs inside a bundle
   (`desktop.LaunchedAsMacApp`). A launch script in between made macOS check
   the quarantined program a second time after "Open Anyway" and offer only
   "Move to Trash".
3. NAS and containers (2026-09-29): the image sets `UNTERLUMEN_LIB_DIR=/data`
   with `/data` and `/cache` writable for any user, so a container run as the
   photos' owner can use named volumes. In server mode Settings shares
   (`/api/setup/sharing`, `/api/setup/share`; `apisetup.Hooks.Sharing`), so the
   NAS shares first and the Mac's setup finds it. The wizard on huepattl.de
   (`/products/unterlumen-nas.html`) is a page with a script that writes the
   `compose.yml` and the steps per NAS in the browser — no service on the
   server, since nothing in it needs one.
4. Store entries (2026-09-29): only the Portainer template URL,
   `huepattl.de/unterlumen/portainer.json` (format 3, a compose stack from
   `packaging/portainer/compose.yml` with PHOTOS, OWNER and PORT), since it
   is the only one that needs no submission to another project. Unraid,
   CasaOS, Umbrel and TrueNAS are left out on purpose.
