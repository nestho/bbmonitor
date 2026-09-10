package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/nestho/bbmonitor/internal/fetcher"
	"github.com/nestho/bbmonitor/internal/storage"
)

const rix4uniProgramsURL = "https://raw.githubusercontent.com/rix4uni/scope/main/programs.json"

type Rix4uniAdapter struct{}

func NewRix4uni() *Rix4uniAdapter { return &Rix4uniAdapter{} }
func (a *Rix4uniAdapter) Name() string { return "rix4uni" }

type rixProgram struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Handle   string   `json:"handle"`
	Platform string   `json:"platform"`
	Bounty   bool     `json:"bounty"`
	Domains  []string `json:"domains"`
}

func (a *Rix4uniAdapter) Fetch(ctx context.Context, st *storage.Storage, client *http.Client) (*fetcher.Result, error) {
	res := &fetcher.Result{Source: a.Name()}
	state, _ := st.GetSyncState(a.Name())
	body, etag, lastMod, notMod, err := fetcher.Download(ctx, client, rix4uniProgramsURL, state.LastETag, state.LastMod)
	if err != nil {
		return res, err
	}
	if notMod {
		return res, nil
	}
	res.ETag, res.LastModified = etag, lastMod
	var programs []rixProgram
	if err := json.Unmarshal(body, &programs); err != nil {
		var wrap struct {
			Programs []rixProgram `json:"programs"`
		}
		if err2 := json.Unmarshal(body, &wrap); err2 != nil {
			return res, fmt.Errorf("parse rix4uni: %w", err)
		}
		programs = wrap.Programs
	}
	for _, p := range programs {
		name := p.Name
		if name == "" {
			name = p.Handle
		}
		if name == "" {
			continue
		}
		handle := p.Handle
		if handle == "" {
			handle = sanitizeHandle(name)
		}
		raw, _ := json.Marshal(p)
		plat := p.Platform
		if plat == "" {
			plat = "rix4uni"
		}
		prog := &storage.Program{Source: a.Name(), Handle: handle, Name: name, URL: p.URL, OffersBounty: p.Bounty, Platform: plat, Layer: "program", RawJSON: string(raw)}
		progID, isNew, err := st.UpsertProgram(prog)
		if err != nil {
			continue
		}
		if isNew {
			res.ProgramsNew++
			ch := storage.Change{Source: a.Name(), Kind: "program_added", Entity: handle, Details: fmt.Sprintf(`{"name":%q}`, name)}
			_ = st.RecordChange(&ch)
			res.Changes = append(res.Changes, ch)
		}
		seen := map[string]struct{}{}
		for _, d := range p.Domains {
			d = strings.TrimSpace(d)
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
