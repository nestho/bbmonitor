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

const diodbURL = "https://raw.githubusercontent.com/disclose/diodb/master/program-list.json"

type DiodbAdapter struct{}

func NewDiodb() *DiodbAdapter { return &DiodbAdapter{} }
func (a *DiodbAdapter) Name() string { return "diodb" }

type diodbEntry struct {
	ProgramName  string `json:"program_name"`
	PolicyURL    string `json:"policy_url"`
	OffersBounty string `json:"offers_bounty"`
}

func (a *DiodbAdapter) Fetch(ctx context.Context, st *storage.Storage, client *http.Client) (*fetcher.Result, error) {
	res := &fetcher.Result{Source: a.Name()}
	state, _ := st.GetSyncState(a.Name())
	body, etag, lastMod, notMod, err := fetcher.Download(ctx, client, diodbURL, state.LastETag, state.LastMod)
	if err != nil {
		return res, err
	}
	if notMod {
		return res, nil
	}
	res.ETag, res.LastModified = etag, lastMod
	var entries []diodbEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return res, fmt.Errorf("parse diodb: %w", err)
	}
	for _, e := range entries {
		if e.ProgramName == "" {
			continue
		}
		handle := sanitizeHandle(e.ProgramName)
		offers := strings.ToLower(e.OffersBounty) == "yes" || strings.ToLower(e.OffersBounty) == "partial"
		raw, _ := json.Marshal(e)
		prog := &storage.Program{Source: a.Name(), Handle: handle, Name: e.ProgramName, URL: e.PolicyURL, OffersBounty: offers, Platform: "disclose", Layer: "program", RawJSON: string(raw)}
		_, isNew, err := st.UpsertProgram(prog)
		if err != nil {
			continue
		}
		if isNew {
			res.ProgramsNew++
			ch := storage.Change{Source: a.Name(), Kind: "program_added", Entity: handle, Details: fmt.Sprintf(`{"name":%q}`, e.ProgramName)}
			_ = st.RecordChange(&ch)
			res.Changes = append(res.Changes, ch)
		}
	}
	return res, nil
}
