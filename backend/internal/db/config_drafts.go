package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
)

type ConfigDraft struct {
	ID             string   `json:"id"`
	BaseSHA256     string   `json:"revision_sha256"`
	DraftPath      string   `json:"-"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	AppliedJobID   string   `json:"applied_job_id,omitempty"`
	ModifiedFields []string `json:"-"`
}

type ConfigPrivateCleanup struct {
	Path      string
	Kind      string
	Attempts  int
	LastError string
	CreatedAt string
	UpdatedAt string
}

func migrateConfigDrafts(ctx context.Context, tx *sql.Tx) error {
	return execAll(ctx, tx, `CREATE TABLE IF NOT EXISTS config_drafts (
		id TEXT PRIMARY KEY,
		base_sha256 TEXT NOT NULL,
		draft_path TEXT NOT NULL,
		status TEXT NOT NULL,
		applied_job_id TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
}

func migrateConfigDraftSafety(ctx context.Context, tx *sql.Tx) error {
	if err := ensureColumn(ctx, tx, "config_drafts", "modified_fields_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	return execAll(ctx, tx, `CREATE TABLE IF NOT EXISTS config_private_cleanup (
		path TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
}

func (s *Store) CreateConfigDraft(ctx context.Context, draft ConfigDraft) error {
	stamp := now()
	modified, err := encodeModifiedFields(draft.ModifiedFields)
	if err != nil {
		return err
	}
	draft.CreatedAt = stamp
	draft.UpdatedAt = stamp
	_, err = s.db.ExecContext(ctx, `INSERT INTO config_drafts
		(id,base_sha256,draft_path,status,applied_job_id,created_at,updated_at,modified_fields_json)
		VALUES (?,?,?,?,?,?,?,?)`, draft.ID, draft.BaseSHA256, draft.DraftPath, draft.Status, draft.AppliedJobID, draft.CreatedAt, draft.UpdatedAt, modified)
	return err
}

func (s *Store) CreateConfigDraftReplacing(ctx context.Context, draft ConfigDraft) ([]ConfigDraft, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	replaced, err := queryConfigDrafts(ctx, tx, `SELECT id,base_sha256,draft_path,status,applied_job_id,created_at,updated_at,modified_fields_json
		FROM config_drafts WHERE status IN ('draft','failed') ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	stamp := now()
	for _, previous := range replaced {
		if err := queueConfigPrivateCleanup(ctx, tx, previous.DraftPath, "config_draft"); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE config_drafts SET status='superseded',updated_at=? WHERE status IN ('draft','failed')`, stamp); err != nil {
		return nil, err
	}
	modified, err := encodeModifiedFields(draft.ModifiedFields)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO config_drafts
		(id,base_sha256,draft_path,status,applied_job_id,created_at,updated_at,modified_fields_json)
		VALUES (?,?,?,?,?,?,?,?)`, draft.ID, draft.BaseSHA256, draft.DraftPath, draft.Status, draft.AppliedJobID, stamp, stamp, modified); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return replaced, nil
}

func (s *Store) ExpireConfigDrafts(ctx context.Context, createdBefore string) ([]ConfigDraft, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	expired, err := queryConfigDrafts(ctx, tx, `SELECT id,base_sha256,draft_path,status,applied_job_id,created_at,updated_at,modified_fields_json
		FROM config_drafts WHERE status IN ('draft','failed') AND created_at<? ORDER BY created_at,id`, createdBefore)
	if err != nil {
		return nil, err
	}
	for _, draft := range expired {
		if err := queueConfigPrivateCleanup(ctx, tx, draft.DraftPath, "config_draft"); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE config_drafts SET status='expired',updated_at=?
		WHERE status IN ('draft','failed') AND created_at<?`, now(), createdBefore); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return expired, nil
}

type configDraftQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func queryConfigDrafts(ctx context.Context, q configDraftQuerier, query string, args ...any) ([]ConfigDraft, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var drafts []ConfigDraft
	for rows.Next() {
		var draft ConfigDraft
		var modified string
		if err := rows.Scan(&draft.ID, &draft.BaseSHA256, &draft.DraftPath, &draft.Status, &draft.AppliedJobID, &draft.CreatedAt, &draft.UpdatedAt, &modified); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(modified), &draft.ModifiedFields); err != nil {
			return nil, err
		}
		drafts = append(drafts, draft)
	}
	return drafts, rows.Err()
}

