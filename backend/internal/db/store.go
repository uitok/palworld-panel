package db

import (
	"context"
	"database/sql"
	"encoding/json"
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
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Source        string   `json:"source"`
	PackageName   string   `json:"package_name"`
	Path          string   `json:"path"`
	Version       string   `json:"version,omitempty"`
	Enabled       bool     `json:"enabled"`
	WorkshopID    string   `json:"workshop_id,omitempty"`
	PreviewURL    string   `json:"preview_url,omitempty"`
	SteamURL      string   `json:"steam_url,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	FileSize      int64    `json:"file_size,omitempty"`
	Subscriptions int64    `json:"subscriptions,omitempty"`
	TimeUpdated   int64    `json:"time_updated,omitempty"`
	LastCheckedAt string   `json:"last_checked_at,omitempty"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
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

type MonitorSample struct {
	ID                string  `json:"id"`
	CreatedAt         string  `json:"created_at"`
	CPUAvailable      bool    `json:"cpu_available"`
	CPUPercent        float64 `json:"cpu_percent"`
	MemoryAvailable   bool    `json:"memory_available"`
	MemoryUsageBytes  int64   `json:"memory_usage_bytes"`
	MemoryLimitBytes  int64   `json:"memory_limit_bytes"`
	DiskAvailable     bool    `json:"disk_available"`
	DiskFreeBytes     int64   `json:"disk_free_bytes"`
	DiskTotalBytes    int64   `json:"disk_total_bytes"`
	CurrentPlayers    int     `json:"current_players"`
	MaxPlayers        int     `json:"max_players"`
	RESTHealthy       bool    `json:"rest_healthy"`
	RCONHealthy       bool    `json:"rcon_healthy"`
	GamePortHealthy   bool    `json:"game_port_healthy"`
	QueryPortHealthy  bool    `json:"query_port_healthy"`
	UnavailableReason string  `json:"unavailable_reason,omitempty"`
}

type Schedule struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	TimeOfDay       string `json:"time_of_day,omitempty"`
	WaitTime        int    `json:"waittime,omitempty"`
	Message         string `json:"message,omitempty"`
	LastRunAt       string `json:"last_run_at,omitempty"`
	NextRunAt       string `json:"next_run_at,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type Alert struct {
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Source    string `json:"source"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	AckAt     string `json:"ack_at,omitempty"`
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
			workshop_id TEXT NOT NULL DEFAULT '',
			preview_url TEXT NOT NULL DEFAULT '',
			steam_url TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL DEFAULT '',
			tags_json TEXT NOT NULL DEFAULT '[]',
			file_size INTEGER NOT NULL DEFAULT 0,
			subscriptions INTEGER NOT NULL DEFAULT 0,
			time_updated INTEGER NOT NULL DEFAULT 0,
			last_checked_at TEXT NOT NULL DEFAULT '',
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
		`CREATE TABLE IF NOT EXISTS monitor_samples (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			cpu_available INTEGER NOT NULL DEFAULT 0,
			cpu_percent REAL NOT NULL DEFAULT 0,
			memory_available INTEGER NOT NULL DEFAULT 0,
			memory_usage_bytes INTEGER NOT NULL DEFAULT 0,
			memory_limit_bytes INTEGER NOT NULL DEFAULT 0,
			disk_available INTEGER NOT NULL DEFAULT 0,
			disk_free_bytes INTEGER NOT NULL DEFAULT 0,
			disk_total_bytes INTEGER NOT NULL DEFAULT 0,
			current_players INTEGER NOT NULL DEFAULT 0,
			max_players INTEGER NOT NULL DEFAULT 0,
			rest_healthy INTEGER NOT NULL DEFAULT 0,
			rcon_healthy INTEGER NOT NULL DEFAULT 0,
			game_port_healthy INTEGER NOT NULL DEFAULT 0,
			query_port_healthy INTEGER NOT NULL DEFAULT 0,
			unavailable_reason TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS schedules (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			interval_minutes INTEGER NOT NULL DEFAULT 0,
			time_of_day TEXT NOT NULL DEFAULT '',
			waittime INTEGER NOT NULL DEFAULT 30,
			message TEXT NOT NULL DEFAULT '',
			last_run_at TEXT NOT NULL DEFAULT '',
			next_run_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id TEXT PRIMARY KEY,
			severity TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			source TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			ack_at TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "mods", "workshop_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "mods", "preview_url", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "mods", "steam_url", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "mods", "summary", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "mods", "tags_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "mods", "file_size", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "mods", "subscriptions", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "mods", "time_updated", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "mods", "last_checked_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_mods_workshop_id ON mods(workshop_id)`); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
	return err
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

