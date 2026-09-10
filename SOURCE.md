# Building from source

```bash
git clone https://github.com/nestho/bbmonitor.git
cd bbmonitor
go mod tidy
go build -o bbmonitor ./cmd/bbmonitor
cp config.example.yaml config.yaml
./bbmonitor -once
```

Full package tree is on `main`:

- `cmd/bbmonitor`
- `internal/config`
- `internal/storage`
- `internal/fetcher` + `adapters/`
- `internal/notify`
- `internal/tui`
- `systemd/`, `docs/`, workflows
