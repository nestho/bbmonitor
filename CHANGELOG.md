# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **Storage schema v2 (stability)**
  - Versioned migrations via `schema_version` table
  - Content-hash based upserts (avoid noisy updates when data unchanged)
  - Soft-delete (`removed_at`) for programs and targets
  - `layer` column (`program` | `inventory`)
  - Stronger composite indexes for lookup and change queries
  - Production SQLite pragmas: WAL, foreign_keys, busy_timeout 10s, mmap, cache
  - WAL checkpoint on close
  - Atomic JSON export (write `.tmp` then rename)
  - `IntegrityCheck()` helper

### Added
- Dotgov inventory adapter (cisagov/dotgov-data)
- Dual-layer model documented in ARCHITECTURE.md

## [0.1.0] - 2026-09-10

### Added
- Core daemon + TUI (Go + bubbletea)
- SQLite storage (WAL) with programs, targets, changes, sync_state
- Adapters: arkadiyt, projectdiscovery, diodb, rix4uni, orgsdata
- Diff on every program/target change
- Telegram + webhook notifications
- Config via YAML + environment variables
- systemd unit example
- Resume-safe sync state
- JSON export (programs + targets)
- GitHub Pages landing page
- Multi-arch release workflow
- USDT donation addresses (TRC20 + BEP20)
