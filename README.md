# tunit

Hardware QC, CPU thermal stress, network tests, and HTTP load testing — single static binary, no dependencies, works in a repair shop or a CI pipeline.

```
tunit                         # interactive menu
tunit qc --auto               # automated hardware diagnostics (CI-safe)
tunit stress --duration 2m    # CPU/thermal stress test
tunit net                     # ping, DNS, TCP, bandwidth
tunit load --url https://… --rps 200 --duration 60s
```

---

## Install

**Linux / macOS**
```bash
curl -fsSL https://github.com/raiz-toff/tunit/releases/latest/download/install.sh | bash
```
Installs to `/usr/local/bin`. Override with `TUNIT_BIN_DIR=/your/path`.

**Windows (PowerShell)**
```powershell
iex (iwr https://github.com/raiz-toff/tunit/releases/latest/download/tunit.ps1).Content
```

**Manual binary** — download from [Releases](https://github.com/raiz-toff/tunit/releases/latest):

| Platform | File |
|---|---|
| Linux x86-64 | `tunit-linux-amd64` |
| Linux ARM64 | `tunit-linux-arm64` |
| macOS x86-64 | `tunit-darwin-amd64` |
| macOS Apple Silicon | `tunit-darwin-arm64` |
| Windows x86-64 | `tunit-windows-amd64.exe` |

**Build from source** (requires Go 1.23+)
```bash
git clone https://github.com/raiz-toff/tunit
cd tunit
go build -o tunit ./cmd/tunit
```

---

## Commands

### `tunit` — interactive menu

No arguments. Guided prompts for all modes. Saves a report file when done.

---

### `tunit qc` — hardware QC

Automated hardware checks (serial, model, CPU, RAM, BIOS, battery health, storage S.M.A.R.T.) followed by guided human checks (audio, microphone, webcam, display dead pixel, keyboard, USB ports).

```
tunit qc [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--auto` | false | Skip interactive human checks (CI-safe) |
| `--lookup` | false | Query manufacturer API for warranty expiry |
| `--repaired <type>` | — | Add post-repair checks: `screen`, `motherboard`, `battery`, `storage`, `keyboard` |

**Exit codes:** `0` = pass or warn, `2` = fail.

```bash
tunit qc                        # full interactive QC
tunit qc --auto                 # automated only, no prompts
tunit qc --repaired battery     # adds post-battery-swap charging check
tunit qc --repaired screen --lookup  # + warranty lookup
```

Repair-specific follow-ups:

| `--repaired` | Extra check |
|---|---|
| `screen` | Wi-Fi antennas (antenna cable may be disturbed by screen swap) |
| `motherboard` | Windows activation (may need re-sync after board swap) |
| `battery` | Charging circuit (AC connected, battery showing as charging) |

A text report (`QC_REPORT_YYYYMMDD_HHMMSS.txt`) is saved to the working directory.

---

### `tunit stress` — CPU/thermal stress

Saturates all logical cores and monitors temperature and throttling.

```
tunit stress [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--duration <d>` | `45s` | How long to run (`20s`, `2m`, etc.) |
| `--tier <1\|2>` | `1` | 1 = native Go matrix burn; 2 = external tools (FurMark / Prime95) |
| `--warn-temp <°C>` | `85` | Warn colour above this temperature |
| `--fail-temp <°C>` | `95` | Mark result failed above this temperature |

```bash
tunit stress                              # 45s, default thresholds
tunit stress --duration 2m --fail-temp 90
```

Prints live temperature, peak, and throttle flag. Measures 15-second cooldown after the run.

---

### `tunit net` — network tests

Ping, DNS latency, TCP port probes, and download bandwidth.

```
tunit net [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--host <ip\|hostname>` | `8.8.8.8` | Host to ping |
| `--no-bandwidth` | false | Skip the 25 MB bandwidth download |

Default TCP probes: `8.8.8.8:53`, `1.1.1.1:53`, `google.com:443`.

---

### `tunit load` — HTTP load test

Sends requests at a fixed rate and reports throughput and latency percentiles.

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
| `--header "Key: Value"` | — | Repeatable |
| `--json <file>` | — | Save results to a JSON file |

```bash
tunit load --url https://api.example.com/health --rps 100 --duration 60s

tunit load --url https://api.example.com/items \
  --method POST \
  --header "Content-Type: application/json" \
  --body '{"name":"test"}' \
  --rps 20 --duration 30s --json results.json
```

Output: total requests, errors, throughput (req/s), and latency at P50/P95/P99/P99.9/max.

---

### `tunit report` — list saved reports

Lists `QC_REPORT_*.txt` files in the current directory.

---

## Warranty lookup

`tunit qc --lookup` queries the manufacturer's public API using the machine's serial number.

| Vendor | Source |
|---|---|
| Dell | Service Tag → Dell SBIL API |
| HP | Product number + serial → HP Care Pack |
| Lenovo | Machine type + serial → Lenovo warranty API |

If offline or the serial isn't found, the QC run still completes — warranty info is optional.

---

## CI / automation

```yaml
# GitHub Actions — self-hosted runner on the target machine
- name: Hardware QC
  run: tunit qc --auto   # exits 2 on failure
```

See [docs/ci-guide.md](docs/ci-guide.md) for a full CI/CD integration guide.

---

## Docs

- [docs/qc-guide.md](docs/qc-guide.md) — step-by-step QC workflow for technicians
- [docs/ci-guide.md](docs/ci-guide.md) — running tunit in pipelines and scripts

---

## License

MIT · [github.com/raiz-toff/tunit](https://github.com/raiz-toff/tunit)
