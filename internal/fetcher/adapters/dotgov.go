package adapters

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/nestho/bbmonitor/internal/fetcher"
	"github.com/nestho/bbmonitor/internal/storage"
)

// Official US .gov domain inventory (federal + full). Updated frequently by CISA.
const (
	dotgovFederalURL = "https://raw.githubusercontent.com/cisagov/dotgov-data/main/current-federal.csv"
	dotgovFullURL    = "https://raw.githubusercontent.com/cisagov/dotgov-data/main/current-full.csv"
)

type DotgovAdapter struct{}

func NewDotgov() *DotgovAdapter { return &DotgovAdapter{} }

func (a *DotgovAdapter) Name() string { return "dotgov" }

func (a *DotgovAdapter) Fetch(ctx context.Context, st *storage.Storage, client *http.Client) (*fetcher.Result, error) {
	res := &fetcher.Result{Source: a.Name()}
	state, _ := st.GetSyncState(a.Name())

	// Single synthetic program for .gov inventory bucket
	prog := &storage.Program{
		Source: a.Name(), Handle: "dotgov", Name: "US .gov domains",
		URL: "https://get.gov", OffersBounty: false, Platform: "inventory",
		RawJSON: `{"kind":"dotgov-inventory"}`,
	}
	progID, isNew, err := st.UpsertProgram(prog)
	if err != nil {
		return res, err
	}
	if isNew {
		res.ProgramsNew++
	}

	seen := make(map[string]struct{})
	for _, url := range []string{dotgovFederalURL, dotgovFullURL} {
		body, etag, lastMod, notMod, err := fetcher.Download(ctx, client, url, state.LastETag, state.LastMod)
		if err != nil {
			continue
		}
		if notMod {
			continue
		}
		res.ETag = etag
		res.LastModified = lastMod

		r := csv.NewReader(strings.NewReader(string(body)))
		r.FieldsPerRecord = -1
		records, err := r.ReadAll()
		if err != nil || len(records) < 2 {
			continue
		}
		// header: Domain name, Domain type, ...
		for _, row := range records[1:] {
			if len(row) < 1 {
				continue
			}
			domain := strings.TrimSpace(strings.ToLower(row[0]))
			if domain == "" || domain == "domain name" {
				continue
			}
			if !strings.Contains(domain, ".") {
				domain = domain + ".gov"
			}
			seen[domain] = struct{}{}
			t := &storage.Target{
				ProgramID: progID, Source: a.Name(), AssetIdentifier: domain,
				AssetType: "DOMAIN", EligibleForBounty: false, EligibleForSubmission: false,
				InScope: true, RawJSON: fmt.Sprintf(`{"domain":%q}`, domain),
			}
			_, tNew, err := st.UpsertTarget(t)
			if err != nil {
				continue
			}
			if tNew {
				res.TargetsNew++
				if res.TargetsNew <= 50 {
					ch := storage.Change{
						Source: a.Name(), Kind: "target_added", Entity: domain,
						Details: `{"type":"dotgov"}`,
					}
					_ = st.RecordChange(&ch)
					res.Changes = append(res.Changes, ch)
				}
			}
		}
	}
	_ = st.MarkMissingTargets(a.Name(), seen, progID)
	return res, nil
}
