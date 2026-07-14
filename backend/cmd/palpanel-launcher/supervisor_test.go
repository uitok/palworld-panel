package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestWaitForHealthDetectsChildExit(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestLauncherExitHelper$")
	command.Env = append(os.Environ(), "PALPANEL_LAUNCHER_EXIT_HELPER=1")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
		close(done)
	}()
	child := &childProcess{done: done}

	started := time.Now()
	err := waitForHealth("http://127.0.0.1:0/health", child, 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "process exited") {
		t.Fatalf("expected child exit error, got %v", err)
	}
	if elapsed := time.Since(started); elapsed >= 2*time.Second {
		t.Fatalf("child exit detection took %s", elapsed)
	}
}

func TestWaitForEitherChildReportsFirstExit(t *testing.T) {
	first := make(chan error, 1)
	second := make(chan error, 1)
	first <- errors.New("sav-cli stopped")
	err := waitForEitherChild(&childProcess{done: first}, &childProcess{done: second})
	second <- nil
	if err == nil || !strings.Contains(err.Error(), "sav-cli stopped") {
		t.Fatalf("expected first child error, got %v", err)
	}
}

func TestWaitForPromptOrChildrenReturnsWhenPromptCompletes(t *testing.T) {
	promptErr := errors.New("prompt failed")
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "accepted"},
		{name: "failed", err: promptErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			cancelled := make(chan struct{}, 1)
			prompt := startAsyncPrompt(
				func() error { return test.err },
				func(<-chan struct{}) { cancelled <- struct{}{} },
			)

			err := waitForPromptOrChildren(
				prompt,
				&childProcess{done: make(chan error)},
				&childProcess{done: make(chan error)},
			)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected prompt result %v, got %v", test.err, err)
			}
			select {
			case <-cancelled:
				t.Fatal("completed prompt was cancelled")
			default:
			}
		})
	}
}

func TestWaitForPromptOrChildrenReturnsWhenChildExits(t *testing.T) {
	tests := []struct {
		name      string
		exitChild string
	}{
		{name: "sav-cli", exitChild: "sav-cli"},
		{name: "backend", exitChild: "palpanel server"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			showStarted := make(chan struct{})
			releasePrompt := make(chan struct{})
			showReturned := make(chan struct{})
			prompt := startAsyncPrompt(
				func() error {
					close(showStarted)
					<-releasePrompt
					close(showReturned)
					return nil
				},
				func(finished <-chan struct{}) {
					close(releasePrompt)
					<-finished
				},
			)
			<-showStarted

			savDone := make(chan error, 1)
			serverDone := make(chan error, 1)
			exitErr := errors.New("unexpected exit")
			if test.exitChild == "sav-cli" {
				savDone <- exitErr
			} else {
				serverDone <- exitErr
			}

			err := waitForPromptOrChildren(
				prompt,
				&childProcess{done: savDone},
				&childProcess{done: serverDone},
			)
			if !errors.Is(err, exitErr) || !strings.Contains(err.Error(), test.exitChild) {
				t.Fatalf("expected %s exit error, got %v", test.exitChild, err)
			}
			select {
			case <-showReturned:
			default:
				t.Fatal("prompt goroutine did not return before lifecycle completed")
			}
		})
	}
}

func TestLauncherExitHelper(t *testing.T) {
	if os.Getenv("PALPANEL_LAUNCHER_EXIT_HELPER") != "1" {
		return
	}
	os.Exit(23)
}
