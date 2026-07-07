package api

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/mods"
	"palpanel/internal/palconfig"
	"palpanel/internal/paldefender"
	"palpanel/internal/palrest"
	"palpanel/internal/server"
)

type Server struct {
	cfg      appconfig.Config
	store    *db.Store
	server   server.Manager
	mods     mods.Manager
	defender paldefender.Manager
	palrest  palrest.Client
}

func NewRouter(cfg appconfig.Config, store *db.Store, serverManager server.Manager, modsManager mods.Manager, defenderManager paldefender.Manager, restClient palrest.Client) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	s := Server{cfg: cfg, store: store, server: serverManager, mods: modsManager, defender: defenderManager, palrest: restClient}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(CORSMiddleware())
	r.GET("/api/health", s.health)

	api := r.Group("/api")
	api.Use(Auth(cfg.PanelToken))
	{
		api.GET("/jobs", s.listJobs)
		api.GET("/jobs/:id", s.getJob)

		api.GET("/server/status", s.serverStatus)
		api.GET("/server/prerequisites", s.serverPrerequisites)
		api.GET("/server/runtime", s.getRuntime)
		api.PUT("/server/runtime", s.putRuntime)
		api.POST("/server/bootstrap", s.serverBootstrap)
		api.GET("/server/logs", s.serverLogs)
		api.POST("/server/install", s.serverInstall)
		api.POST("/server/update", s.serverUpdate)
		api.POST("/server/start", s.serverStart)
		api.POST("/server/stop", s.serverStop)
		api.POST("/server/restart", s.serverRestart)
		api.GET("/server/startup", s.getStartup)
		api.PUT("/server/startup", s.putStartup)
		api.POST("/server/initialize-config", s.initializeConfig)
		api.POST("/server/backup", s.serverBackup)
		api.GET("/backups", s.listBackups)

		api.GET("/config/palworld", s.getPalworldConfig)
		api.PUT("/config/palworld", s.updatePalworldConfig)
		api.GET("/config/palworld/schema", s.getPalworldConfigSchema)
		api.POST("/config/palworld/validate", s.validatePalworldConfig)

		api.GET("/mods", s.listMods)
		api.POST("/mods/upload", s.uploadMod)
		api.POST("/mods/workshop", s.downloadWorkshop)
		api.POST("/mods/:id/enable", s.enableMod)
		api.POST("/mods/:id/disable", s.disableMod)
		api.DELETE("/mods/:id", s.deleteMod)

		api.GET("/security/paldefender/releases", s.palDefenderReleases)
		api.GET("/security/paldefender/status", s.palDefenderStatus)
		api.POST("/security/paldefender/install", s.palDefenderInstall)
		api.POST("/security/paldefender/update", s.palDefenderUpdate)
		api.POST("/security/paldefender/rollback", s.palDefenderRollback)
		api.GET("/security/paldefender/config", s.palDefenderGetConfig)
		api.PUT("/security/paldefender/config", s.palDefenderPutConfig)
		api.POST("/security/paldefender/apply-preset", s.palDefenderApplyPreset)
		api.POST("/security/paldefender/rest-token", s.palDefenderRESTToken)
		api.POST("/security/paldefender/reload-config", s.palDefenderReloadConfig)

		api.GET("/server/info", s.palGet("info"))
		api.GET("/server/players", s.palGet("players"))
		api.GET("/server/settings", s.palGet("settings"))
		api.GET("/server/metrics", s.palGet("metrics"))
		api.POST("/server/announce", s.palPost("announce"))
		api.POST("/server/save", s.palPost("save"))
		api.POST("/server/shutdown", s.palPost("shutdown"))
	}

	if _, err := os.Stat("D:/WL/me/pal/frontend/dist"); err == nil {
		r.Static("/assets", "D:/WL/me/pal/frontend/dist/assets")
		r.StaticFile("/", "D:/WL/me/pal/frontend/dist/index.html")
		r.StaticFile("/favicon.ico", "D:/WL/me/pal/frontend/dist/favicon.ico")
	}

	return r
}

