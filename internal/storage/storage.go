package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// CurrentSchemaVersion is bumped when migrations change.
const CurrentSchemaVersion = 2

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
	Layer        string    `json:"layer"` // program | inventory
	ContentHash  string    `json:"content_hash,omitempty"`
	RawJSON      string    `json:"raw_json,omitempty"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	UpdatedAt    time.Time `json:"updated_at"`
	RemovedAt    *time.Time `json:"removed_at,omitempty"`
}

type Target struct {
	ID                    int64      `json:"id"`
	ProgramID             int64      `json:"program_id"`
	Source                string     `json:"source"`
	AssetIdentifier       string     `json:"asset_identifier"`
	AssetType             string     `json:"asset_type"`
	EligibleForBounty     bool       `json:"eligible_for_bounty"`
	EligibleForSubmission bool       `json:"eligible_for_submission"`
	Instruction           string     `json:"instruction,omitempty"`
	MaxSeverity           string     `json:"max_severity,omitempty"`
	InScope               bool       `json:"in_scope"`
	ContentHash           string     `json:"content_hash,omitempty"`
	RawJSON               string     `json:"raw_json,omitempty"`
	FirstSeen             time.Time  `json:"first_seen"`
	LastSeen              time.Time  `json:"last_seen"`
	UpdatedAt             time.Time  `json:"updated_at"`
	RemovedAt             *time.Time `json:"removed_at,omitempty"`
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

	// DSN pragmas applied at open time (modernc.org/sqlite)
	dsn := dbPath +
		"?_pragma=busy_timeout(10000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=temp_store(MEMORY)" +
		"&_pragma=wal_autocheckpoint(1000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Single writer — avoids SQLITE_BUSY storms under concurrent adapters
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &Storage{db: db}
	if err := s.applyPragmas(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragmas: %w", err)
	}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Storage) applyPragmas() error {
	pragmas := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA busy_timeout = 10000`,
		`PRAGMA temp_store = MEMORY`,
		`PRAGMA mmap_size = 268435456`,
		`PRAGMA cache_size = -65536`, // ~64MB
		`PRAGMA recursive_triggers = ON`,
	}
	for _, p := range pragmas {
		if _, err := s.db.Exec(p); err != nil {
			// non-fatal for unsupported pragmas on some builds
			continue
		}
	}
	return nil
}

func (s *Storage) Close() error {
	// Checkpoint WAL so the main file is consistent on disk
	_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return s.db.Close()
}

func (s *Storage) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS schema_version (
	version INTEGER NOT NULL,
	applied_at DATETIME NOT NULL
)`)
	if err != nil {
		return err
	}

	var ver int
	err = s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&ver)
	if err != nil {
		return err
	}

	if ver < 1 {
		if err := s.migrateV1(); err != nil {
			return err
		}
		if _, err := s.db.Exec(`INSERT INTO schema_version (version, applied_at) VALUES (1, ?)`, time.Now().UTC()); err != nil {
			return err
		}
		ver = 1
	}
	if ver < 2 {
		if err := s.migrateV2(); err != nil {
			return err
		}
		if _, err := s.db.Exec(`INSERT INTO schema_version (version, applied_at) VALUES (2, ?)`, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) migrateV1() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS programs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source TEXT NOT NULL,
	handle TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	url TEXT NOT NULL DEFAULT '',
	offers_bounty INTEGER NOT NULL DEFAULT 0,
	platform TEXT NOT NULL DEFAULT '',
	raw_json TEXT NOT NULL DEFAULT '',
	first_seen DATETIME NOT NULL,
	last_seen DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	UNIQUE(source, handle)
);
CREATE TABLE IF NOT EXISTS targets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	program_id INTEGER NOT NULL,
	source TEXT NOT NULL,
	asset_identifier TEXT NOT NULL,
	asset_type TEXT NOT NULL DEFAULT '',
	eligible_for_bounty INTEGER NOT NULL DEFAULT 0,
	eligible_for_submission INTEGER NOT NULL DEFAULT 1,
	instruction TEXT NOT NULL DEFAULT '',
	max_severity TEXT NOT NULL DEFAULT '',
	in_scope INTEGER NOT NULL DEFAULT 1,
	raw_json TEXT NOT NULL DEFAULT '',
	first_seen DATETIME NOT NULL,
	last_seen DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	UNIQUE(source, program_id, asset_identifier),
	FOREIGN KEY(program_id) REFERENCES programs(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS changes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source TEXT NOT NULL,
	kind TEXT NOT NULL,
	entity TEXT NOT NULL,
	details TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	notified INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS sync_state (
	source TEXT PRIMARY KEY,
	last_etag TEXT NOT NULL DEFAULT '',
	last_mod TEXT NOT NULL DEFAULT '',
	last_success DATETIME,
	last_error TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'idle'
);
CREATE INDEX IF NOT EXISTS idx_targets_source ON targets(source);
CREATE INDEX IF NOT EXISTS idx_targets_asset ON targets(asset_identifier);
CREATE INDEX IF NOT EXISTS idx_changes_notified ON changes(notified);
CREATE INDEX IF NOT EXISTS idx_changes_created ON changes(created_at);
`)
	return err
}

