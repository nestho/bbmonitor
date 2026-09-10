# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- GitHub Pages landing page (live activity feed + charts)
- Multi-arch release workflow (Linux/Windows/macOS × amd64/arm64)
- USDT donation addresses (TRC20 + BEP20)

### Notes
- First complete public source tree on `main`.
