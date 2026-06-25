# NetWatch — Portable LAN Scanner & Monitor

NetWatch discovers hosts on your network that have specific TCP ports open, then
watches them and emails you when one goes down. It is a single, portable Windows
executable — no installer, no runtime, no admin rights, no registry, no
`%APPDATA%`. Everything it writes (log, vendor cache, profiles, reports) lives in
the same folder as the `.exe`.

English | [Español](README.es.md)

---

## 1. Requirements

* Windows 10 or 11 (64-bit). Also runs on Windows 7/8.1.
* Nothing else. The .NET-style runtime headaches don't apply — NetWatch is a
  self-contained native binary.

## 2. How to run

1. Copy `NetWatch.exe` anywhere — a USB stick, `C:\Tools\NetWatch\`, your Desktop.
2. **Double-click `NetWatch.exe`.** That's it.

On first launch Windows SmartScreen may warn that the publisher is unknown
(the binary is unsigned). Click **More info → Run anyway**.

## 3. First-time setup

1. **Network range.** On the **Scanner** tab the active subnet is auto-detected and
   filled in (e.g. `192.168.1.0/24`). You can edit it freely — type any CIDR such
   as `10.0.0.0/16` or `172.16.5.0/24`. Click **Detect** to re-detect.
2. **Ports.** Go to **Settings → Scan ports**. The defaults are `8080` and `3000`.
   Add or remove any ports you like (type a number, click **Add**; select one and
   click **Remove selected**).
3. **Vendor names (optional).** Settings → **Vendor database (IEEE OUI)** →
   **Update OUI Data**. NetWatch downloads the official IEEE OUI list and caches it
   as `oui_cache.json` next to the exe, so MAC vendors show up in the table. This is
   a one-time action (refresh occasionally).
4. **Email (optional).** Settings → **Email notifications (SMTP)**. Enter your SMTP
   server, port, encryption (None / STARTTLS / SSL), username, password, From and
   Recipient. Tick **Enable email alerts**, then click **Test Email** to confirm.
   ⚠ The SMTP password is stored in **plain text** inside the `.site` profile —
   NetWatch warns you about this in the UI.
5. **Language.** Use the dropdown at the top-right to switch between English and
   Español at any time. The whole UI updates instantly.

## 4. Workflow: Scan → Monitor → Save → Report

**Scan.** On the Scanner tab, confirm the range and click **Scan**. A progress bar
fills as NetWatch tries each port on every address in the subnet. There is no time
limit — it is built to be thorough — but you can press **Cancel** at any time and
keep whatever was found so far. The UI never freezes. Discovered hosts appear in the
table with status, IP, hostname, vendor, MAC, open ports and a unique ID.

**Monitor.** When the scan finishes, a large **flashing** *START MONITORING* button
appears below the table. Click it. NetWatch now re-checks only the hosts it just
found, every *N* seconds (default 60, set in Settings). The **Monitor** tab shows a
live 🟢 UP / 🔴 DOWN indicator per host and a running event log. A host is flagged
**DOWN** only after it fails on **all** its open ports for **two consecutive
checks** (this avoids false alarms). When that happens you get one email; you won't
be emailed again until the host recovers and goes down a second time.

**Save the Site.** Settings → give the Site a name → **Save Site**. This writes a
single `.site` file containing the network range, port list, email config, the full
host list, the complete monitoring event log, and whether monitoring was running.

**Generate a report.** Settings → **Generate Report** creates a standalone
`Report_<Site>_<timestamp>.html` (all CSS embedded — one file you can email or
archive) and opens it in your browser. It lists every host and the full UP/DOWN log.

## 5. Load a saved Site and resume

Settings → **Load Site** → pick your `.site` file. NetWatch restores the host list,
the event log and every setting exactly as saved. If the Site was monitoring when
you saved it, NetWatch offers to **resume monitoring** right away — so you can close
the app, reopen it later, and pick up where you left off.

## 6. Settings reference

| Setting | Meaning |
|---|---|
| Scan ports | Ports tested during a scan; a host is "found" if ≥1 is open. |
| Max concurrency | Simultaneous TCP dialers (default 100). Raise for speed, lower for slow links. |
| Connect timeout (ms) | How long to wait for each connection (default 1000). |
| Retries on timeout | Extra attempts when a connection times out (default 1) — thoroughness. |
| Check interval | Seconds between monitoring passes (default 60). |
| Email | SMTP server/port/encryption/credentials/recipient + Test Email. |
| Update OUI Data | Download & cache the IEEE vendor database. |
| Save / Load Site | Persist or restore the entire app state (`.site`). |
| Generate Report | Write & open the standalone HTML report. |

## 7. Files NetWatch creates (all portable, next to the .exe)

* `app.log` — timestamped INFO/WARN/ERROR log.
* `oui_cache.json` — cached IEEE vendor database (after you click Update OUI Data).
* `profiles\` — default folder offered by the Save/Load dialogs.
* `Report_*.html` — generated reports.

Delete any of these freely; NetWatch recreates what it needs.

## 8. Troubleshooting

* **Scan finds nothing.** Confirm the CIDR matches your LAN (check `ipconfig`). Make
  sure you're scanning ports that something actually serves. Corporate networks
  often block host-to-host traffic ("client isolation") — try a port you know is open.
* **Vendor / MAC columns are blank.** MAC addresses come from the OS ARP cache,
  which only knows devices on your **own** layer-2 segment. For routed or remote
  subnets the MAC (and therefore the vendor) is unavailable — this is expected and
  handled gracefully. Also make sure you've run **Update OUI Data**.
* **Email test fails.** Re-check server, port and encryption. Port 587 → STARTTLS,
  port 465 → SSL/TLS, port 25 → None. For providers requiring app passwords (e.g.
  Gmail/Microsoft 365), use an app password, not your normal one. As a last resort
  for self-signed mail servers, tick *Skip TLS certificate verification*.
* **Windows Firewall.** Outbound TCP connects (used for scanning) and SMTP are
  normally allowed without prompts. NetWatch needs no inbound rules.
* **Large subnets.** A `/16` (65k addresses) works but takes a while; the scanner
  streams addresses so it won't exhaust memory. Ranges larger than `/8` are refused.

## 9. Build from source

Requires the **Go toolchain 1.22+** (<https://go.dev/dl/>). Nothing else — the build
is pure Go (`CGO_ENABLED=0`) and the Windows manifest + icon are pre-baked into
`rsrc_windows_amd64.syso`, so a plain `go build` produces a themed, DPI-aware,
non-elevated executable.

```powershell
# Option A — one click
.\build.ps1            # or: build.bat

