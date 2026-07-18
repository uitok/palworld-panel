package communityservers

import "time"

const (
	DefaultBaseURL     = "https://api.battlemetrics.com"
	DefaultFreshTTL    = 60 * time.Second
	DefaultStaleTTL    = 24 * time.Hour
	DefaultRateLimit   = 30
	DefaultPageSize    = 30
	MaximumPageSize    = 100
	DefaultCacheSource = "battlemetrics"
)

type Server struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	Port        int       `json:"port"`
	Connect     string    `json:"connect"`
	Players     int       `json:"players"`
	MaxPlayers  int       `json:"max_players"`
	Password    bool      `json:"password"`
	Country     string    `json:"country"`
	Version     string    `json:"version,omitempty"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type Query struct {
	Region     string `json:"region"`
	Search     string `json:"search,omitempty"`
	MinPlayers int    `json:"min_players,omitempty"`
	MaxPlayers int    `json:"max_players,omitempty"`
	Password   *bool  `json:"password,omitempty"`
	Version    string `json:"version,omitempty"`
	Status     string `json:"status"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

type Result struct {
	Servers         []Server  `json:"servers"`
	Total           int       `json:"total"`
	SourceTotal     int       `json:"source_total"`
	Page            int       `json:"page"`
	PageSize        int       `json:"page_size"`
	Source          string    `json:"source"`
	FetchedAt       time.Time `json:"fetched_at"`
	Stale           bool      `json:"stale"`
	CacheAgeSeconds int64     `json:"cache_age_seconds"`
}

type SourceStatus struct {
	Source          string     `json:"source"`
	Enabled         bool       `json:"enabled"`
	BaseURL         string     `json:"base_url"`
	ProxyConfigured bool       `json:"proxy_configured"`
	Reachable       bool       `json:"reachable"`
	CacheAvailable  bool       `json:"cache_available"`
	CacheFresh      bool       `json:"cache_fresh"`
	CacheWritable   bool       `json:"cache_writable"`
	CachedQueries   int        `json:"cached_queries"`
	LastAttemptAt   *time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt   *time.Time `json:"last_success_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	CacheError      string     `json:"cache_error,omitempty"`
	NextRefreshAt   *time.Time `json:"next_refresh_at,omitempty"`
	RateLimit       int        `json:"rate_limit_per_minute"`
}

func (q Query) Normalize() Query {
	if q.Region != "global" {
		q.Region = "cn"
	}
	q.Search = trimQuery(q.Search, 128)
	q.Version = trimQuery(q.Version, 64)
	if q.Status != "offline" && q.Status != "all" {
		q.Status = "online"
	}
	if q.MinPlayers < 0 {
		q.MinPlayers = 0
	}
	if q.MaxPlayers < 0 {
		q.MaxPlayers = 0
	}
	if q.MaxPlayers > 0 && q.MaxPlayers < q.MinPlayers {
		q.MaxPlayers = q.MinPlayers
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = DefaultPageSize
	}
	if q.PageSize > MaximumPageSize {
		q.PageSize = MaximumPageSize
	}
	return q
}
