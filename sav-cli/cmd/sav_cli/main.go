package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode"

	"palpanel/sav-cli/internal/buildinfo"
	"palpanel/sav-cli/internal/indexer"
	"palpanel/sav-cli/internal/sav"
	"palpanel/sav-cli/internal/sidecar"
)

var escapedBytePattern = regexp.MustCompile(`\\[xX][0-9A-Fa-f]{2}`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, safeDiagnostic(err.Error()))
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "-version", "--version", "version":
		info := buildinfo.Current()
		fmt.Printf("sav-cli %s (commit %s, built %s)\n", info.Version, info.Commit, info.BuildTime)
		return nil
	case "index":
		return runIndex(args[1:])
	case "inspect":
		return runInspect(args[1:])
	case "verify-build":
		return runVerifyBuild(args[1:])
	case "serve":
		return runServe(args[1:])
	case "-h", "--help", "help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q\n\n%w", args[0], usage())
	}
}

func runIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	saveDir := fs.String("save-dir", "", "world save directory, save root, or Level.sav path")
	output := fs.String("output", "-", "output JSON path or - for stdout")
	_ = fs.Int("timeout-seconds", 120, "accepted for sidecar API parity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *saveDir == "" {
		return errors.New("--save-dir is required")
	}
	idx, err := indexer.Build(*saveDir)
	if err != nil {
		return err
	}
	return writeJSON(*output, idx)
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	file := fs.String("file", "", "Level.sav or LevelMeta.sav path")
	output := fs.String("output", "-", "output JSON path or - for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("--file is required")
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	info, inspectErr := sav.Inspect(data)
	if inspectErr != nil {
		info.Magic = ""
		info.SaveType = 0
	}
	payload := map[string]any{
		"file":    *file,
		"size":    len(data),
		"inspect": info,
	}
	if inspectErr != nil {
		payload["error"] = safeDiagnostic(inspectErr.Error())
	}
	return writeJSON(*output, payload)
}

func runVerifyBuild(args []string) error {
	oodleAvailable := sav.OodleAvailable()
	if err := verifyBuild(args, oodleAvailable); err != nil {
		return err
	}
	fmt.Println(buildVerificationMessage(oodleAvailable))
	return nil
}

func buildVerificationMessage(oodleAvailable bool) string {
	return fmt.Sprintf("sav-cli build verification passed: oodle=%t", oodleAvailable)
}

func verifyBuild(args []string, oodleAvailable bool) error {
	fs := flag.NewFlagSet("verify-build", flag.ContinueOnError)
	requireOodle := fs.Bool("require-oodle", false, "fail when this build does not include Oodle support")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *requireOodle && !oodleAvailable {
		return errors.New("Oodle support is required, but this sav-cli build reports oodle=false")
	}
	return nil
}

func safeDiagnostic(value string) string {
	value = strings.ToValidUTF8(value, "")
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, value)
	value = escapedBytePattern.ReplaceAllString(value, "[byte]")
	runes := []rune(value)
	if len(runes) > 512 {
		value = string(runes[:512]) + "…"
	}
	return value
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	host := fs.String("host", getenv("SAVE_INDEXER_HOST", "127.0.0.1"), "HTTP host")
	port := fs.String("port", getenv("SAVE_INDEXER_PORT", "8090"), "HTTP port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	addr := net.JoinHostPort(*host, *port)
	httpServer := sidecar.NewHTTPServer(addr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(os.Stderr, "sav-cli %s listening on http://%s\n", buildinfo.Version, addr)
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	err := <-errCh
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func writeJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if path == "" || path == "-" {
		_, err = os.Stdout.Write(body)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func usage() error {
	return errors.New(`usage:
  sav_cli index --save-dir <world-dir|save-root|Level.sav> [--output <path|-]
  sav_cli inspect --file <Level.sav|LevelMeta.sav> [--output <path|-]
  sav_cli verify-build [--require-oodle]
  sav_cli serve [--host 127.0.0.1] [--port 8090]
  sav_cli --version`)
}
