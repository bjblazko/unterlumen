# ADR-0057: Requests from other sites, and what comes from outside

*Last modified: 2026-10-24*

## Status

Accepted.

## Context

The owner asked for a security review on 2026-10-24, guided by the OWASP
Top 10 and paying particular attention to input from people. The review
combined `govulncheck`, gosec (through `golangci-lint`), `npm audit` and
`gitleaks` with a reading of every place where outside input meets HTML, SQL,
a path or a program.

Unterlumen has no login (Your data says so). The installed app listens on
`localhost`, and every page its owner opens in the same browser can reach it.
Photos, sidecars, file names and shared configuration come from other
programs and other people.

The review found:
- **Requests from other sites.** Any web page could `POST` to the app
  (cross-site request forgery) and delete, move or publish. A name of the
  attacker's resolving to `127.0.0.1` would have made their page same-origin
  (DNS rebinding).
- **Command injection.** The user name, host and key file of a destination
  became arguments of `ssh` and `rsync` unchecked. A user `-oProxyCommand=…`
  would have run a command. The configuration they come from is shared
  between installations.
- **Served files.** `/api/image` served any file under the root with the type
  the browser guessed, so an `.html` or `.svg` among the photos would have run
  as a page of the app.
- **Unescaped values in HTML.** File names in the deletion list, the export
  dialog and the viewer; camera and lens names from EXIF in the Statistics
  tooltips; error messages from the server. Three local escape helpers did not
  escape quotes.
- **No security headers and no read timeout.**
- **A known vulnerability in Go 1.27.2's `crypto/x509`** (GO-2026-6809).

SQL was clean: values go through placeholders, and identifiers come from code.
Paths go through `pathguard`, which resolves symlinks.

## Decision

1. **`guard` (`src/guard.go`) wraps the whole app.**
   - `http.NewCrossOriginProtection` refuses requests that change something
     when they come from another origin (`Sec-Fetch-Site`, `Origin`).
     Requests without either, as from `curl` or the tests, pass.
   - When the server listens only on this computer, it answers only to this
     computer's names (`localhost`, `*.localhost`, loopback addresses).
     Served on a network, as on a NAS behind a proxy, the names are the
     proxy's, so this check does not apply there.
2. **Every response carries security headers.**
   - A Content-Security-Policy:
     - scripts, styles and fonts only from the app;
     - images and connections also from OpenFreeMap;
     - workers from `blob:`;
     - no framing by other sites.
   - `X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN` and
     `Referrer-Policy: same-origin`.
   - Inline scripts stay allowed (`'unsafe-inline'`): `index.html` imports
     MapLibre in one, and thumbnails fall back in `onerror`. Removing them is
     debt for later.
3. **Files that are not images are only downloaded.** `/api/image` shows JPEG,
   PNG, GIF, WebP and HEIF inline. Anything else is sent as an attachment.
4. **Arguments of external programs are checked where they are made.**
   `deploy.TargetFromConfig` accepts:
   - a host name or address;
   - a user name of letters, digits, `.`, `_` and `-`, not starting with
     `-`;
   - a key file that does not start with `-` and holds no spaces or quotes.

   Remote shell commands were already quoted.
5. **One escape helper.**
   - `escapeHtml` escapes `& < > " '`; the local copies are gone.
   - Every value from a file, a name, EXIF/XMP or a server message is escaped
     where it enters HTML.
   - A value passed into `t()` for HTML is escaped by the caller.
6. **The server sets `ReadHeaderTimeout` (10 s).** There is no write timeout,
   because status and scan streams stay open.
7. **Go 1.27.3** (`go.mod`).

## Consequences

- **What it guards against:** another site can no longer act through an open
  Unterlumen. A destination's configuration from the shared folder can no
  longer run a program.
- **Tests:**
  - `TestGuardRefusesRequestsFromOtherSites` and
    `TestGuardSetsSecurityHeaders` (main package);
  - `TestTargetFromConfigRefusesWordsReadAsOptions` (deploy);
  - the e2e suite runs with the CSP in place.
- **A new host in the interface needs a line in the CSP**, as it needs one in
  Your data.
- **Later:**
  - removing the inline scripts, so the CSP can drop `'unsafe-inline'`;
  - showing server error messages in the interface's language.
