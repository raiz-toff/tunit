# CI / Automation Guide

This guide covers using tunit in scripts, pipelines, and automated testing environments.

---

## Automated-only mode

The interactive checks (audio, webcam, keyboard, etc.) require a human in the loop. For CI use `--auto` to skip them and run only the automated hardware checks:

```bash
tunit qc --auto
```

**Exit codes:**
- `0` — all automated checks passed
- `2` — one or more checks returned FAIL

A `WARN` result exits `0`. Treat warns as informational; they won't block a pipeline unless you inspect the report file explicitly.

---

## GitHub Actions

```yaml
name: Hardware QC

on: [push]

jobs:
  qc:
    runs-on: self-hosted   # must be a self-hosted runner on the target machine
    steps:
      - name: Install tunit
        run: |
          curl -fsSL https://github.com/tunit/tunit/releases/latest/download/install.sh | bash

      - name: Run QC
        run: tunit qc --auto

      - name: Upload QC report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: qc-report
          path: QC_REPORT_*.txt
```

The `if: always()` on the upload step ensures the report is captured even when QC fails.

---

## GitLab CI

```yaml
hardware-qc:
  stage: test
  tags:
    - self-hosted        # target machine must have a registered GitLab runner
  script:
    - tunit qc --auto
  artifacts:
    when: always
    paths:
      - QC_REPORT_*.txt
    expire_in: 30 days
```

---

## Stress test in CI

Run a thermal soak after a build to catch throttling regressions:

```bash
tunit stress --duration 2m --fail-temp 90
```

Combine with QC in a pipeline:

```yaml
- name: QC
  run: tunit qc --auto

- name: Thermal soak
  run: tunit stress --duration 90s --warn-temp 80 --fail-temp 92
```

---

## Load test in CI

Test an HTTP endpoint after every deploy:

```bash
tunit load \
  --url https://staging.example.com/api/health \
  --rps 100 \
  --duration 60s \
  --json load-results.json
```

The JSON output file contains all latency percentiles and can be parsed by downstream steps:

```json
{
  "URL": "https://staging.example.com/api/health",
  "Method": "GET",
  "Duration": 60000000000,
  "TotalReqs": 6000,
  "Errors": 0,
  "LatP50": 12000000,
  "LatP95": 45000000,
  "LatP99": 98000000,
  "LatP999": 210000000,
  "LatMax": 350000000,
  "Throughput": 100.0
}
```

Latency values are nanoseconds. Divide by `1_000_000` for milliseconds.

**Example: fail the pipeline if P99 exceeds 200ms**

```bash
P99=$(jq '.LatP99 / 1000000' load-results.json)
if (( $(echo "$P99 > 200" | bc -l) )); then
  echo "P99 latency ${P99}ms exceeds 200ms threshold"
  exit 1
fi
```

---

## Network check in CI

Verify connectivity before running integration tests:

```bash
tunit net --no-bandwidth   # skip the 25MB download, just check ping/DNS/TCP
```

---

## Scripting with repair flags

For automated bench workflows where the repair type is already known:

```bash
# After a battery replacement
tunit qc --auto --repaired battery

# After a screen replacement (checks Wi-Fi antennas)
tunit qc --auto --repaired screen
```

---

## Collecting reports from multiple machines

tunit writes reports to the working directory. To aggregate from several machines:

1. Set `TUNIT_REPORT_DIR` — or just `cd` to a shared mount before running.
2. Use `tunit report` to list all reports in the current directory.

```bash
cd /mnt/reports/$(hostname)
tunit qc --auto
```

---

## Parsing the text report

If you need to extract the verdict from a text report without JSON output, grep for the VERDICT line:

```bash
grep "^VERDICT" QC_REPORT_*.txt
```

Output: `VERDICT: PASS` or `VERDICT: FAIL`.

For programmatic use, the exit code (`$?`) is simpler: `0` = pass, `2` = fail.

---

## Windows (PowerShell)

```powershell
# Install
iex (iwr https://github.com/tunit/tunit/releases/latest/download/tunit.ps1).Content

# Run automated QC
& tunit qc --auto
if ($LASTEXITCODE -eq 2) {
    Write-Error "QC failed"
    exit 1
}
```

---

## Scheduled nightly soak

Run a thermal stress test on a lab machine every night to catch cooling degradation early:

```yaml
# GitHub Actions scheduled workflow
on:
  schedule:
    - cron: '0 2 * * *'   # 2am daily

jobs:
  nightly-soak:
    runs-on: self-hosted
    steps:
      - name: Thermal soak
        run: tunit stress --duration 10m --warn-temp 82 --fail-temp 92

      - name: Network check
        run: tunit net
```
