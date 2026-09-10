# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial public structure (config, fetcher core, main, docs, systemd)
- GitHub Pages status page
- Multi-platform release workflow

## [0.1.0] - 2026-09-10

### Added
- Core daemon + TUI architecture (Go + bubbletea)
- SQLite storage with WAL mode
- Source adapters: arkadiyt, projectdiscovery, diodb, rix4uni, orgsdata
- Diff tracking for every program/target change
- Telegram + generic webhook notifications
- Config via YAML + environment variables
- systemd unit example
- Resume-safe sync state
- JSON export (programs + targets)
- GitHub Pages landing page with donation section
- Multi-arch release workflow (Linux/Windows/macOS × amd64/arm64)

### Notes
- First public release. Full adapter tree continues to land in follow-up commits.