func (s Server) health(c *gin.Context) {
	ok(c, gin.H{"status": "ok"})
}

func (s Server) listJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	jobs, err := s.store.ListJobs(c.Request.Context(), limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, "jobs_list_failed", err.Error())
		return
	}
	ok(c, jobs)
}

func (s Server) getJob(c *gin.Context) {
	j, err := s.store.GetJob(c.Request.Context(), c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		fail(c, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "job_read_failed", err.Error())
		return
	}
	ok(c, j)
}

func (s Server) serverStatus(c *gin.Context) {
	status, err := s.server.Status(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "server_status_failed", err.Error())
		return
	}
	ok(c, status)
}

func (s Server) serverPrerequisites(c *gin.Context) {
	checks, err := s.server.Prerequisites(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "prerequisites_failed", err.Error())
		return
	}
	ok(c, checks)
}

func (s Server) getRuntime(c *gin.Context) {
	mode, err := s.server.RuntimeMode(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "runtime_read_failed", err.Error())
		return
	}
	ok(c, gin.H{"mode": mode})
}

func (s Server) putRuntime(c *gin.Context) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := s.server.SetRuntimeMode(c.Request.Context(), req.Mode); err != nil {
		fail(c, http.StatusBadRequest, "runtime_write_failed", err.Error())
		return
	}
	ok(c, gin.H{"mode": req.Mode})
}

func (s Server) serverBootstrap(c *gin.Context) {
	j, err := s.server.Bootstrap(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "bootstrap_failed", err.Error())
		return
	}
	accepted(c, j)
}

func (s Server) serverLogs(c *gin.Context) {
	tail, _ := strconv.Atoi(c.DefaultQuery("tail", "200"))
	logs, err := s.server.Logs(c.Request.Context(), tail)
	if err != nil {
		fail(c, http.StatusInternalServerError, "server_logs_failed", err.Error())
		return
	}
	ok(c, gin.H{"logs": logs})
}

func (s Server) serverInstall(c *gin.Context) {
	j, err := s.server.Install(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "install_failed", err.Error())
		return
	}
	accepted(c, j)
}

func (s Server) serverUpdate(c *gin.Context) {
	j, err := s.server.Update(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	accepted(c, j)
}

func (s Server) serverStart(c *gin.Context) {
	if err := s.server.Start(c.Request.Context()); err != nil {
		fail(c, http.StatusInternalServerError, "start_failed", err.Error())
		return
	}
	ok(c, gin.H{"status": "started"})
}

func (s Server) serverStop(c *gin.Context) {
	if err := s.server.Stop(c.Request.Context()); err != nil {
		fail(c, http.StatusInternalServerError, "stop_failed", err.Error())
		return
	}
	ok(c, gin.H{"status": "stopped"})
}

func (s Server) serverRestart(c *gin.Context) {
	if err := s.server.Restart(c.Request.Context()); err != nil {
		fail(c, http.StatusInternalServerError, "restart_failed", err.Error())
		return
	}
	ok(c, gin.H{"status": "restarted"})
}

func (s Server) getStartup(c *gin.Context) {
	startup, err := s.server.StartupConfig(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "startup_read_failed", err.Error())
		return
	}
	ok(c, gin.H{"startup": startup, "args": startup.Args(s.cfg), "issues": startup.Validate()})
}

func (s Server) putStartup(c *gin.Context) {
	var startup server.StartupConfig
	if err := c.ShouldBindJSON(&startup); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	issues := startup.Validate()
	if hasServerValidationErrors(issues) {
		fail(c, http.StatusBadRequest, "startup_invalid", "startup config has validation errors")
		return
	}
	saved, err := s.server.SetStartupConfig(c.Request.Context(), startup)
	if err != nil {
		fail(c, http.StatusBadRequest, "startup_write_failed", err.Error())
		return
	}
	ok(c, gin.H{"startup": saved, "args": saved.Args(s.cfg), "issues": issues})
}