# Option B — manual
$env:GOOS="windows"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -trimpath -ldflags "-H windowsgui -s -w" -o dist\NetWatch.exe .
```

The portable package lands in `.\dist`. Run the tests with `go test ./internal/...`
(the engine packages are OS-independent and test on any platform).

To regenerate the embedded manifest/icon (only if you edit `winres\`):

```powershell
go install github.com/tc-hib/go-winres@latest
go-winres make --in winres\winres.json --arch amd64 --out rsrc
```

## 10. Technology justification

**Go + [lxn/walk](https://github.com/lxn/walk).** The hard requirements are: a single
double-click-able Windows binary with **zero** runtime install, a native tabbed GUI
(editable fields, a data grid with live per-row status, a flashing button), heavy
concurrent network I/O that never blocks the UI and can be cancelled, and full
portability (no registry/`%APPDATA%`).

Go compiles to one static, dependency-free `.exe` (~10 MB) and its goroutines +
`context` cancellation are an ideal fit for a bounded-concurrency subnet scanner.
`lxn/walk` wraps the native Win32 controls (TabWidget, TableView, ProgressBar) — so
the app looks native, stays light, and crucially is **CGO-free**, meaning no C
compiler and clean cross-compilation. Long operations run in goroutines and marshal
UI updates back via `walk`'s `Synchronize`, keeping the window responsive and
cancellable. The flashing button is a `walk.Composite` whose background brush is
toggled by a 500 ms ticker; live status is a `TableView` cell styler that colors each
row green/red. Localization is embedded JSON applied by re-labeling every control on
the fly, so language switches instantly with no restart. (A .NET/WinForms build could
also meet the spec, but it ships a much larger self-contained bundle and pulls in a C#
toolchain; Go gives a smaller, simpler, single-file result.)

---

NetWatch v1.0.0 · see [CHANGELOG.md](CHANGELOG.md).
