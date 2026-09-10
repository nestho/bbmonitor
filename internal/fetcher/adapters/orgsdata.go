package adapters

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bbmonitor/bbmonitor/internal/fetcher"
	"github.com/bbmonitor/bbmonitor/internal/storage"
)

var orgsDataFiles = []string{
	"hackerone.tsv", "hackerone.external_program.tsv", "bugcrowd.tsv", "intigriti.tsv",
	"yeswehack.tsv", "immunefi.tsv", "diodb.tsv", "chaos.tsv", "federacy.tsv", "hackenproof.tsv",
}

const orgsDataBase = "https://raw.githubusercontent.com/nikitastupin/orgs-data/main/orgs-data/"

type OrgsDataAdapter struct{}

func NewOrgsData() *OrgsDataAdapter { return &OrgsDataAdapter{} }

func (a *OrgsDataAdapter) Name() string { return "orgsdata" }

func (a *OrgsDataAdapter) Fetch(ctx context.Context, st *storage.Storage, client *http.Client) (*fetcher.Result, error) {
	res := &fetcher.Result{Source: a.Name()}
	state, _ := st.GetSyncState(a.Name())
	for _, fname := range orgsDataFiles {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}
		url := orgsDataBase + fname
		body, etag, lastMod, notMod, err := fetcher.Download(ctx, client, url, state.LastETag, state.LastMod)
		if err != nil {
			continue
		}
		if notMod {
			continue
		}
		res.ETag = etag
		res.LastModified = lastMod
		for _, line := range strings.Split(string(body), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) < 2 {
				continue
			}
			progURL := strings.TrimSpace(parts[0])
			orgVal := strings.TrimSpace(parts[1])
			if progURL == "" {
				continue
			}
			handle := sanitizeHandle(progURL)
			if strings.Contains(progURL, "/") {
				segs := strings.Split(strings.TrimRight(progURL, "/"), "/")
				handle = sanitizeHandle(segs[len(segs)-1])
			}
			raw := fmt.Sprintf(`{"program_url":%q,"github":%q,"file":%q}`, progURL, orgVal, fname)
			prog := &storage.Program{
				Source: a.Name(), Handle: handle, Name: handle, URL: progURL,
				OffersBounty: false, Platform: "orgs-data", RawJSON: raw,
			}
			progID, isNew, err := st.UpsertProgram(prog)
			if err != nil {
				continue
			}
			if isNew {
				res.ProgramsNew++
				ch := storage.Change{Source: a.Name(), Kind: "program_added", Entity: handle, Details: raw}
				_ = st.RecordChange(&ch)
				res.Changes = append(res.Changes, ch)
			}
			if strings.HasPrefix(orgVal, "https://github.com/") || (orgVal != "?" && orgVal != "-" && !strings.Contains(orgVal, "://")) {
				asset := orgVal
				if strings.HasPrefix(asset, "https://github.com/") {
					asset = strings.Trim(strings.TrimPrefix(asset, "https://github.com/"), "/")
				}
				if asset == "" {
					continue
				}
				t := &storage.Target{
					ProgramID: progID, Source: a.Name(), AssetIdentifier: asset, AssetType: "GITHUB_ORG",
					EligibleForBounty: false, EligibleForSubmission: true, InScope: true, RawJSON: raw,
				}
				_, tNew, err := st.UpsertTarget(t)
				if err != nil {
					continue
				}
				if tNew {
					res.TargetsNew++
					ch := storage.Change{Source: a.Name(), Kind: "target_added", Entity: asset, Details: fmt.Sprintf(`{"program":%q,"type":"GITHUB_ORG"}`, handle)}
					_ = st.RecordChange(&ch)
					res.Changes = append(res.Changes, ch)
				}
			}
		}
	}
	return res, nil
}
