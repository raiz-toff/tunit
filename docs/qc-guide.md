# QC Workflow Guide

This guide walks a technician through using `tunit qc` to quality-check a laptop or desktop — either as part of an intake, after a repair, or before shipping.

---

## Overview

A full QC run has two phases:

1. **Automated checks** — tunit reads hardware data from the OS: serial number, model, CPU, RAM, BIOS version, battery state, storage health, and Windows activation. No user input needed.
2. **Interactive checks** — tunit guides you through six human-verified steps: audio, microphone, webcam, dead pixels, keyboard, and USB ports.

If a repair was done you can flag the repair type. tunit will append extra follow-up checks specific to that repair.

Total time: **8–12 minutes** for a full interactive run.

---

## Step 1 — Launch tunit

**Recommended:** run with no arguments to get the interactive menu:

```bash
tunit
```

Choose **1 › Full Computer Check** from the menu, then follow the prompts.

**Or** run the QC command directly:

```bash
tunit qc
```

---

## Step 2 — Select repair type

tunit asks whether a repair was done on this unit:

```
1 › No repair — fresh intake
2 › Screen / Display replaced
3 › Motherboard replaced
4 › Battery replaced
5 › Storage (SSD / HDD) replaced
6 › Keyboard or top case replaced
7 › Other / Not sure
```

Choose the matching option. For fresh intakes choose **1**. For units coming off the bench, choose the repair type — this adds targeted follow-up checks after the main run.

---

## Step 3 — Automated checks

tunit reads the system and prints each check result live:

```
  Checking Serial number...              [PASS]
  Checking Model...                      [PASS]
  Checking CPU...                        [PASS]
  Checking RAM...                        [PASS]
  Checking BIOS version...               [PASS]
  Checking Battery health...             [WARN]
  Checking Storage S.M.A.R.T....         [PASS]
```

**Status meanings:**

| Status | Meaning |
|---|---|
| `PASS` | Check completed, result is healthy |
| `WARN` | Potential issue, worth noting — does not fail the unit alone |
| `FAIL` | Clear problem — drives the overall verdict to FAIL |
| `SKIP` | Check could not run (e.g. Linux-only check on Windows) |
| `INFO` | Informational only, no pass/fail judgment |

You don't need to do anything during this phase. It finishes in under a minute.

---

## Step 4 — Interactive checks

tunit walks you through each step one at a time. For every check answer:

- **Y** — yes, it passed
- **N** — no, there's a problem
- **S** — skip (use if the check isn't applicable or hardware isn't present)

### Audio (speakers)
tunit plays a tone. Listen for audio from both the left and right speaker. Answer Y if both are clear and at normal volume.

> Common fails: one channel silent (speaker connector loose), crackling (speaker damaged).

### Microphone
tunit records 3 seconds and plays back what it captured. Answer Y if you can clearly hear the recorded audio.

> Common fails: silence (microphone not connected), very low volume, heavy distortion.

### Webcam
tunit opens a 3-second camera preview. Answer Y if you see a live image (not a black screen or error).

> Common fails: black screen (camera ribbon not seated), error message (driver missing on Windows).

### Display dead pixel
tunit cycles through five full-screen test colours: white → black → red → green → blue. Press Enter to advance each screen.

Examine the entire display for:
- **Dead pixels** — always black dots
- **Stuck pixels** — always lit dots of a fixed colour
- **Backlight bleed** — bright areas along edges (visible on black screen)

Answer Y if the display is clean.

> Tip: check the black screen for bleed and the white screen for stuck dark pixels.

### Keyboard
tunit prompts you to type a sequence of characters. Press each key shown. Answer Y if every key registered correctly with no misses, doubles, or stuck keys.

> Common fails after a top-case replacement: ribbon cable not fully inserted, one or two rows of keys unresponsive.

### USB ports
tunit waits for USB device insertions. Plug a USB drive (or any USB device) into each port one at a time and wait for the detection indicator. Answer Y if every port detected the device.

> Common fails: ports that show power (charging a phone) but don't enumerate data — answer N if a port doesn't trigger detection.

---

## Step 5 — Repair-specific follow-up checks

If a repair type was selected, tunit runs one additional check:

| Repair | Follow-up check |
|---|---|
| Screen / Display | Wi-Fi signal — the antenna cables run alongside the screen hinge and can be accidentally disconnected |
| Motherboard | Windows activation — a board swap may de-activate Windows; check Settings → System → Activation |
| Battery | Charging circuit — verify the battery shows "Charging" when AC is connected |

---

## Step 6 — Warranty lookup (optional)

Run with `--lookup` to query the manufacturer's API:

```bash
tunit qc --lookup
```

tunit reads the serial number and queries Dell, HP, or Lenovo depending on the machine. The warranty expiry date appears in the report if found. This requires an internet connection.

---

## Step 7 — Report

After all checks complete, tunit prints a summary and saves a report:

```
  Saved: QC_REPORT_20261105_143022.txt
```

The file is written to the directory you ran tunit from. It contains:

- System identity: hostname, serial, model, CPU, RAM, BIOS, OS
- Every check with its status and detail text
- Overall verdict: `PASS`, `WARN`, or `FAIL`
- Warranty expiry if looked up

Keep this file with the unit's service record or attach it to the ticket.

---

## Verdict guide

| Verdict | Action |
|---|---|
| `PASS` | Unit is good to go |
| `WARN` | Review the warnings — may require a note on the unit (e.g. battery health below optimal) |
| `FAIL` | Do not ship. At least one check failed — address the issue and re-run |

---

## Common scenarios

**Intake of a used unit (no repair)**
```bash
tunit qc
# → Select 1 (no repair)
# → Run all checks
# → Keep the report
```

**Post-repair QC after a battery replacement**
```bash
tunit qc --repaired battery
```

**Quick automated check for a machine that was just reimaged (no interactive checks needed)**
```bash
tunit qc --auto
```

**Full QC with warranty lookup before listing for resale**
```bash
tunit qc --lookup
```

---

## Tips

- Run tunit from a folder where you keep service records so reports land in the right place.
- If the machine doesn't have speakers (e.g. a bare board on the bench), use **S** to skip audio and microphone checks.
- The dead-pixel test requires the full display — close any other windows first.
- For the USB check, a simple USB thumb drive works best; avoid hubs (plug devices directly).