func (s *Store) GetConfigDraft(ctx context.Context, id string) (ConfigDraft, error) {
	var draft ConfigDraft
	var modified string
	err := s.db.QueryRowContext(ctx, `SELECT id,base_sha256,draft_path,status,applied_job_id,created_at,updated_at,modified_fields_json
		FROM config_drafts WHERE id=?`, id).Scan(&draft.ID, &draft.BaseSHA256, &draft.DraftPath, &draft.Status, &draft.AppliedJobID, &draft.CreatedAt, &draft.UpdatedAt, &modified)
	if err == nil {
		err = json.Unmarshal([]byte(modified), &draft.ModifiedFields)
	}
	return draft, err
}

func (s *Store) LatestConfigDraft(ctx context.Context) (ConfigDraft, error) {
	var draft ConfigDraft
	var modified string
	err := s.db.QueryRowContext(ctx, `SELECT id,base_sha256,draft_path,status,applied_job_id,created_at,updated_at,modified_fields_json
		FROM config_drafts ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&draft.ID, &draft.BaseSHA256, &draft.DraftPath, &draft.Status, &draft.AppliedJobID, &draft.CreatedAt, &draft.UpdatedAt, &modified)
	if err == nil {
		err = json.Unmarshal([]byte(modified), &draft.ModifiedFields)
	}
	return draft, err
}

func (s *Store) UpdateConfigDraftStatusAndQueueCleanup(ctx context.Context, id, status, jobID, kind string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var path string
	if err := tx.QueryRowContext(ctx, `SELECT draft_path FROM config_drafts WHERE id=?`, id).Scan(&path); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE config_drafts SET status=?,applied_job_id=?,updated_at=? WHERE id=?`, status, jobID, now(), id); err != nil {
		return err
	}
	if err := queueConfigPrivateCleanup(ctx, tx, path, kind); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) QueueConfigPrivateCleanup(ctx context.Context, path, kind string) error {
	return queueConfigPrivateCleanup(ctx, s.db, path, kind)
}

type configPrivateCleanupExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func queueConfigPrivateCleanup(ctx context.Context, execer configPrivateCleanupExecer, path, kind string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	stamp := now()
	_, err := execer.ExecContext(ctx, `INSERT INTO config_private_cleanup(path,kind,attempts,last_error,created_at,updated_at)
		VALUES(?,?,0,'',?,?) ON CONFLICT(path) DO UPDATE SET kind=excluded.kind,updated_at=excluded.updated_at`, path, kind, stamp, stamp)
	return err
}

func (s *Store) ListConfigPrivateCleanup(ctx context.Context, limit int) ([]ConfigPrivateCleanup, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT path,kind,attempts,last_error,created_at,updated_at FROM config_private_cleanup ORDER BY created_at,path LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ConfigPrivateCleanup
	for rows.Next() {
		var item ConfigPrivateCleanup
		if err := rows.Scan(&item.Path, &item.Kind, &item.Attempts, &item.LastError, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RecordConfigPrivateCleanupFailure(ctx context.Context, path, detail string) error {
	if len(detail) > 1024 {
		detail = detail[:1024]
	}
	_, err := s.db.ExecContext(ctx, `UPDATE config_private_cleanup SET attempts=attempts+1,last_error=?,updated_at=? WHERE path=?`, detail, now(), path)
	return err
}

func (s *Store) CompleteConfigPrivateCleanup(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM config_private_cleanup WHERE path=?`, path)
	return err
}

func encodeModifiedFields(fields []string) (string, error) {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" && !seen[field] {
			seen[field] = true
			normalized = append(normalized, field)
		}
	}
	sort.Strings(normalized)
	raw, err := json.Marshal(normalized)
	return string(raw), err
}

func (s *Store) UpdateConfigDraftStatus(ctx context.Context, id, status, jobID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE config_drafts SET status=?,applied_job_id=?,updated_at=? WHERE id=?`, status, jobID, now(), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ClaimConfigDraft(ctx context.Context, id, jobID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE config_drafts
		SET status='applying',applied_job_id=?,updated_at=?
		WHERE id=? AND status IN ('draft','failed')`, jobID, now(), id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
