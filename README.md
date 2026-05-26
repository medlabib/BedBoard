# BedBoard

<p align="center">
  <img src="logo.png" alt="BedBoard" width="120" />
</p>

<p align="center">
  <img alt="Local First" src="https://img.shields.io/badge/Local%20First-Fast%20on%20site-7ab893" />
  <img alt="Real-Time" src="https://img.shields.io/badge/Real--Time-Bed%20visibility-7fa7d4" />
  <img alt="Patient Flow" src="https://img.shields.io/badge/Patient%20Flow-Assigned%20to%20Archive-a58ac9" />
  <img alt="Safe Access" src="https://img.shields.io/badge/Safe%20Access-Role%20based-d97a70" />
</p>

<p align="center">
  <code style="background:#7ab893;color:#1e1a17;padding:4px 8px;border-radius:999px;">#7ab893</code>
  <code style="background:#7fa7d4;color:#1e1a17;padding:4px 8px;border-radius:999px;">#7fa7d4</code>
  <code style="background:#a58ac9;color:#1e1a17;padding:4px 8px;border-radius:999px;">#a58ac9</code>
  <code style="background:#d97a70;color:#1e1a17;padding:4px 8px;border-radius:999px;">#d97a70</code>
</p>

BedBoard helps emergency and ward teams answer one operational question in seconds:

**Which beds are available now, who is assigned, and what is the next patient action?**

## The Problem It Solves

Hospital teams often lose time between handwritten notes, verbal updates, and fragmented screens.

- Bed status changes are not visible to everyone at the same time.
- Patient assignment can lag behind reality.
- Consultation and archive steps are easy to miss during rush hours.
- Leadership lacks a clear daily view of throughput.

## The BedBoard Solution

BedBoard is built for a single-site, local deployment where speed and clarity matter most.

- One live board for the full bed map.
- Direct patient assignment from bed cards and patient list.
- Clear lifecycle: unassigned, assigned, consulted, archived.
- Full-screen patient view for display screens.
- Simple role model: staff can operate beds, admins manage configuration and users.

## What Teams Gain

- Faster decisions at shift change.
- Fewer coordination errors between triage, nursing, and consultation.
- Better visibility of occupancy pressure.
- Reliable local operation without external cloud dependency.

## Typical Workflow

1. Staff logs in on the local station.
2. A patient is created (assigned or unassigned).
3. Patient is assigned to a bed directly from the main board.
4. Bed status is updated during care (occupied, cleaning, alert, free).
5. Consultation is marked complete, then patient is archived.
6. Stats view reflects daily activity for follow-up.

## Quick Start

```bash
npm --prefix frontend ci
npm --prefix frontend run build
go run .
```

Open http://localhost:8080

Default admin access:

- Username: `admin`
- Password: `admin123`

## Releases

Each release ships ready-to-download artifacts for Windows and Linux.

- Windows executable and ZIP package
- Linux binary and tar.gz package
- Integrity and signature files for verification

## Deployment Positioning

BedBoard is intentionally local-first for private hospital environments.
For internet-facing deployments, place it behind HTTPS and restricted network access policies.
