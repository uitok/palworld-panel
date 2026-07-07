package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Job struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Mod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	PackageName string `json:"package_name"`
	Path        string `json:"path"`
	Version     string `json:"version,omitempty"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type AuditLog struct {
	ID        string `json:"id"`
	Actor     string `json:"actor"`
	Role      string `json:"role"`
	Action    string `json:"action"`
	Target    string `json:"target,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	IP        string `json:"ip,omitempty"`
	CreatedAt string `json:"created_at"`
}

type PlayerAccessEntry struct {
	SteamID   string `json:"steam_id"`
	Nickname  string `json:"nickname,omitempty"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func Open(path string) (*Store, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)
	s := &Store{db: d}
	if err := s.Migrate(context.Background()); err != nil {
		_ = d.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			progress INTEGER NOT NULL DEFAULT 0,
			message TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS mods (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			source TEXT NOT NULL,
			package_name TEXT NOT NULL,
			path TEXT NOT NULL,
			version TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS kv (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			actor TEXT NOT NULL,
			role TEXT NOT NULL,
			action TEXT NOT NULL,
			target TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			message TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS player_access (
			list_type TEXT NOT NULL,
			steam_id TEXT NOT NULL,
			nickname TEXT NOT NULL DEFAULT '',
			reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (list_type, steam_id)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateAuditLog(ctx context.Context, log AuditLog) error {
	if log.ID == "" {
		return errors.New("audit log id is required")
	}
	if log.CreatedAt == "" {
		log.CreatedAt = now()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_logs (id,actor,role,action,target,status,message,ip,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		log.ID, log.Actor, log.Role, log.Action, log.Target, log.Status, log.Message, log.IP, log.CreatedAt)
	return err
}

func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,actor,role,action,target,status,message,ip,created_at FROM audit_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditLog
	for rows.Next() {
		var item AuditLog
		if err := rows.Scan(&item.ID, &item.Actor, &item.Role, &item.Action, &item.Target, &item.Status, &item.Message, &item.IP, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListPlayerAccess(ctx context.Context, listType string) ([]PlayerAccessEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT steam_id,nickname,reason,created_at,updated_at FROM player_access WHERE list_type=? ORDER BY updated_at DESC`, listType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlayerAccessEntry
	for rows.Next() {
		var item PlayerAccessEntry
		if err := rows.Scan(&item.SteamID, &item.Nickname, &item.Reason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpsertPlayerAccess(ctx context.Context, listType string, item PlayerAccessEntry) error {
	if item.SteamID == "" {
		return errors.New("steam_id is required")
	}
	now := now()
	if item.CreatedAt == "" {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO player_access (list_type,steam_id,nickname,reason,created_at,updated_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(list_type, steam_id) DO UPDATE SET nickname=excluded.nickname, reason=excluded.reason, updated_at=excluded.updated_at`,
		listType, item.SteamID, item.Nickname, item.Reason, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) DeletePlayerAccess(ctx context.Context, listType, steamID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM player_access WHERE list_type=? AND steam_id=?`, listType, steamID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ReplacePlayerAccess(ctx context.Context, listType string, items []PlayerAccessEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM player_access WHERE list_type=?`, listType); err != nil {
		_ = tx.Rollback()
		return err
	}
	now := now()
	for _, item := range items {
		if item.SteamID == "" {
			_ = tx.Rollback()
			return errors.New("steam_id is required")
		}
		if item.CreatedAt == "" {
			item.CreatedAt = now
		}
		item.UpdatedAt = now
		if _, err := tx.ExecContext(ctx, `INSERT INTO player_access (list_type,steam_id,nickname,reason,created_at,updated_at) VALUES (?,?,?,?,?,?)`,
			listType, item.SteamID, item.Nickname, item.Reason, item.CreatedAt, item.UpdatedAt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) CreateJob(ctx context.Context, id, typ, message string) (Job, error) {
	now := now()
	j := Job{ID: id, Type: typ, Status: "queued", Progress: 0, Message: message, CreatedAt: now, UpdatedAt: now}
	_, err := s.db.ExecContext(ctx, `INSERT INTO jobs (id,type,status,progress,message,error,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		j.ID, j.Type, j.Status, j.Progress, j.Message, j.Error, j.CreatedAt, j.UpdatedAt)
	return j, err
}

func (s *Store) UpdateJob(ctx context.Context, id, status string, progress int, message, errText string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET status=?, progress=?, message=?, error=?, updated_at=? WHERE id=?`,
		status, progress, message, errText, now(), id)
	return err
}

func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,type,status,progress,message,error,created_at,updated_at FROM jobs WHERE id=?`, id)
	var j Job
	err := row.Scan(&j.ID, &j.Type, &j.Status, &j.Progress, &j.Message, &j.Error, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

func (s *Store) ListJobs(ctx context.Context, limit int) ([]Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,type,status,progress,message,error,created_at,updated_at FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Type, &j.Status, &j.Progress, &j.Message, &j.Error, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (s *Store) UpsertMod(ctx context.Context, m Mod) error {
	now := now()
	if m.CreatedAt == "" {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO mods (id,name,source,package_name,path,version,enabled,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, source=excluded.source, package_name=excluded.package_name,
		path=excluded.path, version=excluded.version, enabled=excluded.enabled, updated_at=excluded.updated_at`,
		m.ID, m.Name, m.Source, m.PackageName, m.Path, m.Version, boolInt(m.Enabled), m.CreatedAt, m.UpdatedAt)
	return err
}

func (s *Store) ListMods(ctx context.Context) ([]Mod, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,source,package_name,path,version,enabled,created_at,updated_at FROM mods ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mods []Mod
	for rows.Next() {
		var m Mod
		var enabled int
		if err := rows.Scan(&m.ID, &m.Name, &m.Source, &m.PackageName, &m.Path, &m.Version, &enabled, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Enabled = enabled == 1
		mods = append(mods, m)
	}
	return mods, rows.Err()
}

func (s *Store) GetMod(ctx context.Context, id string) (Mod, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,source,package_name,path,version,enabled,created_at,updated_at FROM mods WHERE id=?`, id)
	var m Mod
	var enabled int
	err := row.Scan(&m.ID, &m.Name, &m.Source, &m.PackageName, &m.Path, &m.Version, &enabled, &m.CreatedAt, &m.UpdatedAt)
	m.Enabled = enabled == 1
	return m, err
}

func (s *Store) SetModEnabled(ctx context.Context, id string, enabled bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE mods SET enabled=?, updated_at=? WHERE id=?`, boolInt(enabled), now(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteMod(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM mods WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SetKV(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO kv (key,value,updated_at) VALUES (?,?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, key, value, now())
	return err
}

func (s *Store) GetKV(ctx context.Context, key string) (string, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT value FROM kv WHERE key=?`, key)
	var v string
	err := row.Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
