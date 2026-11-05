# tunit

Universal system diagnostic, stress, and HTTP load testing tool — single binary, cross-platform, works in a shop or in CI.

```
tunit                         # interactive menu (recommended for most cases)
tunit qc --auto               # automated hardware diagnostics, no prompts
tunit stress --duration 60s   # CPU/thermal stress test
tunit net                     # ping, DNS, TCP, and bandwidth
tunit load --url https://... --rps 200 --duration 30s
```

---

## Install

**Linux / macOS (one-liner)**
```bash
curl -fsSL https://github.com/tunit/tunit/releases/latest/download/install.sh | bash
```

**Windows (PowerShell)**
```powershell
iex (iwr https://github.com/tunit/tunit/releases/latest/download/tunit.ps1).Content
```

**Manual download** — grab the right binary from [Releases](https://github.com/tunit/tunit/releases):

| Platform | File |
|---|---|
| Linux x86-64 | `tunit-linux-amd64` |
| Linux ARM64  | `tunit-linux-arm64` |
| macOS x86-64 | `tunit-darwin-amd64` |
| macOS Apple Silicon | `tunit-darwin-arm64` |
| Windows x86-64 | `tunit-windows-amd64.exe` |

**Build from source**
```bash
git clone https://github.com/tunit/tunit
cd tunit
go build -o tunit ./cmd/tunit
```
Requires Go 1.23+.

---

## Commands

### `tunit` — interactive menu

Run with no arguments to get the guided menu. Covers all modes below, asks questions, and saves a report file when done.

---

### `tunit qc` — hardware QC checklist

Runs automated hardware checks followed by interactive human-confirmation steps (audio, webcam, keyboard, USB, dead pixels).

```
tunit qc [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--auto` | false | Skip interactive human checks (CI-safe) |
| `--lookup` | false | Query manufacturer API for warranty info |
| `--repaired <type>` | — | Add post-repair checks: `screen`, `motherboard`, `battery`, `storage`, `keyboard` |

**Exit codes:** `0` = pass, `2` = one or more checks failed.

**Examples**
```bash
# Full interactive QC (technician workflow)
tunit qc

# Automated only, no human prompts — good for CI
tunit qc --auto

# After a battery swap — adds charging-circuit check
tunit qc --repaired battery

# Full QC with warranty lookup and screen-repair follow-up
tunit qc --repaired screen --lookup
```

Automated checks include: serial number, model, OS version, CPU, RAM, BIOS version, battery health, storage S.M.A.R.T. status, and Windows activation state (Windows only).

Interactive checks: audio (both speakers), microphone, webcam, display dead-pixel, keyboard, USB ports.

Repair-specific follow-up checks:

| `--repaired` value | Extra check |
|---|---|
| `screen` | Wi-Fi antennas (antenna cable may be disturbed) |
| `motherboard` | Windows activation (may need re-sync after board swap) |
| `battery` | Charging circuit (AC adapter detected, battery charging) |

A text report (`QC_REPORT_YYYYMMDD_HHMMSS.txt`) is saved to the current directory.

---

### `tunit stress` — CPU/thermal stress test

Saturates all logical CPU cores and monitors temperature and throttling.

```
tunit stress [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--duration <d>` | `45s` | How long to run (e.g. `20s`, `2m`) |
| `--tier <1\|2>` | `1` | 1 = native Go matrix burn; 2 = external tools (FurMark / Prime95) |
| `--warn-temp <°C>` | `85` | Print warning colour above this temperature |
| `--fail-temp <°C>` | `95` | Mark result as failed above this temperature |

**Examples**
```bash
tunit stress                          # 45s, all cores, default thresholds
tunit stress --duration 2m            # longer soak
tunit stress --warn-temp 80 --fail-temp 90   # stricter thresholds
tunit stress --tier 2 --duration 60s  # GPU + external tools
```

Output shows live temperature, peak so far, and a throttling flag. After the run a 15-second cooldown is measured.

**Interactive menu tiers** map to these durations:

| Menu tier | Duration | Use case |
|---|---|---|
| 1 — Quick check | 20s | Baseline, fan spin |
| 2 — Standard | 60s | Recommended |
| 3 — Heavy | 120s | Post-cooling or motherboard repair |
| 4 — GPU stress | 60s | External FurMark (requires internet) |

---

### `tunit net` — network tests

Runs ping, DNS latency, TCP port probes, and a download bandwidth test.

```
tunit net [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--host <ip\|hostname>` | `8.8.8.8` | Host to ping and traceroute |
| `--no-bandwidth` | false | Skip the 25 MB bandwidth download |

**Example**
```bash
tunit net
tunit net --host 1.1.1.1 --no-bandwidth
```

TCP probes checked by default: `8.8.8.8:53`, `1.1.1.1:53`, `google.com:443`.

---

### `tunit load` — HTTP load test

Sends HTTP requests at a controlled rate and reports throughput and latency percentiles.

```
tunit load --url <url> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--url <url>` | — | **Required.** Target URL |
| `--method <verb>` | `GET` | HTTP method |
| `--rps <n>` | `50` | Requests per second |
| `--duration <d>` | `30s` | Test duration |
| `--body <string>` | — | Request body |
| `--header "Key: Value"` | — | Repeatable header flag |
| `--json <file>` | — | Save results to a JSON file |

**Examples**
```bash
# Simple GET
tunit load --url https://api.example.com/health --rps 100 --duration 60s

# POST with JSON body
tunit load --url https://api.example.com/items \
  --method POST \
  --header "Content-Type: application/json" \
  --body '{"name":"test"}' \
  --rps 20 --duration 30s

# Save results for later
tunit load --url https://api.example.com --rps 200 --duration 60s --json results.json
```

Output: total requests, error count, throughput (req/s), and latency at P50/P95/P99/P99.9/max.

---

### `tunit report` — list saved QC reports

Lists `QC_REPORT_*.txt` files in the current directory with timestamps.

```bash
tunit report
```

---

## Warranty lookup

`tunit qc --lookup` queries the manufacturer's public API using the machine's serial number. Supported vendors:

| Vendor | Coverage |
|---|---|
| Dell | Service Tag → entitlement expiry via Dell SBIL API |
| HP | Product number + serial → HP Care Pack expiry |
| Lenovo | Machine type + serial → Lenovo warranty expiry |

If the machine is offline or the serial isn't recognised the QC run still completes — warranty info is optional.

---

## Report format

Each QC run writes a plain-text report to the working directory:

```
QC_REPORT_20261105_143022.txt
```

The file contains the system identity block (hostname, serial, model, CPU, RAM, BIOS), all check results with PASS/WARN/FAIL/SKIP, the overall verdict, and warranty expiry if looked up.

JSON output is available for the load test via `--json`.

---

## CI / automation

```yaml
# GitHub Actions example
- name: Hardware QC
  run: tunit qc --auto
  # exits 2 on failure, 0 on pass
```

See [docs/ci-guide.md](docs/ci-guide.md) for a full CI/CD integration guide.

---

## Docs

- [QC workflow guide](docs/qc-guide.md) — step-by-step for technicians
- [CI integration guide](docs/ci-guide.md) — running tunit in pipelines and automation

---

## License

MIT
