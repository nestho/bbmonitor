package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bbmonitor/bbmonitor/internal/fetcher"
	"github.com/bbmonitor/bbmonitor/internal/storage"
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
	Website        string `json:"website"`
	Targets        struct {
		InScope []struct {
			AssetIdentifier       string `json:"asset_identifier"`
			AssetType             string `json:"asset_type"`
			EligibleForBounty     bool   `json:"eligible_for_bounty"`
			EligibleForSubmission bool   `json:"eligible_for_submission"`
			Instruction           string `json:"instruction"`
			MaxSeverity           string `json:"max_severity"`
		} `json:"in_scope"`
		OutOfScope []struct {
			AssetIdentifier string `json:"asset_identifier"`
			AssetType       string `json:"asset_type"`
		} `json:"out_of_scope"`
	} `json:"targets"`
}

func (a *ArkadiytAdapter) Fetch(ctx context.Context, st *storage.Storage, client *http.Client) (*fetcher.Result, error) {
	res := &fetcher.Result{Source: a.Name()}
	state, err := st.GetSyncState(a.Name())
	if err != nil {
		return res, err
	}
	files := []struct {
		name     string
		platform string
	}{
		{"hackerone_data.json", "hackerone"},
		{"bugcrowd_data.json", "bugcrowd"},
		{"intigriti_data.json", "intigriti"},
		{"yeswehack_data.json", "yeswehack"},
		{"federacy_data.json", "federacy"},
	}
	var allChanges []storage.Change
	for _, f := range files {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}
		url := arkadiytBase + f.name
		body, etag, lastMod, notMod, err := fetcher.Download(ctx, client, url, state.LastETag, state.LastMod)
		if err != nil {
			continue
		}
		if notMod || len(body) == 0 {
			continue
		}
		res.ETag = etag
		res.LastModified = lastMod
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
			prog := &storage.Program{
				Source: a.Name(), Handle: handle, Name: p.Name, URL: p.URL,
				OffersBounty: p.OffersBounties, Platform: f.platform, RawJSON: string(raw),
			}
			progID, isNew, err := st.UpsertProgram(prog)
			if err != nil {
				continue
			}
			if isNew {
				res.ProgramsNew++
				ch := storage.Change{
					Source: a.Name(), Kind: "program_added", Entity: handle,
					Details: fmt.Sprintf(`{"name":%q,"platform":%q,"url":%q}`, p.Name, f.platform, p.URL),
				}
				_ = st.RecordChange(&ch)
				allChanges = append(allChanges, ch)
			}
			seen := make(map[string]struct{})
			for _, t := range p.Targets.InScope {
				if t.AssetIdentifier == "" {
					continue
				}
				seen[t.AssetIdentifier] = struct{}{}
				tRaw, _ := json.Marshal(t)
				target := &storage.Target{
					ProgramID: progID, Source: a.Name(), AssetIdentifier: t.AssetIdentifier,
					AssetType: t.AssetType, EligibleForBounty: t.EligibleForBounty,
					EligibleForSubmission: t.EligibleForSubmission, Instruction: t.Instruction,
					MaxSeverity: t.MaxSeverity, InScope: true, RawJSON: string(tRaw),
				}
				_, tNew, err := st.UpsertTarget(target)
				if err != nil {
					continue
				}
				if tNew {
					res.TargetsNew++
					ch := storage.Change{
						Source: a.Name(), Kind: "target_added", Entity: t.AssetIdentifier,
						Details: fmt.Sprintf(`{"program":%q,"type":%q,"eligible_bounty":%v}`, handle, t.AssetType, t.EligibleForBounty),
					}
					_ = st.RecordChange(&ch)
					allChanges = append(allChanges, ch)
				} else {
					res.TargetsUpd++
				}
			}
			_ = st.MarkMissingTargets(a.Name(), seen, progID)
		}
	}
	res.Changes = allChanges
	return res, nil
}