// migrateV2 adds content hashes, soft-delete, layer, and stronger indexes.
func (s *Storage) migrateV2() error {
	alters := []string{
		`ALTER TABLE programs ADD COLUMN content_hash TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE programs ADD COLUMN layer TEXT NOT NULL DEFAULT 'program'`,
		`ALTER TABLE programs ADD COLUMN removed_at DATETIME`,
		`ALTER TABLE targets ADD COLUMN content_hash TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE targets ADD COLUMN removed_at DATETIME`,
	}
	for _, q := range alters {
		if _, err := s.db.Exec(q); err != nil {
			// column may already exist on fresh installs that embedded v2 fields — ignore duplicate
			if !isDuplicateColumn(err) {
				// SQLite returns error if column exists; safe to continue for IF-style
				_ = err
			}
		}
	}
	_, err := s.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_programs_source_handle ON programs(source, handle);
CREATE INDEX IF NOT EXISTS idx_programs_layer ON programs(layer);
CREATE INDEX IF NOT EXISTS idx_programs_removed ON programs(removed_at);
CREATE INDEX IF NOT EXISTS idx_targets_source_asset ON targets(source, asset_identifier);
CREATE INDEX IF NOT EXISTS idx_targets_program ON targets(program_id);
CREATE INDEX IF NOT EXISTS idx_targets_inscope ON targets(in_scope);
CREATE INDEX IF NOT EXISTS idx_targets_removed ON targets(removed_at);
CREATE INDEX IF NOT EXISTS idx_changes_source_kind ON changes(source, kind);
CREATE INDEX IF NOT EXISTS idx_changes_notified_created ON changes(notified, created_at);
`)
	return err
}

func isDuplicateColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsFold(msg, "duplicate column") || containsFold(msg, "already exists")
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if equalFoldASCII(s[i:i+len(sub)], sub) {
					return true
				}
			}
			return false
		})())
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func HashContent(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:16]) // 32 hex chars — enough for change detection
}

// ---------- Sync State ----------

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
	last_etag=excluded.last_etag, last_mod=excluded.last_mod,
	last_success=excluded.last_success, last_error=excluded.last_error, status=excluded.status`,
		st.Source, st.LastETag, st.LastMod, nullTime(st.LastSuccess), st.LastError, st.Status)
	return err
}

// ---------- Programs ----------

