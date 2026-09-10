# Building from source

```bash
git clone https://github.com/nestho/bbmonitor.git
cd bbmonitor
go build -o bbmonitor ./cmd/bbmonitor
```

Packages under active sync from the development tree: `internal/storage`, `internal/tui`, and fetcher adapters. Core entrypoint, config, fetcher manager, and notify are already present.