func (s *Store) InsertMonitorSample(ctx context.Context, sample MonitorSample) error {
	if sample.ID == "" {
		return errors.New("monitor sample id is required")
	}
	if sample.CreatedAt == "" {
		sample.CreatedAt = now()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO monitor_samples (
		id,created_at,cpu_available,cpu_percent,memory_available,memory_usage_bytes,memory_limit_bytes,
		disk_available,disk_free_bytes,disk_total_bytes,current_players,max_players,rest_healthy,rcon_healthy,
		game_port_healthy,query_port_healthy,unavailable_reason
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sample.ID, sample.CreatedAt, boolInt(sample.CPUAvailable), sample.CPUPercent,
		boolInt(sample.MemoryAvailable), sample.MemoryUsageBytes, sample.MemoryLimitBytes,
		boolInt(sample.DiskAvailable), sample.DiskFreeBytes, sample.DiskTotalBytes,
		sample.CurrentPlayers, sample.MaxPlayers, boolInt(sample.RESTHealthy), boolInt(sample.RCONHealthy),
		boolInt(sample.GamePortHealthy), boolInt(sample.QueryPortHealthy), sample.UnavailableReason)
	return err
}

func (s *Store) ListMonitorSamples(ctx context.Context, limit int) ([]MonitorSample, error) {
	if limit <= 0 || limit > 1000 {
		limit = 120
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,cpu_available,cpu_percent,memory_available,memory_usage_bytes,memory_limit_bytes,
		disk_available,disk_free_bytes,disk_total_bytes,current_players,max_players,rest_healthy,rcon_healthy,
		game_port_healthy,query_port_healthy,unavailable_reason
		FROM monitor_samples ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonitorSample
	for rows.Next() {
		var item MonitorSample
		var cpuAvailable, memoryAvailable, diskAvailable, restHealthy, rconHealthy, gameHealthy, queryHealthy int
		if err := rows.Scan(&item.ID, &item.CreatedAt, &cpuAvailable, &item.CPUPercent, &memoryAvailable,
			&item.MemoryUsageBytes, &item.MemoryLimitBytes, &diskAvailable, &item.DiskFreeBytes,
			&item.DiskTotalBytes, &item.CurrentPlayers, &item.MaxPlayers, &restHealthy, &rconHealthy,
			&gameHealthy, &queryHealthy, &item.UnavailableReason); err != nil {
			return nil, err
		}
		item.CPUAvailable = cpuAvailable == 1
		item.MemoryAvailable = memoryAvailable == 1
		item.DiskAvailable = diskAvailable == 1
		item.RESTHealthy = restHealthy == 1
		item.RCONHealthy = rconHealthy == 1
		item.GamePortHealthy = gameHealthy == 1
		item.QueryPortHealthy = queryHealthy == 1
		out = append(out, item)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (s *Store) UpsertSchedule(ctx context.Context, item Schedule) error {
	if item.ID == "" {
		return errors.New("schedule id is required")
	}
	if item.Type == "" {
		return errors.New("schedule type is required")
	}
	now := now()
	if item.CreatedAt == "" {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO schedules (id,type,enabled,interval_minutes,time_of_day,waittime,message,last_run_at,next_run_at,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET type=excluded.type, enabled=excluded.enabled, interval_minutes=excluded.interval_minutes,
		time_of_day=excluded.time_of_day, waittime=excluded.waittime, message=excluded.message, last_run_at=excluded.last_run_at,
		next_run_at=excluded.next_run_at, updated_at=excluded.updated_at`,
		item.ID, item.Type, boolInt(item.Enabled), item.IntervalMinutes, item.TimeOfDay, item.WaitTime,
		item.Message, item.LastRunAt, item.NextRunAt, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) ListSchedules(ctx context.Context) ([]Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,type,enabled,interval_minutes,time_of_day,waittime,message,last_run_at,next_run_at,created_at,updated_at
		FROM schedules ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Schedule
	for rows.Next() {
		var item Schedule
		var enabled int
		if err := rows.Scan(&item.ID, &item.Type, &enabled, &item.IntervalMinutes, &item.TimeOfDay, &item.WaitTime,
			&item.Message, &item.LastRunAt, &item.NextRunAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled == 1
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetSchedule(ctx context.Context, id string) (Schedule, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,type,enabled,interval_minutes,time_of_day,waittime,message,last_run_at,next_run_at,created_at,updated_at
		FROM schedules WHERE id=?`, id)
	var item Schedule
	var enabled int
	err := row.Scan(&item.ID, &item.Type, &enabled, &item.IntervalMinutes, &item.TimeOfDay, &item.WaitTime,
		&item.Message, &item.LastRunAt, &item.NextRunAt, &item.CreatedAt, &item.UpdatedAt)
	item.Enabled = enabled == 1
	return item, err
}

func (s *Store) DeleteSchedule(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM schedules WHERE id=?`, id)
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

func (s *Store) CreateAlert(ctx context.Context, item Alert) error {
	if item.ID == "" {
		return errors.New("alert id is required")
	}
	if item.CreatedAt == "" {
		item.CreatedAt = now()
	}
	if item.Status == "" {
		item.Status = "open"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO alerts (id,severity,title,message,source,status,created_at,ack_at) VALUES (?,?,?,?,?,?,?,?)`,
		item.ID, item.Severity, item.Title, item.Message, item.Source, item.Status, item.CreatedAt, item.AckAt)
	return err
}

func (s *Store) ListAlerts(ctx context.Context, limit int) ([]Alert, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,severity,title,message,source,status,created_at,ack_at FROM alerts ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		var item Alert
		if err := rows.Scan(&item.ID, &item.Severity, &item.Title, &item.Message, &item.Source, &item.Status, &item.CreatedAt, &item.AckAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) AckAlert(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE alerts SET status='acked', ack_at=? WHERE id=?`, now(), id)
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
	tagsJSON := encodeTags(m.Tags)
	_, err := s.db.ExecContext(ctx, `INSERT INTO mods (
			id,name,source,package_name,path,version,enabled,workshop_id,preview_url,steam_url,summary,tags_json,
			file_size,subscriptions,time_updated,last_checked_at,created_at,updated_at
		)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, source=excluded.source, package_name=excluded.package_name,
		path=excluded.path, version=excluded.version, enabled=excluded.enabled, workshop_id=excluded.workshop_id,
		preview_url=excluded.preview_url, steam_url=excluded.steam_url, summary=excluded.summary, tags_json=excluded.tags_json,
		file_size=excluded.file_size, subscriptions=excluded.subscriptions, time_updated=excluded.time_updated,
		last_checked_at=excluded.last_checked_at, updated_at=excluded.updated_at`,
		m.ID, m.Name, m.Source, m.PackageName, m.Path, m.Version, boolInt(m.Enabled), m.WorkshopID, m.PreviewURL, m.SteamURL,
		m.Summary, tagsJSON, m.FileSize, m.Subscriptions, m.TimeUpdated, m.LastCheckedAt, m.CreatedAt, m.UpdatedAt)
	return err
}

func (s *Store) ListMods(ctx context.Context) ([]Mod, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,source,package_name,path,version,enabled,workshop_id,preview_url,steam_url,summary,tags_json,file_size,subscriptions,time_updated,last_checked_at,created_at,updated_at FROM mods ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mods []Mod
	for rows.Next() {
		var m Mod
		var enabled int
		var tagsJSON string
		if err := rows.Scan(
			&m.ID, &m.Name, &m.Source, &m.PackageName, &m.Path, &m.Version, &enabled, &m.WorkshopID, &m.PreviewURL,
			&m.SteamURL, &m.Summary, &tagsJSON, &m.FileSize, &m.Subscriptions, &m.TimeUpdated, &m.LastCheckedAt,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		m.Enabled = enabled == 1
		m.Tags = decodeTags(tagsJSON)
		mods = append(mods, m)
	}
	return mods, rows.Err()
}

func (s *Store) GetMod(ctx context.Context, id string) (Mod, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,source,package_name,path,version,enabled,workshop_id,preview_url,steam_url,summary,tags_json,file_size,subscriptions,time_updated,last_checked_at,created_at,updated_at FROM mods WHERE id=?`, id)
	var m Mod
	var enabled int
	var tagsJSON string
	err := row.Scan(
		&m.ID, &m.Name, &m.Source, &m.PackageName, &m.Path, &m.Version, &enabled, &m.WorkshopID, &m.PreviewURL,
		&m.SteamURL, &m.Summary, &tagsJSON, &m.FileSize, &m.Subscriptions, &m.TimeUpdated, &m.LastCheckedAt,
		&m.CreatedAt, &m.UpdatedAt,
	)
	m.Enabled = enabled == 1
	m.Tags = decodeTags(tagsJSON)
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

func encodeTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	data, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func decodeTags(value string) []string {
	if value == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(value), &tags); err != nil {
		return nil
	}
	return tags
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