func (s Server) initializeConfig(c *gin.Context) {
	if err := s.server.InitializeConfig(c.Request.Context()); err != nil {
		fail(c, http.StatusInternalServerError, "config_init_failed", err.Error())
		return
	}
	ok(c, gin.H{"path": s.cfg.PalWorldSettingsPath()})
}

func (s Server) serverBackup(c *gin.Context) {
	j, err := s.server.Backup(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "backup_failed", err.Error())
		return
	}
	accepted(c, j)
}

func (s Server) listBackups(c *gin.Context) {
	backups, err := s.server.ListBackups()
	if err != nil {
		fail(c, http.StatusInternalServerError, "backups_list_failed", err.Error())
		return
	}
	ok(c, backups)
}

func (s Server) getPalworldConfig(c *gin.Context) {
	settings, err := palconfig.Read(s.cfg.PalWorldSettingsPath())
	if err != nil {
		fail(c, http.StatusInternalServerError, "config_read_failed", err.Error())
		return
	}
	ok(c, gin.H{"path": s.cfg.PalWorldSettingsPath(), "settings": settings, "issues": palconfig.Validate(settings)})
}

func (s Server) updatePalworldConfig(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	updates := raw
	if nested, found := raw["settings"].(map[string]any); found {
		updates = nested
	}
	current, err := palconfig.Read(s.cfg.PalWorldSettingsPath())
	if err != nil {
		fail(c, http.StatusInternalServerError, "config_read_failed", err.Error())
		return
	}
	next := palconfig.Merge(current, updates)
	if err := palconfig.Write(s.cfg.PalWorldSettingsPath(), next); err != nil {
		fail(c, http.StatusInternalServerError, "config_write_failed", err.Error())
		return
	}
	_ = s.server.MarkPendingRestart(c.Request.Context())
	ok(c, gin.H{"path": s.cfg.PalWorldSettingsPath(), "settings": next, "pending_restart": true, "issues": palconfig.Validate(next)})
}

func (s Server) getPalworldConfigSchema(c *gin.Context) {
	ok(c, gin.H{"version": "0.7.2", "fields": palconfig.Schema()})
}

func (s Server) validatePalworldConfig(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	updates := raw
	if nested, found := raw["settings"].(map[string]any); found {
		updates = nested
	}
	current, err := palconfig.Read(s.cfg.PalWorldSettingsPath())
	if err != nil {
		fail(c, http.StatusInternalServerError, "config_read_failed", err.Error())
		return
	}
	next := palconfig.Merge(current, updates)
	issues := palconfig.Validate(next)
	ok(c, gin.H{"issues": issues, "valid": !hasPalconfigErrors(issues)})
}

func (s Server) listMods(c *gin.Context) {
	list, err := s.mods.List(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "mods_list_failed", err.Error())
		return
	}
	ok(c, list)
}

func (s Server) uploadMod(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "upload_missing_file", err.Error())
		return
	}
	defer file.Close()
	enable := strings.EqualFold(c.PostForm("enable"), "true")
	mod, err := s.mods.UploadZip(c.Request.Context(), file, header.Filename, enable)
	if err != nil {
		fail(c, http.StatusBadRequest, "mod_upload_failed", err.Error())
		return
	}
	created(c, mod)
}

