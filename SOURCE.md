# Source status

Core packages already on this repo:

- `cmd/bbmonitor/main.go`
- `internal/config/`
- `internal/fetcher/` (core)
- `docs/`, `systemd/`, `config.example.yaml`, `go.mod`, `LICENSE`

Remaining packages (storage, adapters, tui, notify) are maintained in the development workspace and will continue to be pushed in subsequent commits. The architecture and public interface are stable.

To build from what is currently public:

```bash
git clone https://github.com/nestho/bbmonitor
cd bbmonitor
# full source will be completed shortly; watch Releases / commits
```

Contributions welcome once the full tree is present.
