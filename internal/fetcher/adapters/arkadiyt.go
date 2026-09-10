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

const arkadiytBase = "https://raw.githubusercontent.com/arkadiyt/bounty-targets-data/master/data/"

type ArkadiytAdapter struct{}

func NewArkadiyt() *ArkadiytAdapter { return &ArkadiytAdapter{} }
func (a *ArkadiytAdapter) Name() string { return "arkadiyt" }

type h1Program struct {
	Handle         string `json:"handle"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	OffersBounties bool   `json:"offers_bounties"`
	Targets        struct {
		InScope []struct {
			AssetIdentifier       string `json:"asset_identifier"`
			AssetType             string `json:"asset_type"`
			EligibleForBounty     bool   `json:"eligible_for_bounty"`
			EligibleForSubmission bool   `json:"eligible_for_submission"`
			Instruction           string `json:"instruction"`
			MaxSeverity           string `json:"max_severity"`
		} `json:"in_scope"`
	} `json:"targets"`
}

func (a *ArkadiytAdapter) Fetch(ctx context.Context, st *storage.Storage, client *http.Client) (*fetcher.Result, error) {
	res := &fetcher.Result{Source: a.Name()}
	state, _ := st.GetSyncState(a.Name())
	files := []struct{ name, platform string }{
		{"hackerone_data.json", "hackerone"}, {"bugcrowd_data.json", "bugcrowd"},
		{"intigriti_data.json", "intigriti"}, {"yeswehack_data.json", "yeswehack"}, {"federacy_data.json", "federacy"},
	}
	for _, f := range files {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}
		body, etag, lastMod, notMod, err := fetcher.Download(ctx, client, arkadiytBase+f.name, state.LastETag, state.LastMod)
		if err != nil || notMod || len(body) == 0 {
			continue
		}
		res.ETag, res.LastModified = etag, lastMod
		var programs []h1Program
		if err := json.Unmarshal(body, &programs); err != nil {
			return res, fmt.Errorf("parse %s: %w", f.name, err)
		}
		for _, p := range programs {
			handle := p.Handle
			if handle == "" {
				handle = strings.ToLower(strings.ReplaceAll(p.Name, " ", "_"))
			}
			if handle == "" {
				continue
			}
			raw, _ := json.Marshal(p)
			prog := &storage.Program{Source: a.Name(), Handle: handle, Name: p.Name, URL: p.URL, OffersBounty: p.OffersBounties, Platform: f.platform, Layer: "program", RawJSON: string(raw)}
			progID, isNew, err := st.UpsertProgram(prog)
			if err != nil {
				continue
			}
			if isNew {
				res.ProgramsNew++
				ch := storage.Change{Source: a.Name(), Kind: "program_added", Entity: handle, Details: fmt.Sprintf(`{"name":%q,"platform":%q}`, p.Name, f.platform)}
				_ = st.RecordChange(&ch)
				res.Changes = append(res.Changes, ch)
			}
			seen := map[string]struct{}{}
			for _, t := range p.Targets.InScope {
				if t.AssetIdentifier == "" {
					continue
				}
				seen[t.AssetIdentifier] = struct{}{}
				tRaw, _ := json.Marshal(t)
				target := &storage.Target{ProgramID: progID, Source: a.Name(), AssetIdentifier: t.AssetIdentifier, AssetType: t.AssetType, EligibleForBounty: t.EligibleForBounty, EligibleForSubmission: t.EligibleForSubmission, Instruction: t.Instruction, MaxSeverity: t.MaxSeverity, InScope: true, RawJSON: string(tRaw)}
				_, tNew, err := st.UpsertTarget(target)
				if err != nil {
					continue
				}
				if tNew {
					res.TargetsNew++
					ch := storage.Change{Source: a.Name(), Kind: "target_added", Entity: t.AssetIdentifier, Details: fmt.Sprintf(`{"program":%q}`, handle)}
					_ = st.RecordChange(&ch)
					res.Changes = append(res.Changes, ch)
				}
			}
			_ = st.MarkMissingTargets(a.Name(), seen, progID)
		}
	}
	return res, nil
}
