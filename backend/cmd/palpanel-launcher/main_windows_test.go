package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestStartChildRedirectsOutputToSavCLILog(t *testing.T) {
	job, err := createKillOnCloseJob()
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)

	logPath := filepath.Join(t.TempDir(), "sav-cli.log")
	child, err := startChild(
		job,
		os.Args[0],
		[]string{"-test.run=TestLauncherChildHelper"},
		map[string]string{"PALPANEL_LAUNCHER_CHILD_HELPER": "1"},
		logPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-child.done; err != nil {
		t.Fatalf("helper process failed: %v", err)
	}
	child.closeLog()

	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "sav-cli stdout marker") || !strings.Contains(text, "sav-cli stderr marker") {
		t.Fatalf("sav-cli.log did not receive both output streams: %q", text)
	}
}

func TestLauncherChildHelper(t *testing.T) {
	if os.Getenv("PALPANEL_LAUNCHER_CHILD_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "sav-cli stdout marker")
	fmt.Fprintln(os.Stderr, "sav-cli stderr marker")
	os.Exit(0)
}
