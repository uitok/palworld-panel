package steamcmd

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/id"
	"palpanel/internal/networkproxy"
)

const (
	defaultCommandTimeout  = 60 * time.Minute
	defaultDownloadTimeout = 5 * time.Minute
	maxCommandOutputBytes  = 1 << 20
	maxArchiveEntries      = 10_000
	maxExtractedBytes      = 1 << 30
)

type commandRunner func(context.Context, string, string, ...string) ([]byte, error)

// Client owns all native SteamCMD setup and command execution. Commands that
// share an installation directory are serialized, including across clients.
type Client struct {
	cfg          appconfig.Config
	httpClient   *http.Client
	goos         string
	timeout      time.Duration
	runCommand   commandRunner
	now          func() time.Time
	network      *networkproxy.Service
	credentialMu sync.Mutex
	loginMu      sync.Mutex
	login        loginState
}

type commandGate struct {
	token chan struct{}
}

var commandGates sync.Map

func New(cfg appconfig.Config) *Client {
	client := &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: defaultDownloadTimeout},
		goos:       runtime.GOOS,
		timeout:    defaultCommandTimeout,
		runCommand: runCommand,
		now:        time.Now,
		network:    networkproxy.New(cfg),
	}
	return client
}

func (c *Client) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.httpClient = client
	}
}

