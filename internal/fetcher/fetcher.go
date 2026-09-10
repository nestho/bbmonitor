package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/nestho/bbmonitor/internal/config"
	"github.com/nestho/bbmonitor/internal/storage"
)

type Adapter interface {
	Name() string
	Fetch(ctx context.Context, st *storage.Storage, client *http.Client) (*Result, error)
}

type Result struct {
	Source       string
	ProgramsNew  int
	ProgramsUpd  int
	TargetsNew   int
	TargetsUpd   int
	TargetsGone  int
	Changes      []storage.Change
	ETag         string
	LastModified string
	Error        error
}

type Manager struct {
	cfg      *config.Config
	store    *storage.Storage
	client   *http.Client
	adapters []Adapter
}

func NewManager(cfg *config.Config, store *storage.Storage, adapters []Adapter) *Manager {
	return &Manager{
		cfg: cfg, store: store, adapters: adapters,
		client: &http.Client{
			Timeout: 90 * time.Second,
			Transport: &http.Transport{MaxIdleConns: 20, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 15 * time.Second},
		},
	}
}

func (m *Manager) RunOnce(ctx context.Context) []Result {
	type job struct{ ad Adapter }
	jobs := make(chan job, len(m.adapters))
	results := make(chan Result, len(m.adapters))
	workers := m.cfg.Workers
	if workers > len(m.adapters) {
		workers = len(m.adapters)
	}
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				_ = m.store.UpsertSyncState(&storage.SyncState{Source: j.ad.Name(), Status: "running"})
				res, err := j.ad.Fetch(ctx, m.store, m.client)
				if res == nil {
					res = &Result{Source: j.ad.Name()}
				}
				res.Error = err
				if err != nil {
					_ = m.store.UpsertSyncState(&storage.SyncState{Source: j.ad.Name(), Status: "error", LastError: err.Error()})
				} else {
					_ = m.store.UpsertSyncState(&storage.SyncState{
						Source: j.ad.Name(), Status: "success", LastETag: res.ETag, LastMod: res.LastModified,
						LastSuccess: time.Now().UTC(), LastError: "",
					})
				}
				results <- *res
			}
		}()
	}
	for _, ad := range m.adapters {
		jobs <- job{ad: ad}
	}
	close(jobs)
	wg.Wait()
	close(results)
	var out []Result
	for r := range results {
		out = append(out, r)
	}
	return out
}

func Download(ctx context.Context, client *http.Client, url, etag, lastMod string) (body []byte, newETag, newLastMod string, notModified bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", "", false, err
	}
	req.Header.Set("User-Agent", "bbmonitor/1.0 (+https://github.com/nestho/bbmonitor)")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastMod != "" {
		req.Header.Set("If-Modified-Since", lastMod)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil, etag, lastMod, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", false, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, "", "", false, err
	}
	return data, resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), false, nil
}
