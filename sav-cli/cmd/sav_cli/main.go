package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"palpanel/sav-cli/internal/indexer"
	"palpanel/sav-cli/internal/sav"
	"palpanel/sav-cli/internal/sidecar"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "index":
		return runIndex(args[1:])
	case "inspect":
		return runInspect(args[1:])
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
	payload := map[string]any{
		"file":    *file,
		"size":    len(data),
		"inspect": info,
	}
	if inspectErr != nil {
		payload["error"] = inspectErr.Error()
	}
	return writeJSON(*output, payload)
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	host := fs.String("host", getenv("SAVE_INDEXER_HOST", "127.0.0.1"), "HTTP host")
	port := fs.String("port", getenv("SAVE_INDEXER_PORT", "8090"), "HTTP port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	addr := net.JoinHostPort(*host, *port)
	fmt.Fprintf(os.Stderr, "sav_cli sidecar listening on http://%s\n", addr)
	return sidecar.ListenAndServe(addr)
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
  sav_cli serve [--host 127.0.0.1] [--port 8090]`)
}
