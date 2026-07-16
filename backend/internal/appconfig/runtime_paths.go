package appconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type RuntimeLayout struct {
	ApplicationRoot string
	RepositoryRoot  string
	RuntimeRoot     string
	Development     bool
	Structured      bool
}

func ResolveRuntimeLayout(override string) (RuntimeLayout, error) {
	repositoryRoot, _ := discoverRepositoryRoot()
	applicationRoot := executableRoot()
	if repositoryRoot != "" && !pathWithin(repositoryRoot, applicationRoot) {
		applicationRoot = repositoryRoot
	}
	layout := RuntimeLayout{ApplicationRoot: applicationRoot, RepositoryRoot: repositoryRoot}

	override = strings.TrimSpace(override)
	if override != "" {
		base := applicationRoot
		if repositoryRoot != "" {
			base = repositoryRoot
		}
		root, err := absoluteFrom(base, override)
		if err != nil {
			return RuntimeLayout{}, fmt.Errorf("resolve PALPANEL_RUNTIME_ROOT: %w", err)
		}
		layout.RuntimeRoot = root
		layout.Structured = true
		layout.Development = repositoryRoot != "" && pathWithin(repositoryRoot, root)
		if err := validateRuntimeRoot(layout); err != nil {
			return RuntimeLayout{}, err
		}
		return layout, nil
	}

	if runtime.GOOS == "windows" && repositoryRoot != "" {
		layout.RuntimeRoot = filepath.Join(repositoryRoot, "dev-runtime", "windows")
		layout.Development = true
		layout.Structured = true
		if err := validateRuntimeRoot(layout); err != nil {
			return RuntimeLayout{}, err
		}
	}
	return layout, nil
}

func (c Config) ValidateManagedPath(path string, allowRoot bool) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("managed path is empty")
	}
	root := strings.TrimSpace(c.RuntimeRoot)
	if root == "" {
		root = strings.TrimSpace(c.DataDir)
	}
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	if isVolumeRoot(rootAbs) {
		return fmt.Errorf("managed root must not be a volume root: %s", rootAbs)
	}
	if !allowRoot && samePath(rootAbs, targetAbs) {
		return fmt.Errorf("managed target must not equal runtime root: %s", targetAbs)
	}
	if !pathWithin(rootAbs, targetAbs) {
		if c.ServerDirectoryImported() {
			if err := validateImportedServerPath(c.ServerDirectory(), targetAbs); err == nil {
				return nil
			}
		}
		return fmt.Errorf("managed path escapes runtime root: %s", targetAbs)
	}
	if c.RepositoryRoot != "" && samePath(c.RepositoryRoot, targetAbs) {
		return fmt.Errorf("managed path must not equal repository root: %s", targetAbs)
	}

	resolvedRoot, err := resolveExistingPath(rootAbs)
	if err != nil {
		return fmt.Errorf("resolve runtime root: %w", err)
	}
	resolvedTarget, err := resolveExistingPath(targetAbs)
	if err != nil {
		return fmt.Errorf("resolve managed path: %w", err)
	}
	if !pathWithin(resolvedRoot, resolvedTarget) {
		return fmt.Errorf("managed path escapes runtime root through a link or reparse point: %s", targetAbs)
	}
	if c.DevelopmentMode && c.RepositoryRoot != "" {
		resolvedRepository, err := resolveExistingPath(c.RepositoryRoot)
		if err != nil {
			return fmt.Errorf("resolve repository root: %w", err)
		}
		if !pathWithin(resolvedRepository, resolvedRoot) {
			return fmt.Errorf("development runtime root escapes repository through a link or reparse point: %s", rootAbs)
		}
	}
	return nil
}

func validateImportedServerPath(serverRoot, target string) error {
	serverAbs, err := filepath.Abs(filepath.Clean(serverRoot))
	if err != nil {
		return err
	}
	if isVolumeRoot(serverAbs) {
		return fmt.Errorf("imported server directory must not be a volume root: %s", serverAbs)
	}
	if !pathWithin(serverAbs, target) {
		return fmt.Errorf("managed path escapes imported server directory: %s", target)
	}
	resolvedServer, err := resolveExistingPath(serverAbs)
	if err != nil {
		return fmt.Errorf("resolve imported server directory: %w", err)
	}
	resolvedTarget, err := resolveExistingPath(target)
	if err != nil {
		return fmt.Errorf("resolve imported server path: %w", err)
	}
	if !pathWithin(resolvedServer, resolvedTarget) {
		return fmt.Errorf("managed path escapes imported server directory through a link or reparse point: %s", target)
	}
	return nil
}

func validateRuntimeRoot(layout RuntimeLayout) error {
	root := filepath.Clean(layout.RuntimeRoot)
	if root == "" || root == "." {
		return fmt.Errorf("PALPANEL_RUNTIME_ROOT must not be empty")
	}
	if isVolumeRoot(root) {
		return fmt.Errorf("PALPANEL_RUNTIME_ROOT must not be a volume root: %s", root)
	}
	if layout.RepositoryRoot == "" {
		return nil
	}
	if samePath(root, layout.RepositoryRoot) {
		return fmt.Errorf("PALPANEL_RUNTIME_ROOT must not equal the repository root")
	}
	for _, name := range []string{".git", "backend", "frontend", "sav-cli", "scripts"} {
		protected := filepath.Join(layout.RepositoryRoot, name)
		if pathWithin(protected, root) || pathWithin(root, protected) {
			return fmt.Errorf("PALPANEL_RUNTIME_ROOT overlaps protected source path %s", protected)
		}
	}
	return nil
}

func discoverRepositoryRoot() (string, bool) {
	var starts []string
	if _, source, _, ok := runtime.Caller(0); ok && filepath.IsAbs(source) {
		starts = append(starts, filepath.Dir(source))
	}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	starts = append(starts, executableRoot())
	for _, start := range starts {
		if root := findRepositoryAncestor(start); root != "" {
			return root, true
		}
	}
	return "", false
}

func findRepositoryAncestor(start string) string {
	current, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if isRepositoryRoot(current) {
			return filepath.Clean(current)
		}
		parent := filepath.Dir(current)
		if samePath(parent, current) {
			return ""
		}
		current = parent
	}
}

func isRepositoryRoot(path string) bool {
	for _, relative := range []string{".git", filepath.Join("backend", "go.mod"), filepath.Join("frontend", "package.json"), filepath.Join("sav-cli", "go.mod")} {
		if _, err := os.Stat(filepath.Join(path, relative)); err != nil {
			return false
		}
	}
	return true
}

func executableRoot() string {
	if executable, err := os.Executable(); err == nil {
		if absolute, err := filepath.Abs(executable); err == nil {
			return filepath.Dir(absolute)
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Clean(cwd)
	}
	return "."
}

func absoluteFrom(base, path string) (string, error) {
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return filepath.Abs(filepath.Clean(path))
}

func resolveExistingPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	current := absolute
	var suffix []string
	for {
		_, statErr := os.Lstat(current)
		if statErr == nil {
			resolved, err := resolveFinalPath(current)
			if err != nil {
				return "", err
			}
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return filepath.Abs(filepath.Clean(resolved))
		}
		if !os.IsNotExist(statErr) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if samePath(parent, current) {
			return absolute, nil
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func pathWithin(root, target string) bool {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)))
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func isVolumeRoot(path string) bool {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	if volume == "" {
		return clean == string(os.PathSeparator)
	}
	return samePath(clean, volume+string(os.PathSeparator))
}