func (s *Storage) UpsertProgram(p *Program) (int64, bool, error) {
	now := time.Now().UTC()
	if p.Layer == "" {
		p.Layer = "program"
	}
	if p.ContentHash == "" {
		p.ContentHash = HashContent(p.Name, p.URL, fmt.Sprintf("%v", p.OffersBounty), p.Platform, p.RawJSON)
	}

	var existingID int64
	var existingHash string
	err := s.db.QueryRow(
		`SELECT id, content_hash FROM programs WHERE source = ? AND handle = ?`,
		p.Source, p.Handle,
	).Scan(&existingID, &existingHash)

	if err == sql.ErrNoRows {
		res, err := s.db.Exec(`
INSERT INTO programs (source, handle, name, url, offers_bounty, platform, layer, content_hash, raw_json, first_seen, last_seen, updated_at, removed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
			p.Source, p.Handle, p.Name, p.URL, boolToInt(p.OffersBounty), p.Platform, p.Layer, p.ContentHash, p.RawJSON, now, now, now)
		if err != nil {
			return 0, false, err
		}
		id, _ := res.LastInsertId()
		return id, true, nil
	}
	if err != nil {
		return 0, false, err
	}

	// Touch last_seen; only bump updated_at / clear removed when content changes
	if existingHash != p.ContentHash {
		_, err = s.db.Exec(`
UPDATE programs SET name=?, url=?, offers_bounty=?, platform=?, layer=?, content_hash=?, raw_json=?,
	last_seen=?, updated_at=?, removed_at=NULL WHERE id=?`,
			p.Name, p.URL, boolToInt(p.OffersBounty), p.Platform, p.Layer, p.ContentHash, p.RawJSON, now, now, existingID)
	} else {
		_, err = s.db.Exec(`UPDATE programs SET last_seen=?, removed_at=NULL WHERE id=?`, now, existingID)
	}
	if err != nil {
		return 0, false, err
	}
	return existingID, false, nil
}

// ---------- Targets ----------

func (s *Storage) UpsertTarget(t *Target) (int64, bool, error) {
	now := time.Now().UTC()
	if t.ContentHash == "" {
		t.ContentHash = HashContent(t.AssetType, fmt.Sprintf("%v", t.EligibleForBounty),
			fmt.Sprintf("%v", t.EligibleForSubmission), t.Instruction, t.MaxSeverity,
			fmt.Sprintf("%v", t.InScope), t.RawJSON)
	}

	var existingID int64
	var existingHash string
	err := s.db.QueryRow(
		`SELECT id, content_hash FROM targets WHERE source = ? AND program_id = ? AND asset_identifier = ?`,
		t.Source, t.ProgramID, t.AssetIdentifier,
	).Scan(&existingID, &existingHash)

	if err == sql.ErrNoRows {
		res, err := s.db.Exec(`
INSERT INTO targets (program_id, source, asset_identifier, asset_type, eligible_for_bounty, eligible_for_submission,
	instruction, max_severity, in_scope, content_hash, raw_json, first_seen, last_seen, updated_at, removed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
			t.ProgramID, t.Source, t.AssetIdentifier, t.AssetType, boolToInt(t.EligibleForBounty),
			boolToInt(t.EligibleForSubmission), t.Instruction, t.MaxSeverity, boolToInt(t.InScope),
			t.ContentHash, t.RawJSON, now, now, now)
		if err != nil {
			return 0, false, err
		}
		id, _ := res.LastInsertId()
		return id, true, nil
	}
	if err != nil {
		return 0, false, err
	}

	if existingHash != t.ContentHash {
		_, err = s.db.Exec(`
UPDATE targets SET asset_type=?, eligible_for_bounty=?, eligible_for_submission=?, instruction=?,
	max_severity=?, in_scope=?, content_hash=?, raw_json=?, last_seen=?, updated_at=?, removed_at=NULL WHERE id=?`,
			t.AssetType, boolToInt(t.EligibleForBounty), boolToInt(t.EligibleForSubmission),
			t.Instruction, t.MaxSeverity, boolToInt(t.InScope), t.ContentHash, t.RawJSON, now, now, existingID)
	} else {
		_, err = s.db.Exec(`UPDATE targets SET last_seen=?, in_scope=1, removed_at=NULL WHERE id=?`, now, existingID)
	}
	if err != nil {
		return 0, false, err
	}
	return existingID, false, nil
}

// MarkMissingTargets soft-removes targets not seen in this sync cycle.
func (s *Storage) MarkMissingTargets(source string, seen map[string]struct{}, programID int64) error {
	rows, err := s.db.Query(`
SELECT id, asset_identifier FROM targets
WHERE source=? AND program_id=? AND in_scope=1 AND removed_at IS NULL`, source, programID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id    int64
		asset string
	}
	var missing []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.asset); err != nil {
			return err
		}
		if _, ok := seen[r.asset]; !ok {
			missing = append(missing, r)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, m := range missing {
		if _, err := s.db.Exec(`UPDATE targets SET in_scope=0, removed_at=?, updated_at=? WHERE id=?`, now, now, m.id); err != nil {
			return err
		}
		_ = s.RecordChange(&Change{
			Source:  source,
			Kind:    "target_removed",
			Entity:  m.asset,
			Details: fmt.Sprintf(`{"program_id":%d}`, programID),
		})
	}
	return nil
}

// ---------- Changes ----------

func (s *Storage) RecordChange(c *Change) error {
	_, err := s.db.Exec(
		`INSERT INTO changes (source, kind, entity, details, created_at, notified) VALUES (?, ?, ?, ?, ?, 0)`,
		c.Source, c.Kind, c.Entity, c.Details, time.Now().UTC(),
	)
	return err
}

func (s *Storage) UnnotifiedChanges(limit int) ([]Change, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.Query(`
SELECT id, source, kind, entity, details, created_at, notified
FROM changes WHERE notified = 0 ORDER BY id ASC LIMIT ?`, limit)
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
	return out, rows.Err()
}

func (s *Storage) MarkChangesNotified(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`UPDATE changes SET notified=1 WHERE id=?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------- Export ----------

func (s *Storage) ExportProgramsJSON(path string) error {
	rows, err := s.db.Query(`
SELECT id, source, handle, name, url, offers_bounty, platform, layer, first_seen, last_seen, updated_at
FROM programs WHERE removed_at IS NULL ORDER BY source, handle`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var list []Program
	for rows.Next() {
		var p Program
		var offers int
		var layer sql.NullString
		if err := rows.Scan(&p.ID, &p.Source, &p.Handle, &p.Name, &p.URL, &offers, &p.Platform, &layer, &p.FirstSeen, &p.LastSeen, &p.UpdatedAt); err != nil {
			return err
		}
		p.OffersBounty = offers == 1
		if layer.Valid {
			p.Layer = layer.String
		}
		list = append(list, p)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}

func (s *Storage) ExportTargetsJSON(path string) error {
	rows, err := s.db.Query(`
SELECT id, program_id, source, asset_identifier, asset_type, eligible_for_bounty,
	eligible_for_submission, instruction, max_severity, in_scope, first_seen, last_seen, updated_at
FROM targets WHERE removed_at IS NULL ORDER BY source, asset_identifier`)
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
	return atomicWrite(path, data)
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Storage) Stats() (programs, targets, changes int64, err error) {
	err = s.db.QueryRow(`SELECT COUNT(*) FROM programs WHERE removed_at IS NULL`).Scan(&programs)
	if err != nil {
		return
	}
	err = s.db.QueryRow(`SELECT COUNT(*) FROM targets WHERE removed_at IS NULL AND in_scope=1`).Scan(&targets)
	if err != nil {
		return
	}
	err = s.db.QueryRow(`SELECT COUNT(*) FROM changes WHERE notified=0`).Scan(&changes)
	return
}

// IntegrityCheck runs PRAGMA integrity_check.
func (s *Storage) IntegrityCheck() (string, error) {
	var result string
	err := s.db.QueryRow(`PRAGMA integrity_check`).Scan(&result)
	return result, err
}

func (s *Storage) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
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
