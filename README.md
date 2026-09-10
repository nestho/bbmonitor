# bbmonitor

**Clean, production-ready Bug Bounty scope monitor** written in Go.

Concurrently fetches public bug bounty / VDP scopes from multiple sources, stores them in SQLite, detects **every** change, exports JSON, and notifies via Telegram or webhook.

Single binary • TUI + daemon • systemd ready • resume-safe

---

## Features

- **Sources** (pluggable adapters)
  - arkadiyt/bounty-targets-data (HackerOne, Bugcrowd, Intigriti, YesWeHack, Federacy)
  - projectdiscovery/public-bugbounty-programs
  - disclose/diodb
  - rix4uni/scope
  - nikitastupin/orgs-data (program → GitHub org mapping)
- SQLite storage (WAL) + automatic JSON export
- Diff on every change (program added/removed/updated, target added/removed/updated)
- Telegram + generic Webhook notifications
- Beautiful minimal TUI (bubbletea) + pure daemon mode
- Config via `config.yaml` + environment variables (perfect for systemd/VPS)
- Resume-safe state

## Quick Start

```bash
# Build
go build -o bbmonitor ./cmd/bbmonitor

# Copy config
cp config.example.yaml config.yaml

# One-shot sync
./bbmonitor -once

# Interactive TUI
./bbmonitor

# Daemon
BB_MODE=daemon ./bbmonitor
```

### Environment variables (override config)

| Variable | Description |
|----------|-------------|
| `BB_MODE` | `tui` or `daemon` |
| `BB_INTERVAL` | e.g. `10m` |
| `BB_DATA_DIR` | data directory |
| `BB_DB_PATH` | sqlite path |
| `BB_EXPORT_DIR` | JSON export path |
| `BB_TELEGRAM_BOT_TOKEN` | Telegram bot token |
| `BB_TELEGRAM_CHAT_ID` | chat / group id |
| `BB_WEBHOOK_URL` | generic webhook |
| `BB_SOURCE_*` | enable/disable individual sources |

## systemd

```bash
sudo cp bbmonitor /opt/bbmonitor/
sudo cp config.example.yaml /etc/bbmonitor/config.yaml
sudo cp systemd/bbmonitor.service /etc/systemd/system/
# edit Environment= lines for secrets
sudo systemctl daemon-reload
sudo systemctl enable --now bbmonitor
```

## Project structure

```
bbmonitor/
├── cmd/bbmonitor/
├── internal/
│   ├── config/
│   ├── storage/          # SQLite
│   ├── fetcher/ + adapters/
│   ├── notify/
│   └── tui/
├── systemd/
├── docs/                 # GitHub Pages
└── .github/workflows/
```

## Web

A simple status page is available via GitHub Pages:  
→ [https://nestho.github.io/bbmonitor](https://nestho.github.io/bbmonitor)

## Contributing

PRs and issues welcome. New source adapters are easy to add under `internal/fetcher/adapters/`.

## License

MIT

---

If this tool helped you, consider starring the repo or supporting via [GitHub Sponsors](https://github.com/sponsors/nestho).
