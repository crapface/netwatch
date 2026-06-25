# Changelog

All notable changes to NetWatch are documented here.
This project bumps the `version` constant in `main.go` on every change.

## [1.0.0] — 2026-06-24
Initial release. Portable Windows port of the LanApp concept, rebuilt to spec.

### Added
- **Network detection & manual override** — auto-detects the active IPv4 adapter
  and subnet on startup; the CIDR field is fully editable (e.g. `10.0.0.0/16`).
- **Thorough TCP-connect port scanner** — scans the entire user-defined subnet for
  the configured ports (default `8080`, `3000`), with a bounded worker pool
  (default 100), per-connect timeout, retry-on-timeout, streamed IP generation
  (handles `/16`+ without exhausting memory), full cancellation, and a responsive
  UI. Discovered hosts show IP, hostname (reverse DNS), open ports, vendor
  (MAC OUI), and a unique ID.
- **Monitoring mode** — a flashing "Start Monitoring" button appears after a scan.
  Monitors only the approved host list at a configurable interval (default 60 s).
  A host that misses all its open ports for two consecutive checks is marked DOWN
  (anti-flap); live 🟢/🔴 status per host.
- **Email alerts** — SMTP (none/STARTTLS/SSL), one alert per DOWN event with
  debounce (re-sends only after recovery), Test Email button, plain-text password
  warning.
- **Bilingual UI (English / Spanish)** — instant runtime switch; all labels,
  columns, dialogs and messages from embedded JSON translation maps.
- **IEEE OUI vendor lookup** — one-click download/parse of `oui.txt` into a local
  `oui_cache.json`, loaded on startup.
- **Site profiles (`.site`)** — save/load the entire state (range, ports, email,
  hosts, monitoring event log, monitor state) and resume monitoring after restart.
- **HTML report** — standalone, embedded-CSS report of hosts + monitoring log,
  opened in the default browser.
- **Logging** — plain-text `app.log` (INFO/WARN/ERROR) next to the executable.

### Engineering
- Single static Go binary (lxn/walk native Win32 GUI), CGO-free, ~10 MB.
- No installer, no runtime, no admin rights, no registry, no `%APPDATA%`.
- Unit + integration tests for scanning, monitoring transitions/debounce,
  profile round-trip, MAC/OUI parsing, and port normalization.