func (c *Client) Ensure(ctx context.Context) error {
	if err := c.validateInstalled(); err == nil {
		return nil
	}
	if c.goos != "windows" {
		return fmt.Errorf("native SteamCMD requires a Windows host")
	}
	release, err := c.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := c.validateInstalled(); err == nil {
		return nil
	}

	if err := c.validateManaged(c.cfg.SteamCMDDir); err != nil {
		return fmt.Errorf("validate SteamCMD directory: %w", err)
	}
	stageParent := strings.TrimSpace(c.cfg.ToolsDir)
	if stageParent == "" {
		stageParent = filepath.Join(filepath.Dir(c.cfg.SteamCMDDir), ".palpanel-steamcmd-staging")
	}
	if err := c.validateManaged(stageParent); err != nil {
		return fmt.Errorf("validate SteamCMD staging directory: %w", err)
	}
	if err := os.MkdirAll(stageParent, 0o755); err != nil {
		return fmt.Errorf("create SteamCMD staging directory: %w", err)
	}
	stageRoot, err := os.MkdirTemp(stageParent, "steamcmd-stage-")
	if err != nil {
		return err
	}
	defer func() { _ = c.removeManagedDirectory(stageRoot) }()
	if err := c.validateManaged(stageRoot); err != nil {
		return err
	}

	archivePath := filepath.Join(stageRoot, "steamcmd.zip")
	if err := c.downloadWithRetry(ctx, archivePath); err != nil {
		return err
	}
	extracted := filepath.Join(stageRoot, "install")
	if err := extractZip(archivePath, extracted, c.validateManaged); err != nil {
		return fmt.Errorf("extract SteamCMD: %w", err)
	}
	if err := ValidatePEExecutable(filepath.Join(extracted, "steamcmd.exe")); err != nil {
		return fmt.Errorf("verify downloaded SteamCMD: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(c.cfg.SteamCMDDir), 0o755); err != nil {
		return fmt.Errorf("create SteamCMD parent directory: %w", err)
	}

	backup := c.cfg.SteamCMDDir + ".palpanel-backup-" + id.New("swap")
	if err := c.validateManaged(backup); err != nil {
		return err
	}
	if err := c.validateManaged(c.cfg.SteamCMDDir); err != nil {
		return err
	}
	hadPrevious := false
	if _, err := os.Stat(c.cfg.SteamCMDDir); err == nil {
		hadPrevious = true
		if err := c.validateManaged(c.cfg.SteamCMDDir); err != nil {
			return err
		}
		if err := os.Rename(c.cfg.SteamCMDDir, backup); err != nil {
			return fmt.Errorf("stage previous SteamCMD: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	rollback := func() {
		_ = c.removeManagedDirectory(c.cfg.SteamCMDDir)
		if hadPrevious {
			_ = os.Rename(backup, c.cfg.SteamCMDDir)
		}
	}
	if err := c.validateManaged(extracted); err != nil {
		rollback()
		return err
	}
	if err := os.Rename(extracted, c.cfg.SteamCMDDir); err != nil {
		rollback()
		return fmt.Errorf("activate SteamCMD: %w", err)
	}
	if err := ValidatePEExecutable(c.cfg.SteamCMDBinaryPath()); err != nil {
		rollback()
		return fmt.Errorf("verify installed SteamCMD: %w", err)
	}
	if hadPrevious {
		if err := c.removeManagedDirectory(backup); err != nil {
			log.Printf("SteamCMD installed; retained previous directory %s: %v", backup, err)
		}
	}
	return nil
}

func (c *Client) InstallOrUpdate(ctx context.Context, appID, destination string) error {
	if err := validateNumericID("Steam app", appID); err != nil {
		return err
	}
	if err := c.validateManaged(destination); err != nil {
		return fmt.Errorf("validate Steam app destination: %w", err)
	}
	if err := c.Ensure(ctx); err != nil {
		return err
	}
	if err := c.validateManaged(destination); err != nil {
		return fmt.Errorf("revalidate Steam app destination: %w", err)
	}
	_, err := c.executeValidated(ctx, func() error {
		if err := c.validateManaged(destination); err != nil {
			return fmt.Errorf("revalidate Steam app destination before command: %w", err)
		}
		return nil
	},
		"+force_install_dir", destination,
		"+login", "anonymous",
		"+app_update", appID, "validate",
		"+quit",
	)
	if err != nil {
		return fmt.Errorf("SteamCMD app_update failed: %w", err)
	}
	return nil
}

func (c *Client) AppInfo(ctx context.Context, appID string) (string, error) {
	if err := validateNumericID("Steam app", appID); err != nil {
		return "", err
	}
	if err := c.Ensure(ctx); err != nil {
		return "", err
	}
	out, err := c.execute(ctx,
		"+login", "anonymous",
		"+app_info_update", "1",
		"+app_info_print", appID,
		"+quit",
	)
	if err != nil {
		return "", fmt.Errorf("SteamCMD app_info_print failed: %w", err)
	}
	return string(out), nil
}

// DownloadWorkshopTo downloads into a unique staging tree and only exposes
// destination/itemID after the command and filesystem verification succeed.
func (c *Client) DownloadWorkshopTo(ctx context.Context, appID, itemID, destination string) error {
	c.credentialMu.Lock()
	defer c.credentialMu.Unlock()
	if err := validateNumericID("Workshop app", appID); err != nil {
		return err
	}
	if err := validateNumericID("Workshop item", itemID); err != nil {
		return err
	}
	if err := c.validateManaged(destination); err != nil {
		return fmt.Errorf("validate Workshop staging directory: %w", err)
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return fmt.Errorf("create Workshop staging directory: %w", err)
	}
	if err := c.validateManaged(destination); err != nil {
		return fmt.Errorf("revalidate Workshop staging directory: %w", err)
	}
	if err := c.Ensure(ctx); err != nil {
		return err
	}
	credentials, err := c.explicitCredentials()
	if err != nil {
		return err
	}
	if err := c.validateManaged(destination); err != nil {
		return fmt.Errorf("revalidate Workshop staging directory before download: %w", err)
	}

	stageRoot, err := os.MkdirTemp(destination, ".steamcmd-workshop-")
	if err != nil {
		return fmt.Errorf("create Workshop command staging directory: %w", err)
	}
	defer func() { _ = c.removeManagedDirectory(stageRoot) }()
	if err := c.validateManaged(stageRoot); err != nil {
		return err
	}

	request := LoginRequest{AccountName: credentials.AccountName, Password: credentials.Password}
	commands := []string{
		"force_install_dir " + steamScriptArg(stageRoot),
		"workshop_download_item " + appID + " " + itemID + " validate",
	}
	out, runErr := c.runExplicitLoginOutput(ctx, request, commands)
	if runErr != nil {
		c.setSteamGuardRequired(errors.Is(runErr, ErrSteamGuardRequired))
		if errors.Is(runErr, ErrSteamGuardRequired) || errors.Is(runErr, ErrInvalidCredentials) {
			_ = c.markCredentialsUnverified(ctx)
		}
		return runErr
	}
	c.setSteamGuardRequired(false)
	if err := workshopCommandFailure(out, itemID, credentials.AccountName, credentials.Password); err != nil {
		return err
	}

	source := filepath.Join(stageRoot, "steamapps", "workshop", "content", appID, itemID)
	if err := c.validateDownloadedTree(source); err != nil {
		detail := sanitizeOutput(string(out), credentials.AccountName, credentials.Password)
		if detail != "" {
			return fmt.Errorf("SteamCMD did not produce a complete Workshop item: %w; command output: %s", err, detail)
		}
		return fmt.Errorf("SteamCMD did not produce a complete Workshop item: %w", err)
	}
	if err := c.activateWorkshopItem(source, filepath.Join(destination, itemID)); err != nil {
		return err
	}
	return nil
}

func (c *Client) execute(ctx context.Context, args ...string) ([]byte, error) {
	return c.executeValidated(ctx, nil, args...)
}

func (c *Client) executeRedacted(ctx context.Context, secrets []string, args ...string) ([]byte, error) {
	return c.executeValidatedRedacted(ctx, nil, secrets, args...)
}

func (c *Client) executeValidated(ctx context.Context, validate func() error, args ...string) ([]byte, error) {
	return c.executeValidatedRedacted(ctx, validate, nil, args...)
}

func (c *Client) executeValidatedRedacted(ctx context.Context, validate func() error, secrets []string, args ...string) ([]byte, error) {
	release, err := c.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := c.validateInstalled(); err != nil {
		return nil, fmt.Errorf("revalidate SteamCMD before command: %w", err)
	}
	if validate != nil {
		if err := validate(); err != nil {
			return nil, err
		}
	}
	commandCtx, cancel := c.commandContext(ctx)
	defer cancel()
	out, runErr := c.runConfiguredCommand(commandCtx, args...)
	if runErr != nil {
		return out, c.commandError(commandCtx, runErr, out, secrets...)
	}
	if err := commandCtx.Err(); err != nil {
		return out, fmt.Errorf("SteamCMD command interrupted: %w", err)
	}
	return out, nil
}

func (c *Client) commandContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := c.timeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

func (c *Client) runConfiguredCommand(ctx context.Context, args ...string) ([]byte, error) {
	run := func() ([]byte, error) {
		return c.runCommand(ctx, c.cfg.SteamCMDBinaryPath(), c.cfg.SteamCMDDir, args...)
	}
	if c.network == nil {
		return run()
	}
	rawProxy, err := c.network.InstallProxyURL()
	if err != nil {
		return nil, err
	}
	if rawProxy == "" {
		return run()
	}
	return withSteamCMDProxy(ctx, rawProxy, c.cfg.SteamCMDProxyRestorePath(), run)
}

func (c *Client) commandError(ctx context.Context, runErr error, output []byte, secrets ...string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("SteamCMD command interrupted: %w", err)
	}
	detail := sanitizeOutput(string(output), secrets...)
	if detail == "" {
		return runErr
	}
	return fmt.Errorf("%w: %s", runErr, detail)
}

func (c *Client) activateWorkshopItem(source, destination string) error {
	if err := c.validateManaged(source); err != nil {
		return err
	}
	if err := c.validateManaged(destination); err != nil {
		return err
	}
	parent := filepath.Dir(destination)
	activation, err := os.MkdirTemp(parent, ".palpanel-workshop-activate-")
	if err != nil {
		return fmt.Errorf("create Workshop activation path: %w", err)
	}
	if err := c.removeManagedDirectory(activation); err != nil {
		return err
	}
	if err := os.Rename(source, activation); err != nil {
		return fmt.Errorf("stage downloaded Workshop item: %w", err)
	}
	activationOwned := true
	defer func() {
		if activationOwned {
			_ = c.removeManagedDirectory(activation)
		}
	}()
	if err := c.validateDownloadedTree(activation); err != nil {
		return fmt.Errorf("verify staged Workshop item: %w", err)
	}

	backup := destination + ".palpanel-backup-" + id.New("swap")
	if err := c.validateManaged(backup); err != nil {
		return err
	}
	hadPrevious := false
	if _, err := os.Lstat(destination); err == nil {
		hadPrevious = true
		if err := c.validateManaged(destination); err != nil {
			return err
		}
		if err := os.Rename(destination, backup); err != nil {
			return fmt.Errorf("stage previous Workshop item: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	rollback := func() {
		_ = c.removeManagedDirectory(destination)
		if hadPrevious {
			_ = os.Rename(backup, destination)
		}
	}
	if err := c.validateManaged(activation); err != nil {
		rollback()
		return err
	}
	if err := os.Rename(activation, destination); err != nil {
		rollback()
		return fmt.Errorf("activate Workshop item: %w", err)
	}
	activationOwned = false
	if err := c.validateDownloadedTree(destination); err != nil {
		rollback()
		return fmt.Errorf("verify activated Workshop item: %w", err)
	}
	if hadPrevious {
		if err := c.removeManagedDirectory(backup); err != nil {
			log.Printf("Workshop item activated; retained previous directory %s: %v", backup, err)
		}
	}
	return nil
}

func (c *Client) validateDownloadedTree(root string) error {
	if err := c.validateManaged(root); err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("downloaded path is not a regular directory: %s", root)
	}
	files := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := c.validateManaged(path); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("downloaded item contains a link or reparse point: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("downloaded item contains unsupported file type: %s", path)
		}
		files++
		if files > 200_000 {
			return fmt.Errorf("downloaded item contains too many files")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if files == 0 {
		return fmt.Errorf("downloaded item contains no regular files")
	}
	return nil
}

func (c *Client) downloadWithRetry(ctx context.Context, destination string) error {
	var failures []string
	for attempt := 1; attempt <= 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		_ = os.Remove(destination)
		err := c.download(ctx, destination)
		if err == nil {
			return nil
		}
		failures = append(failures, fmt.Sprintf("attempt %d: %v", attempt, err))
		if attempt < 3 {
			timer := time.NewTimer(time.Duration(attempt) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return fmt.Errorf("download SteamCMD failed after 3 attempts: %s", strings.Join(failures, "; "))
}

func (c *Client) download(ctx context.Context, destination string) error {
	if err := c.validateManaged(destination); err != nil {
		return err
	}
	downloadURL := strings.TrimSpace(c.cfg.SteamCMDDownloadURL)
	if downloadURL == "" {
		downloadURL = appconfig.DefaultSteamCMDDownloadURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultDownloadTimeout}
	}
	if c.network != nil {
		rawProxy, err := c.network.InstallProxyURL()
		if err != nil {
			return err
		}
		if rawProxy != "" {
			client, err = networkproxy.HTTPClient(client, rawProxy, defaultDownloadTimeout)
			if err != nil {
				return err
			}
		}
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}
	limit := c.cfg.SteamCMDDownloadMaxBytes
	if limit <= 0 {
		limit = int64(appconfig.DefaultSteamCMDDownloadMaxMB) << 20
	}
	if response.ContentLength > limit {
		return fmt.Errorf("download Content-Length %d exceeds %d byte limit", response.ContentLength, limit)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".steamcmd-download-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	complete := false
	defer func() {
		_ = temporary.Close()
		if !complete {
			_ = os.Remove(temporaryPath)
		}
	}()
	written, copyErr := io.Copy(temporary, io.LimitReader(response.Body, limit+1))
	if copyErr != nil {
		return copyErr
	}
	if written > limit {
		return fmt.Errorf("download exceeds %d byte limit", limit)
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	complete = true
	return nil
}

func (c *Client) acquire(ctx context.Context) (func(), error) {
	key, err := filepath.Abs(filepath.Clean(c.cfg.SteamCMDDir))
	if err != nil {
		return nil, err
	}
	if c.goos == "windows" {
		key = strings.ToLower(key)
	}
	value, _ := commandGates.LoadOrStore(key, &commandGate{token: make(chan struct{}, 1)})
	gate := value.(*commandGate)
	select {
	case gate.token <- struct{}{}:
		return func() { <-gate.token }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) validateManaged(path string) error {
	return c.cfg.ValidateManagedPath(path, false)
}

func (c *Client) validateInstalled() error {
	if err := c.validateManaged(c.cfg.SteamCMDDir); err != nil {
		return err
	}
	if err := c.validateManaged(c.cfg.SteamCMDBinaryPath()); err != nil {
		return err
	}
	return ValidatePEExecutable(c.cfg.SteamCMDBinaryPath())
}

func (c *Client) removeManagedDirectory(path string) error {
	if err := c.validateManaged(path); err != nil {
		return err
	}
	if err := c.validateManaged(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func ValidatePEExecutable(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer file.Close()
	var signature [2]byte
	if _, err := io.ReadFull(file, signature[:]); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if signature != [2]byte{'M', 'Z'} {
		return fmt.Errorf("%s does not have a Windows PE signature", path)
	}
	return nil
}

func runCommand(ctx context.Context, binary, directory string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = directory
	cmd.WaitDelay = 5 * time.Second
	buffer := newTailBuffer(maxCommandOutputBytes)
	cmd.Stdout = buffer
	cmd.Stderr = buffer
	err := cmd.Run()
	return buffer.Bytes(), err
}

type tailBuffer struct {
	limit int
	data  []byte
}

func newTailBuffer(limit int) *tailBuffer { return &tailBuffer{limit: limit} }

func (b *tailBuffer) Write(value []byte) (int, error) {
	written := len(value)
	if b.limit <= 0 {
		return written, nil
	}
	if len(value) >= b.limit {
		b.data = append(b.data[:0], value[len(value)-b.limit:]...)
		return written, nil
	}
	if overflow := len(b.data) + len(value) - b.limit; overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:len(b.data)-overflow]
	}
	b.data = append(b.data, value...)
	return written, nil
}

func (b *tailBuffer) Bytes() []byte { return bytes.Clone(b.data) }

func sanitizeOutput(output string, secrets ...string) string {
	output = strings.TrimSpace(output)
	for _, secret := range secrets {
		if secret != "" {
			output = strings.ReplaceAll(output, secret, "[REDACTED]")
		}
	}
	if len(output) > maxCommandOutputBytes {
		output = output[len(output)-maxCommandOutputBytes:]
	}
	return output
}

func workshopCommandFailure(output []byte, itemID string, secrets ...string) error {
	detail := sanitizeOutput(string(output), secrets...)
	lower := strings.ToLower(detail)
	if !strings.Contains(lower, "error! download item") &&
		!strings.Contains(lower, "download item "+strings.ToLower(itemID)+" failed") {
		return nil
	}
	message := "SteamCMD reported that Workshop item " + itemID + " failed to download"
	message += "; a missing decryption key usually means the signed-in account does not own Palworld or cannot access this Workshop item"
	if detail != "" {
		message += ": " + detail
	}
	return errors.New(message)
}

func validateNumericID(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s ID is required", label)
	}
	if len(value) > 20 {
		return fmt.Errorf("%s ID is invalid", label)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return fmt.Errorf("%s ID must be numeric", label)
		}
	}
	return nil
}

func extractZip(archivePath, destination string, validate func(string) error) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) == 0 {
		return errors.New("zip is empty")
	}
	if len(reader.File) > maxArchiveEntries {
		return fmt.Errorf("zip contains too many entries")
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	if err := validate(destination); err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(reader.File))
	var declared, extracted int64
	for _, entry := range reader.File {
		clean, err := safeArchivePath(entry.Name)
		if err != nil {
			return err
		}
		key := strings.ToLower(filepath.ToSlash(clean))
		if _, ok := seen[key]; ok {
			return fmt.Errorf("zip contains duplicate path: %s", entry.Name)
		}
		seen[key] = struct{}{}
		if entry.UncompressedSize64 > uint64(maxExtractedBytes) || declared > maxExtractedBytes-int64(entry.UncompressedSize64) {
			return fmt.Errorf("zip exceeds extracted size limit")
		}
		declared += int64(entry.UncompressedSize64)
		mode := entry.Mode()
		isDirectory := entry.FileInfo().IsDir()
		if mode&os.ModeSymlink != 0 || (!isDirectory && mode.Type() != 0) {
			return fmt.Errorf("zip contains unsupported file type: %s", entry.Name)
		}
		target := filepath.Join(destination, clean)
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(destination, targetAbs)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("zip contains unsafe path: %s", entry.Name)
		}
		if err := validate(targetAbs); err != nil {
			return err
		}
		if isDirectory {
			if err := os.MkdirAll(targetAbs, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		permissions := mode.Perm()
		if permissions == 0 {
			permissions = 0o644
		}
		output, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, permissions)
		if err != nil {
			_ = input.Close()
			return err
		}
		remaining := maxExtractedBytes - extracted
		written, copyErr := io.Copy(output, io.LimitReader(input, remaining+1))
		extracted += written
		closeErr := output.Close()
		_ = input.Close()
		if copyErr != nil {
			return copyErr
		}
		if extracted > maxExtractedBytes {
			return fmt.Errorf("zip exceeds extracted size limit")
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func safeArchivePath(raw string) (string, error) {
	normalized := strings.ReplaceAll(raw, "\\", "/")
	normalized = strings.TrimSuffix(normalized, "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") {
		return "", fmt.Errorf("zip contains unsafe path: %s", raw)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(normalized)))
	if clean != normalized || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(filepath.FromSlash(clean)) {
		return "", fmt.Errorf("zip contains unsafe path: %s", raw)
	}
	for _, component := range strings.Split(clean, "/") {
		if !safeWindowsPathComponent(component) {
			return "", fmt.Errorf("zip contains a path unsupported on Windows: %s", raw)
		}
	}
	return filepath.FromSlash(clean), nil
}

func safeWindowsPathComponent(component string) bool {
	if component == "" || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") || strings.ContainsAny(component, `<>"|?*`) {
		return false
	}
	for _, character := range component {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	base := component
	if index := strings.IndexByte(base, '.'); index >= 0 {
		base = base[:index]
	}
	base = strings.ToUpper(base)
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CLOCK$" {
		return false
	}
	return !(len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9')
}
