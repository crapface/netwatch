# Changelog

All notable changes to NetWatch are documented here.
This project bumps the `version` constant in `main.go` on every change.

## [1.1.2] — 2026-06-25

### Fixed
- **Host identity is now IP-based.** Hosts were keyed by MAC, but a shared MAC
  (e.g. the gateway answering ARP for several IPs) made two hosts collide —
  breaking per-row status updates (rows stuck on "—", wrong UP/DOWN counts) and
  causing edits to land on the wrong row. IDs are now unique per IP.
- **Port-less hosts are now monitored by ping.** A manually-added host with no
  ports is checked via ICMP ping (no admin needed) instead of being skipped, so
  "hosts I know exist" actually report UP/DOWN.
- **Edits apply to a running monitor immediately** — adding, editing or removing a
  host re-seeds the active monitor instead of waiting for a stop/start.

## [1.1.1] — 2026-06-25

### Added
- **Check for Updates** — a toolbar button that asks GitHub for the latest release
  and, if a newer version is out, offers to open the download page. It never
  downloads or runs anything itself.

### Fixed / changed
- **Status column** now fills with a green/red background so UP/DOWN reads clearly
  (previously just a small colored glyph); column widened.
- **Double-click** (or press Enter on) a host row to edit its details — on both the
  Scanner and Monitor tabs.
- **HTML report** is now a light, printer-friendly theme with `@media print` rules
  (repeating header row, no page-breaks mid-row, ink-friendly colors).

## [1.1.0] — 2026-06-25

### Added
- **Prominent toolbar** — Save Site / Load Site / Generate Report / About are now
  always visible at the top of the window (in addition to the Settings tab).
- **Manual hosts** — add a host by hand (IP + ports + label + notes) for devices
  you know exist but the scan didn't surface; edit or remove any host. Manual hosts
  are force-included and survive re-scans.
- **Per-host Label and Notes** columns, editable via the Add/Edit host dialog and
  included in saved Sites and HTML reports.
- **Port labels** — name a port (e.g. `8080 → "Door Controller Web"`); labels show
  in the port list, the host table's Ports column, alerts, and the report. Useful
  for hunting specific access-control hosts.
- **Argentine Spanish (es_AR)** as a third language alongside English and neutral
  Spanish; switch instantly from the toolbar dropdown.
- **About screen** with a professional bio and an embedded image (prefers an
  external `about.jpg` next to the exe if present, else the bundled one).

### Fixed / changed
- **Vendor lookup** — added a "Re-resolve vendors" button and automatic
  re-resolution after an OUI update or Site load, so vendor names populate on
  already-scanned hosts (previously stayed blank until a re-scan). Re-resolution
  also fills in MACs that weren't in the ARP cache during the scan.
- Manual hosts with no ports are shown but never flagged DOWN (nothing to probe).
- Vendor is now user-editable per host.

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
