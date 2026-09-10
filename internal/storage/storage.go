package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sql.DB
}

type Program struct {
	ID           int64     `json:"id"`
	Source       string    `json:"source"`
	Handle       string    `json:"handle"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	OffersBounty bool      `json:"offers_bounty"`
	Platform     string    `json:"platform"`
	RawJSON      string    `json:"raw_json,omitempty"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Target struct {
	ID                    int64     `json:"id"`
	ProgramID             int64     `json:"program_id"`
	Source                string    `json:"source"`
	AssetIdentifier       string    `json:"asset_identifier"`
	AssetType             string    `json:"asset_type"`
	EligibleForBounty     bool      `json:"eligible_for_bounty"`
	EligibleForSubmission bool      `json:"eligible_for_submission"`
	Instruction           string    `json:"instruction,omitempty"`
	MaxSeverity           string    `json:"max_severity,omitempty"`
	InScope               bool      `json:"in_scope"`
	RawJSON               string    `json:"raw_json,omitempty"`
	FirstSeen             time.Time `json:"first_seen"`
	LastSeen              time.Time `json:"last_seen"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type Change struct {
	ID        int64     `json:"id"`
	Source    string    `json:"source"`
	Kind      string    `json:"kind"`
	Entity    string    `json:"entity"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
	Notified  bool      `json:"notified"`
}

type SyncState struct {
	Source      string
	LastETag    string
	LastMod     string
	LastSuccess time.Time
	LastError   string
	Status      string
}

func Open(dbPath string) (*Storage, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Storage) Close() error { return s.db.Close() }

func (s *Storage) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS programs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source TEXT NOT NULL, handle TEXT NOT NULL, name TEXT NOT NULL DEFAULT '',
	url TEXT NOT NULL DEFAULT '', offers_bounty INTEGER NOT NULL DEFAULT 0,
	platform TEXT NOT NULL DEFAULT '', raw_json TEXT NOT NULL DEFAULT '',
	first_seen DATETIME NOT NULL, last_seen DATETIME NOT NULL, updated_at DATETIME NOT NULL,
	UNIQUE(source, handle)
);
CREATE TABLE IF NOT EXISTS targets (
	id INTEGER PRIMARY KEY AUTOINCREMENT, program_id INTEGER NOT NULL, source TEXT NOT NULL,
	asset_identifier TEXT NOT NULL, asset_type TEXT NOT NULL DEFAULT '',
	eligible_for_bounty INTEGER NOT NULL DEFAULT 0, eligible_for_submission INTEGER NOT NULL DEFAULT 1,
	instruction TEXT NOT NULL DEFAULT '', max_severity TEXT NOT NULL DEFAULT '',
	in_scope INTEGER NOT NULL DEFAULT 1, raw_json TEXT NOT NULL DEFAULT '',
	first_seen DATETIME NOT NULL, last_seen DATETIME NOT NULL, updated_at DATETIME NOT NULL,
	UNIQUE(source, program_id, asset_identifier),
	FOREIGN KEY(program_id) REFERENCES programs(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS changes (
	id INTEGER PRIMARY KEY AUTOINCREMENT, source TEXT NOT NULL, kind TEXT NOT NULL,
	entity TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL, notified INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS sync_state (
	source TEXT PRIMARY KEY, last_etag TEXT NOT NULL DEFAULT '', last_mod TEXT NOT NULL DEFAULT '',
	last_success DATETIME, last_error TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'idle'
);
CREATE INDEX IF NOT EXISTS idx_targets_source ON targets(source);
CREATE INDEX IF NOT EXISTS idx_targets_asset ON targets(asset_identifier);
CREATE INDEX IF NOT EXISTS idx_changes_notified ON changes(notified);
CREATE INDEX IF NOT EXISTS idx_changes_created ON changes(created_at);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Storage) GetSyncState(source string) (*SyncState, error) {
	row := s.db.QueryRow(`SELECT source, last_etag, last_mod, last_success, last_error, status FROM sync_state WHERE source = ?`, source)
	var st SyncState
	var lastSuccess sql.NullTime
	err := row.Scan(&st.Source, &st.LastETag, &st.LastMod, &lastSuccess, &st.LastError, &st.Status)
	if err == sql.ErrNoRows {
		return &SyncState{Source: source, Status: "idle"}, nil
	}
	if err != nil {
		return nil, err
	}
	if lastSuccess.Valid {
		st.LastSuccess = lastSuccess.Time
	}
	return &st, nil
}

func (s *Storage) UpsertSyncState(st *SyncState) error {
	_, err := s.db.Exec(`
INSERT INTO sync_state (source, last_etag, last_mod, last_success, last_error, status)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(source) DO UPDATE SET
	last_etag=excluded.last_etag, last_mod=excluded.last_mod, last_success=excluded.last_success,
	last_error=excluded.last_error, status=excluded.status`,
		st.Source, st.LastETag, st.LastMod, nullTime(st.LastSuccess), st.LastError, st.Status)
	return err
}

func (s *Storage) UpsertProgram(p *Program) (int64, bool, error) {
	now := time.Now().UTC()
	var existingID int64
	var existingRaw string
	err := s.db.QueryRow(`SELECT id, raw_json FROM programs WHERE source = ? AND handle = ?`, p.Source, p.Handle).Scan(&existingID, &existingRaw)
	if err == sql.ErrNoRows {
		res, err := s.db.Exec(`
INSERT INTO programs (source, handle, name, url, offers_bounty, platform, raw_json, first_seen, last_seen, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Source, p.Handle, p.Name, p.URL, boolToInt(p.OffersBounty), p.Platform, p.RawJSON, now, now, now)
		if err != nil {
			return 0, false, err
		}
		id, _ := res.LastInsertId()
		return id, true, nil
	}
	if err != nil {
		return 0, false, err
	}
	_, err = s.db.Exec(`UPDATE programs SET name=?, url=?, offers_bounty=?, platform=?, raw_json=?, last_seen=?, updated_at=? WHERE id=?`,
		p.Name, p.URL, boolToInt(p.OffersBounty), p.Platform, p.RawJSON, now, now, existingID)
	if err != nil {
		return 0, false, err
	}
	_ = existingRaw
	return existingID, false, nil
}

func (s *Storage) UpsertTarget(t *Target) (int64, bool, error) {
	now := time.Now().UTC()
	var existingID int64
	var existingRaw string
	err := s.db.QueryRow(`SELECT id, raw_json FROM targets WHERE source = ? AND program_id = ? AND asset_identifier = ?`,
		t.Source, t.ProgramID, t.AssetIdentifier).Scan(&existingID, &existingRaw)
	if err == sql.ErrNoRows {
		res, err := s.db.Exec(`
INSERT INTO targets (program_id, source, asset_identifier, asset_type, eligible_for_bounty, eligible_for_submission,
	instruction, max_severity, in_scope, raw_json, first_seen, last_seen, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ProgramID, t.Source, t.AssetIdentifier, t.AssetType, boolToInt(t.EligibleForBounty),
			boolToInt(t.EligibleForSubmission), t.Instruction, t.MaxSeverity, boolToInt(t.InScope),
			t.RawJSON, now, now, now)
		if err != nil {
			return 0, false, err
		}
		id, _ := res.LastInsertId()
		return id, true, nil
	}
	if err != nil {
		return 0, false, err
	}
	_, err = s.db.Exec(`UPDATE targets SET asset_type=?, eligible_for_bounty=?, eligible_for_submission=?, instruction=?,
	max_severity=?, in_scope=?, raw_json=?, last_seen=?, updated_at=? WHERE id=?`,
		t.AssetType, boolToInt(t.EligibleForBounty), boolToInt(t.EligibleForSubmission),
		t.Instruction, t.MaxSeverity, boolToInt(t.InScope), t.RawJSON, now, now, existingID)
	if err != nil {
		return 0, false, err
	}
	return existingID, false, nil
}

func (s *Storage) MarkMissingTargets(source string, seen map[string]struct{}, programID int64) error {
	rows, err := s.db.Query(`SELECT id, asset_identifier FROM targets WHERE source=? AND program_id=? AND in_scope=1`, source, programID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type missing struct {
		id    int64
		asset string
	}
	var toRemove []missing
	for rows.Next() {
		var id int64
		var asset string
		if err := rows.Scan(&id, &asset); err != nil {
			return err
		}
		if _, ok := seen[asset]; !ok {
			toRemove = append(toRemove, missing{id, asset})
		}
	}
	now := time.Now().UTC()
	for _, m := range toRemove {
		if _, err := s.db.Exec(`UPDATE targets SET in_scope=0, updated_at=? WHERE id=?`, now, m.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) RecordChange(c *Change) error {
	_, err := s.db.Exec(`INSERT INTO changes (source, kind, entity, details, created_at, notified) VALUES (?, ?, ?, ?, ?, 0)`,
		c.Source, c.Kind, c.Entity, c.Details, time.Now().UTC())
	return err
}

func (s *Storage) UnnotifiedChanges(limit int) ([]Change, error) {
	rows, err := s.db.Query(`SELECT id, source, kind, entity, details, created_at, notified FROM changes WHERE notified = 0 ORDER BY created_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Change
	for rows.Next() {
		var c Change
		var notified int
		if err := rows.Scan(&c.ID, &c.Source, &c.Kind, &c.Entity, &c.Details, &c.CreatedAt, &notified); err != nil {
			return nil, err
		}
		c.Notified = notified == 1
		out = append(out, c)
	}
	return out, nil
}

func (s *Storage) MarkChangesNotified(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`UPDATE changes SET notified=1 WHERE id=?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) ExportProgramsJSON(path string) error {
	rows, err := s.db.Query(`SELECT id, source, handle, name, url, offers_bounty, platform, first_seen, last_seen, updated_at FROM programs ORDER BY source, handle`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var list []Program
	for rows.Next() {
		var p Program
		var offers int
		if err := rows.Scan(&p.ID, &p.Source, &p.Handle, &p.Name, &p.URL, &offers, &p.Platform, &p.FirstSeen, &p.LastSeen, &p.UpdatedAt); err != nil {
			return err
		}
		p.OffersBounty = offers == 1
		list = append(list, p)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *Storage) ExportTargetsJSON(path string) error {
	rows, err := s.db.Query(`SELECT t.id, t.program_id, t.source, t.asset_identifier, t.asset_type, t.eligible_for_bounty,
	t.eligible_for_submission, t.instruction, t.max_severity, t.in_scope, t.first_seen, t.last_seen, t.updated_at FROM targets t ORDER BY t.source, t.asset_identifier`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var list []Target
	for rows.Next() {
		var t Target
		var eb, es, is int
		if err := rows.Scan(&t.ID, &t.ProgramID, &t.Source, &t.AssetIdentifier, &t.AssetType, &eb, &es, &t.Instruction, &t.MaxSeverity, &is, &t.FirstSeen, &t.LastSeen, &t.UpdatedAt); err != nil {
			return err
		}
		t.EligibleForBounty = eb == 1
		t.EligibleForSubmission = es == 1
		t.InScope = is == 1
		list = append(list, t)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *Storage) Stats() (programs, targets, changes int64, err error) {
	err = s.db.QueryRow(`SELECT COUNT(*) FROM programs`).Scan(&programs)
	if err != nil {
		return
	}
	err = s.db.QueryRow(`SELECT COUNT(*) FROM targets`).Scan(&targets)
	if err != nil {
		return
	}
	err = s.db.QueryRow(`SELECT COUNT(*) FROM changes WHERE notified=0`).Scan(&changes)
	return
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

func (s *Storage) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