func (s Server) downloadWorkshop(c *gin.Context) {
	var req struct {
		ItemID string `json:"item_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	j, err := s.mods.DownloadWorkshop(c.Request.Context(), req.ItemID)
	if err != nil {
		fail(c, http.StatusBadRequest, "workshop_failed", err.Error())
		return
	}
	accepted(c, j)
}

func (s Server) enableMod(c *gin.Context) {
	mod, err := s.mods.SetEnabled(c.Request.Context(), c.Param("id"), true)
	if errors.Is(err, sql.ErrNoRows) {
		fail(c, http.StatusNotFound, "mod_not_found", "mod not found")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "mod_enable_failed", err.Error())
		return
	}
	ok(c, mod)
}

func (s Server) disableMod(c *gin.Context) {
	mod, err := s.mods.SetEnabled(c.Request.Context(), c.Param("id"), false)
	if errors.Is(err, sql.ErrNoRows) {
		fail(c, http.StatusNotFound, "mod_not_found", "mod not found")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "mod_disable_failed", err.Error())
		return
	}
	ok(c, mod)
}

func (s Server) deleteMod(c *gin.Context) {
	err := s.mods.Delete(c.Request.Context(), c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		fail(c, http.StatusNotFound, "mod_not_found", "mod not found")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "mod_delete_failed", err.Error())
		return
	}
	ok(c, gin.H{"deleted": true})
}

func (s Server) palDefenderReleases(c *gin.Context) {
	releases, err := s.defender.Releases(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadGateway, "paldefender_releases_failed", err.Error())
		return
	}
	ok(c, releases)
}

func (s Server) palDefenderStatus(c *gin.Context) {
	status, err := s.defender.Status(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "paldefender_status_failed", err.Error())
		return
	}
	ok(c, status)
}

func (s Server) palDefenderInstall(c *gin.Context) {
	j, err := s.defender.Install(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "paldefender_install_failed", err.Error())
		return
	}
	accepted(c, j)
}

func (s Server) palDefenderUpdate(c *gin.Context) {
	j, err := s.defender.Update(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "paldefender_update_failed", err.Error())
		return
	}
	accepted(c, j)
}

func (s Server) palDefenderRollback(c *gin.Context) {
	status, err := s.defender.Rollback(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadRequest, "paldefender_rollback_failed", err.Error())
		return
	}
	ok(c, status)
}

func (s Server) palDefenderGetConfig(c *gin.Context) {
	cfg, err := s.defender.ReadConfig()
	if err != nil {
		fail(c, http.StatusInternalServerError, "paldefender_config_read_failed", err.Error())
		return
	}
	ok(c, cfg)
}

func (s Server) palDefenderPutConfig(c *gin.Context) {
	var cfg map[string]any
	if err := c.ShouldBindJSON(&cfg); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	saved, err := s.defender.WriteConfig(cfg)
	if err != nil {
		fail(c, http.StatusInternalServerError, "paldefender_config_write_failed", err.Error())
		return
	}
	ok(c, saved)
}

func (s Server) palDefenderApplyPreset(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)
	cfg, err := s.defender.ApplyPreset(req.Name)
	if err != nil {
		fail(c, http.StatusBadRequest, "paldefender_preset_failed", err.Error())
		return
	}
	ok(c, cfg)
}

func (s Server) palDefenderRESTToken(c *gin.Context) {
	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}
	_ = c.ShouldBindJSON(&req)
	token, err := s.defender.CreateRESTToken(c.Request.Context(), req.Name, req.Permissions)
	if err != nil {
		fail(c, http.StatusInternalServerError, "paldefender_token_failed", err.Error())
		return
	}
	ok(c, token)
}

func (s Server) palDefenderReloadConfig(c *gin.Context) {
	if err := s.defender.ReloadConfig(c.Request.Context()); err != nil {
		fail(c, http.StatusBadGateway, "paldefender_reload_failed", err.Error())
		return
	}
	ok(c, gin.H{"reloaded": true})
}

func (s Server) palGet(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp, err := s.palrest.Do(c.Request.Context(), http.MethodGet, path, nil)
		if err != nil {
			fail(c, http.StatusBadGateway, "palworld_api_failed", err.Error())
			return
		}
		ok(c, resp)
	}
}

func (s Server) palPost(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var payload any
		if c.Request.ContentLength > 0 {
			if err := c.ShouldBindJSON(&payload); err != nil {
				fail(c, http.StatusBadRequest, "invalid_json", err.Error())
				return
			}
		}
		resp, err := s.palrest.Do(c.Request.Context(), http.MethodPost, path, payload)
		if err != nil {
			fail(c, http.StatusBadGateway, "palworld_api_failed", err.Error())
			return
		}
		ok(c, resp)
	}
}

func hasServerValidationErrors(issues []server.ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

func hasPalconfigErrors(issues []palconfig.ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
