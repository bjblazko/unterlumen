# ADR-0006: No Authentication

*Last modified: 2026-02-21*

## Status

Accepted

## Context

The application serves photos and allows file operations (copy, move) via an HTTP API. Adding authentication would protect against unauthorized access but adds configuration complexity.

## Decision

No authentication is implemented. The server binds to `localhost` by default, making it accessible only from the local machine. A `-bind` flag allows binding to `0.0.0.0` for LAN/remote use, which is an explicit opt-in by the user.

## Consequences

- **Zero configuration** — No usernames, passwords, tokens, or certificates to set up.
- **Safe by default** — Binding to `localhost` means only local processes can reach the server.
- **User responsibility for remote use** — When using `-bind 0.0.0.0`, anyone on the network can browse and move/copy files within the root directory. Users are expected to use network-level controls (firewall, VPN, SSH tunnel) for access control.
- **Path traversal is still enforced** — Regardless of authentication, all API paths are validated to stay within the configured root directory. This is a defense-in-depth measure, not a substitute for access control.
