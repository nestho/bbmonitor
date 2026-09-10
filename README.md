# bbmonitor

**Clean, production-ready Bug Bounty scope monitor** written in Go.

Concurrently fetches public bug bounty / VDP scopes from multiple sources, stores them in SQLite, detects **every** change, exports JSON, and notifies via Telegram or webhook.

Single binary · TUI + daemon · systemd ready · resume-safe · multi-arch releases

[![Release](https://img.shields.io/github/v/release/nestho/bbmonitor?style=flat-square)](https://github.com/nestho/bbmonitor/releases)
[![License](https://img.shields.io/github/license/nestho/bbmonitor?style=flat-square)](LICENSE)

---

## Features

- **Sources** (pluggable adapters)
  - [arkadiyt/bounty-targets-data](https://github.com/arkadiyt/bounty-targets-data) — HackerOne, Bugcrowd, Intigriti, YesWeHack, Federacy
  - [projectdiscovery/public-bugbounty-programs](https://github.com/projectdiscovery/public-bugbounty-programs)
  - [disclose/diodb](https://github.com/disclose/diodb)
  - [rix4uni/scope](https://github.com/rix4uni/scope)
  - [nikitastupin/orgs-data](https://github.com/nikitastupin/orgs-data) — program → GitHub org mapping
- SQLite storage (WAL) + automatic JSON export
- Diff on **every** change (program / target added, removed, updated)
- Telegram + generic Webhook notifications
- Minimal TUI (bubbletea) + pure daemon mode
- Config via `config.yaml` **and** environment variables
- Resume-safe sync state
- Multi-platform binaries (Linux / Windows / macOS × amd64 / arm64)

## Quick Start

```bash
# From source
git clone https://github.com/nestho/bbmonitor.git
cd bbmonitor
go build -o bbmonitor ./cmd/bbmonitor

cp config.example.yaml config.yaml
./bbmonitor -once          # one-shot sync
./bbmonitor                # interactive TUI
BB_MODE=daemon ./bbmonitor # background
```

Or download a pre-built binary from [Releases](https://github.com/nestho/bbmonitor/releases).

### Environment variables

| Variable | Description |
|----------|-------------|
| `BB_MODE` | `tui` or `daemon` |
| `BB_INTERVAL` | poll interval, e.g. `10m` |
| `BB_DATA_DIR` | data directory |
| `BB_DB_PATH` | SQLite path |
| `BB_EXPORT_DIR` | JSON export directory |
| `BB_TELEGRAM_BOT_TOKEN` | Telegram bot token |
| `BB_TELEGRAM_CHAT_ID` | chat / group id |
| `BB_WEBHOOK_URL` | generic webhook URL |
| `BB_SOURCE_*` | enable/disable individual sources |

## systemd

```bash
sudo cp bbmonitor /opt/bbmonitor/
sudo cp config.example.yaml /etc/bbmonitor/config.yaml
sudo cp systemd/bbmonitor.service /etc/systemd/system/
# set secrets via Environment= in the unit or drop-in
sudo systemctl daemon-reload
sudo systemctl enable --now bbmonitor
```

## Web

Status / landing page (GitHub Pages):  
→ [https://nestho.github.io/bbmonitor](https://nestho.github.io/bbmonitor)

## Changelog

See [CHANGELOG.md](CHANGELOG.md).  
Releases include the same notes plus downloadable binaries for all supported platforms.

## Support / Donate

If this tool saved you time, consider supporting development.

**USDT (TRC20)**  
`TAH1B9ksFekoqBmQaRbGqooca7MSUadvn2`

**USDT (BEP20)**  
`0x5f4d2227e66837Dcf94B1d647218edF16b35eba1`

Stars and issues also help a lot.

## License

MIT — see [LICENSE](LICENSE).
