# Easy installation

*Last modified: 2026-09-29*

## Summary

Unterlumen installs with one pasted line or, later, from a download package,
and is set up in the browser: the photo folder, and whether destinations are
shared with another installation that shows the same photos. No terminal
questions, no quarantine commands, no hunting for ffmpeg and exiftool. See
[ADR-0042](../../architecture/adr/0042-installation-one-line-setup-in-the-browser.md).

## Details

Stage 1 (this document's first part):

- `curl -fsSL https://huepattl.de/unterlumen/install | sh` on macOS and Linux,
  `irm https://huepattl.de/unterlumen/install.ps1 | iex` on Windows.
- The installer checks the download, installs the helper programs and starts
  the app, which opens on `#setup`.
- Settings live in `config.json`; `-desktop-install` asks nothing and keeps an
  older launcher's settings.
- `.unterlumen-shared` in the photo folder is found and used; the setup offers
  to join it or to start sharing.

Later stages: download packages (`.dmg`, Windows setup), a NAS/container
wizard on huepattl.de, NAS store entries.

## Acceptance Criteria

Stage 1:

- [x] `-desktop-install` asks nothing; the launcher passes only `-desktop`
- [x] An older launcher's port, data folder, destinations folder and photo folder are kept in config.json
- [x] Flag > environment > config.json > default; config.json is not read when a folder or `UNTERLUMEN_ROOT_PATH` is given
- [x] A first start opens `#setup`; choosing a folder shows it in Folders without a restart
- [x] A photo folder with `.unterlumen-shared` is offered for joining, and its destinations appear
- [x] Sharing a folder copies `channels.json` and the album register there without overwriting what is shared
- [x] A missing photo folder at start opens the setup with a sentence saying why
- [x] Settings links to the setup
- [x] Helper programs in the tools folder are found by the app
- [x] `install.sh` installs on macOS (with and without Homebrew) and on Debian
- [ ] `install.ps1` tested on Windows (CI job `Installers`)
- [ ] huepattl.de forwards `/unterlumen/install` and `/unterlumen/install.ps1`
- [ ] The install scripts are attached to a release

Stage 1b — download packages:

- [ ] `.dmg` with `Unterlumen.app`, built in CI with free tools
- [ ] Windows setup (Inno Setup or NSIS) with Start menu entry and uninstaller
- [ ] huepattl.de explains the one click through Gatekeeper/SmartScreen

Stage 2 — NAS and containers:

- [ ] Web wizard on huepattl.de writing compose.yml, Synology and Portainer setups
- [ ] Docker image with `UNTERLUMEN_LIB_DIR=/data` and a volume
- [ ] Setup in server mode: sharing only, the photo folder stays fixed

Stage 3 — store entries (Unraid, CasaOS, Umbrel, Portainer template URL)
