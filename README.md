# tunit

> Single-binary system diagnostic, stress testing, and load testing tool.
> Works in a repair shop or a CI pipeline. No dependencies.

### Quick install

```bash
# Linux / macOS
curl -fsSL https://github.com/raiz-toff/tunit/releases/latest/download/install.sh | bash
```

```powershell
# Windows (PowerShell — run as Administrator)
irm https://github.com/raiz-toff/tunit/releases/latest/download/tunit.ps1 | iex
```

Then just run:

```bash
tunit
```

That opens the interactive menu — it walks you through everything.

---

### What it does

| Command | What it runs |
|---|---|
| `tunit` | Interactive guided menu (start here) |
| `tunit qc` | Full hardware QC — serial, CPU, RAM, BIOS, battery, S.M.A.R.T., then human checks for audio, webcam, keyboard, display, USB |
| `tunit stress` | CPU thermal stress test with live temp monitoring and throttle detection |
| `tunit net` | Ping, DNS latency, TCP port probes, bandwidth test |
| `tunit load` | HTTP load test with rate control and latency percentiles |
| `tunit report` | List saved QC reports in the current directory |

---

### Install options

**Download binary** from [Releases](https://github.com/raiz-toff/tunit/releases/latest):

| Platform | Binary |
|---|---|
| Linux x86-64 | `tunit-linux-amd64` |
| Linux ARM64 | `tunit-linux-arm64` |
| macOS Intel | `tunit-darwin-amd64` |
| macOS Apple Silicon | `tunit-darwin-arm64` |
| Windows x86-64 | `tunit-windows-amd64.exe` |

**Build from source** (Go 1.23+):

```bash
git clone https://github.com/raiz-toff/tunit && cd tunit
go build -o tunit ./cmd/tunit
```

---

### Hardware QC

```bash
tunit qc                            # full interactive QC
tunit qc --auto                     # skip human checks (CI-safe)
tunit qc --repaired battery         # add post-repair checks
tunit qc --repaired screen --lookup # + warranty lookup
```

| Flag | Description |
|---|---|
| `--auto` | Skip interactive human checks — automated only |
| `--lookup` | Query Dell / HP / Lenovo API for warranty expiry |
| `--repaired <type>` | Post-repair follow-ups: `screen`, `motherboard`, `battery`, `storage`, `keyboard` |

Exit code `0` on pass, `2` on fail.

**Automated checks:** serial, model, OS, CPU, RAM, BIOS, battery health, storage S.M.A.R.T.

**Human checks:** speakers, microphone, webcam, dead pixel test, keyboard, USB ports.

**Post-repair follow-ups:**

| Repair type | Extra check |
|---|---|
| `screen` | Wi-Fi antenna signal (cable runs through hinge) |
| `motherboard` | Windows activation (may deactivate after board swap) |
| `battery` | Charging circuit (AC adapter + battery charging state) |

**Report output.** Interactive runs show the full report on screen and
leave **no file on the machine** — it's written to a session temp directory
that's removed on exit (including on Ctrl+C). Add `--auto` to keep a durable
`QC_REPORT_YYYYMMDD_HHMMSS.txt` in the current directory (for CI or your own
records); `tunit report` lists those saved files.

---

### Stress test

```bash
tunit stress                             # 45 seconds, default thresholds
tunit stress --duration 2m               # longer soak
tunit stress --duration 2m --fail-temp 90
tunit stress --tier 2 --duration 60s     # GPU via FurMark
```

| Flag | Default | Description |
|---|---|---|
| `--duration` | `45s` | Test duration (`20s`, `2m`, etc.) |
| `--tier` | `1` | 1 = Go matrix burn, 2 = FurMark / Prime95 |
| `--warn-temp` | `85` | Yellow warning above this °C |
| `--fail-temp` | `95` | Fail above this °C |

Shows live temperature, peak, throttle flag, and a 15-second cooldown measurement.

---

### Network test

```bash
tunit net                                # full: ping, DNS, TCP, bandwidth
tunit net --no-bandwidth                 # skip the 25 MB download
tunit net --host 1.1.1.1                 # custom ping target
```

Tests ping (min/avg/max/loss), DNS latency, TCP probes (`8.8.8.8:53`, `1.1.1.1:53`, `google.com:443`), and download speed via Cloudflare CDN.

---

### HTTP load test

```bash
tunit load --url https://api.example.com/health --rps 100 --duration 60s

tunit load --url https://api.example.com/items \
  --method POST \
  --header "Content-Type: application/json" \
  --body '{"name":"test"}' \
  --rps 200 --duration 30s \
  --json results.json
```

| Flag | Default | Description |
|---|---|---|
| `--url` | — | Target URL (required) |
| `--method` | `GET` | HTTP method |
| `--rps` | `50` | Requests per second |
| `--duration` | `30s` | Test duration |
| `--body` | — | Request body |
| `--header` | — | Request header (repeatable) |
| `--json` | — | Save results to JSON file |

Reports: total requests, errors, throughput (req/s), latency P50 / P95 / P99 / P99.9 / max.

---

### CI / automation

```yaml
- name: Hardware QC
  run: tunit qc --auto    # exits 2 on failure

- name: Thermal soak
  run: tunit stress --duration 90s --fail-temp 92
```

See the full [CI integration guide](docs/ci-guide.md).

---

### Docs

- [QC workflow guide](docs/qc-guide.md) — step-by-step for technicians
- [CI integration guide](docs/ci-guide.md) — pipelines, scripts, JSON output

---

### License

MIT — [github.com/raiz-toff/tunit](https://github.com/raiz-toff/tunit)
