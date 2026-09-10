package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nestho/bbmonitor/internal/fetcher"
	"github.com/nestho/bbmonitor/internal/storage"
)

const pdURL = "https://raw.githubusercontent.com/projectdiscovery/public-bugbounty-programs/main/dist/data.json"

type ProjectDiscoveryAdapter struct{}

func NewProjectDiscovery() *ProjectDiscoveryAdapter { return &ProjectDiscoveryAdapter{} }
func (a *ProjectDiscoveryAdapter) Name() string { return "projectdiscovery" }

type pdRoot struct {
	Programs []struct {
		Name    string   `json:"name"`
		URL     string   `json:"url"`
		Bounty  bool     `json:"bounty"`
		Domains []string `json:"domains"`
	} `json:"programs"`
}

func (a *ProjectDiscoveryAdapter) Fetch(ctx context.Context, st *storage.Storage, client *http.Client) (*fetcher.Result, error) {
	res := &fetcher.Result{Source: a.Name()}
	state, _ := st.GetSyncState(a.Name())
	body, etag, lastMod, notMod, err := fetcher.Download(ctx, client, pdURL, state.LastETag, state.LastMod)
	if err != nil {
		return res, err
	}
	if notMod {
		return res, nil
	}
	res.ETag, res.LastModified = etag, lastMod
	var root pdRoot
	if err := json.Unmarshal(body, &root); err != nil {
		return res, fmt.Errorf("parse projectdiscovery: %w", err)
	}
	for _, p := range root.Programs {
		handle := sanitizeHandle(p.Name)
		raw, _ := json.Marshal(p)
		prog := &storage.Program{Source: a.Name(), Handle: handle, Name: p.Name, URL: p.URL, OffersBounty: p.Bounty, Platform: "public", Layer: "program", RawJSON: string(raw)}
		progID, isNew, err := st.UpsertProgram(prog)
		if err != nil {
			continue
		}
		if isNew {
			res.ProgramsNew++
			ch := storage.Change{Source: a.Name(), Kind: "program_added", Entity: handle, Details: fmt.Sprintf(`{"name":%q}`, p.Name)}
			_ = st.RecordChange(&ch)
			res.Changes = append(res.Changes, ch)
		}
		seen := map[string]struct{}{}
		for _, d := range p.Domains {
			if d == "" {
				continue
			}
			seen[d] = struct{}{}
			t := &storage.Target{ProgramID: progID, Source: a.Name(), AssetIdentifier: d, AssetType: "DOMAIN", EligibleForBounty: p.Bounty, EligibleForSubmission: true, InScope: true}
			_, tNew, err := st.UpsertTarget(t)
			if err != nil {
				continue
			}
			if tNew {
				res.TargetsNew++
				ch := storage.Change{Source: a.Name(), Kind: "target_added", Entity: d, Details: fmt.Sprintf(`{"program":%q}`, handle)}
				_ = st.RecordChange(&ch)
				res.Changes = append(res.Changes, ch)
			}
		}
		_ = st.MarkMissingTargets(a.Name(), seen, progID)
	}
	return res, nil
}

func sanitizeHandle(name string) string {
	r := make([]rune, 0, len(name))
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			r = append(r, c)
		} else if c == ' ' {
			r = append(r, '_')
		}
	}
	if s := string(r); s != "" {
		return s
	}
	return "unknown"
}
