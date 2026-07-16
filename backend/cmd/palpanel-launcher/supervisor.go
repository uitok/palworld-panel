package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type childProcess struct {
	log  *os.File
	done <-chan error
}

type promptEvent struct {
	done   <-chan error
	cancel func()
}

type namedChild struct {
	name  string
	child *childProcess
}

func startAsyncPrompt(show func() error, dismiss func(finished <-chan struct{})) promptEvent {
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		err := show()
		close(finished)
		done <- err
		close(done)
	}()

	var dismissOnce sync.Once
	return promptEvent{
		done: done,
		cancel: func() {
			dismissOnce.Do(func() {
				if dismiss == nil {
					<-finished
					return
				}
				dismiss(finished)
			})
		},
	}
}

func (c *childProcess) closeLog() {
	if c != nil && c.log != nil {
		_ = c.log.Close()
	}
}

func waitForHealth(url string, child *childProcess, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-child.done:
			return childExitError(err)
		case <-ticker.C:
			request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			response, err := client.Do(request)
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode == http.StatusOK {
					select {
					case err := <-child.done:
						return childExitError(err)
					default:
						return nil
					}
				}
			}
		}
	}
}

func childExitError(err error) error {
	if err == nil {
		return errors.New("process exited")
	}
	return fmt.Errorf("process exited: %w", err)
}

func waitForPromptOrChildren(prompt promptEvent, savChild, serverChild *childProcess) error {
	return waitForPromptOrManagedChildren(prompt, namedChild{"sav-cli", savChild}, namedChild{"palpanel server", serverChild})
}

func waitForPromptOrManagedChildren(prompt promptEvent, children ...namedChild) error {
	result := make(chan error, len(children))
	for _, managed := range children {
		go func(item namedChild) {
			result <- managedChildExitError(item.name, <-item.child.done)
		}(managed)
	}
	select {
	case err := <-prompt.done:
		return err
	case err := <-result:
		prompt.cancel()
		<-prompt.done
		return err
	}
}

func managedChildExitError(name string, err error) error {
	if err == nil {
		return fmt.Errorf("%s exited", name)
	}
	return fmt.Errorf("%s exited: %w", name, err)
}

func waitForEitherChild(children ...*childProcess) error {
	result := make(chan error, len(children))
	for _, child := range children {
		go func(done <-chan error) { result <- <-done }(child.done)
	}
	err := <-result
	if err == nil {
		return errors.New("a managed process exited")
	}
	return fmt.Errorf("a managed process exited: %w", err)
}
