package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nestho/bbmonitor/internal/config"
	"github.com/nestho/bbmonitor/internal/fetcher"
	"github.com/nestho/bbmonitor/internal/fetcher/adapters"
	"github.com/nestho/bbmonitor/internal/notify"
	"github.com/nestho/bbmonitor/internal/storage"
	"github.com/nestho/bbmonitor/internal/tui"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config.yaml")
	once := flag.Bool("once", false, "run one sync and exit")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("data dir: %v", err)
	}
	if err := os.MkdirAll(cfg.ExportDir, 0o755); err != nil {
		log.Fatalf("export dir: %v", err)
	}

	store, err := storage.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	defer store.Close()

	var ads []fetcher.Adapter
	if cfg.Sources.Arkadiyt {
		ads = append(ads, adapters.NewArkadiyt())
	}
	if cfg.Sources.ProjectDiscovery {
		ads = append(ads, adapters.NewProjectDiscovery())
	}
	if cfg.Sources.Diodb {
		ads = append(ads, adapters.NewDiodb())
	}
	if cfg.Sources.Rix4uni {
		ads = append(ads, adapters.NewRix4uni())
	}
	if cfg.Sources.OrgsData {
		ads = append(ads, adapters.NewOrgsData())
	}
	if cfg.Sources.Dotgov {
		ads = append(ads, adapters.NewDotgov())
	}

	mgr := fetcher.NewManager(cfg, store, ads)
	notifier := notify.New(cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if *once || cfg.Mode == "daemon" {
		runDaemon(ctx, cfg, store, mgr, notifier, *once)
		return
	}

	model := tui.New(store)
	p := tea.NewProgram(model, tea.WithAltScreen())
	go func() {
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		doSync(ctx, cfg, store, mgr, notifier, p)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				doSync(ctx, cfg, store, mgr, notifier, p)
			}
		}
	}()
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}

func doSync(ctx context.Context, cfg *config.Config, store *storage.Storage, mgr *fetcher.Manager, notifier *notify.Notifier, p *tea.Program) {
	results := mgr.RunOnce(ctx)
	changes, err := store.UnnotifiedChanges(500)
	if err == nil && len(changes) > 0 {
		if err := notifier.SendChanges(ctx, changes); err == nil {
			ids := make([]int64, len(changes))
			for i, c := range changes {
				ids[i] = c.ID
			}
			_ = store.MarkChangesNotified(ids)
		}
	}
	_ = store.ExportProgramsJSON(filepath.Join(cfg.ExportDir, "programs.json"))
	_ = store.ExportTargetsJSON(filepath.Join(cfg.ExportDir, "targets.json"))
	if p != nil {
		p.Send(tui.SyncDone(results))
	}
}

func runDaemon(ctx context.Context, cfg *config.Config, store *storage.Storage, mgr *fetcher.Manager, notifier *notify.Notifier, once bool) {
	log.Printf("bbmonitor daemon started (interval=%s)", cfg.Interval)
	run := func() {
		log.Printf("sync started…")
		results := mgr.RunOnce(ctx)
		for _, r := range results {
			if r.Error != nil {
				log.Printf("[%s] error: %v", r.Source, r.Error)
			} else {
				log.Printf("[%s] new programs=%d new targets=%d", r.Source, r.ProgramsNew, r.TargetsNew)
			}
		}
		changes, err := store.UnnotifiedChanges(1000)
		if err == nil && len(changes) > 0 {
			log.Printf("notifying %d changes", len(changes))
			if err := notifier.SendChanges(ctx, changes); err != nil {
				log.Printf("notify error: %v", err)
			} else {
				ids := make([]int64, len(changes))
				for i, c := range changes {
					ids[i] = c.ID
				}
				_ = store.MarkChangesNotified(ids)
			}
		}
		_ = store.ExportProgramsJSON(filepath.Join(cfg.ExportDir, "programs.json"))
		_ = store.ExportTargetsJSON(filepath.Join(cfg.ExportDir, "targets.json"))
		log.Printf("sync finished")
	}
	run()
	if once {
		return
	}
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down")
			return
		case <-ticker.C:
			run()
		}
	}
}
