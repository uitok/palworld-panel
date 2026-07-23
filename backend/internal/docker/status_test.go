package docker

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"palpanel/internal/appconfig"
)

func TestStatusUsesOneBoundedStructuredInspect(t *testing.T) {
	var calls [][]string
	bounded := false
	runner := Runner{
		cfg: appconfig.Config{DockerContainer: "palworld"},
		runFunc: func(ctx context.Context, args ...string) ([]byte, error) {
			calls = append(calls, append([]string(nil), args...))
			deadline, ok := ctx.Deadline()
			bounded = ok && time.Until(deadline) > 0 && time.Until(deadline) <= 5*time.Second
			return []byte(`[{"RestartCount":3,"State":{"Status":"exited","OOMKilled":true,"ExitCode":137,"StartedAt":"2026-07-22T01:00:00Z","FinishedAt":"2026-07-22T02:00:00Z"}}]`), nil
		},
	}
	status, err := runner.Status(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], []string{"inspect", "palworld"}) {
		t.Fatalf("inspect calls = %#v", calls)
	}
	if !bounded {
		t.Fatal("docker inspect context was not bounded to five seconds")
	}
	if !status.Exists || status.Status != "exited" || !status.LifecycleAvailable || !status.OOMKilled || status.ExitCode != 137 || status.RestartCount != 3 || status.FinishedAt == "" {
		t.Fatalf("status = %#v", status)
	}
}

func TestStatusMissingStateDoesNotClaimLifecycleAvailability(t *testing.T) {
	runner := Runner{runFunc: func(context.Context, ...string) ([]byte, error) {
		return []byte(`[{"RestartCount":0}]`), nil
	}}
	status, err := runner.Status(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Exists || status.LifecycleAvailable || status.Status != "unknown" {
		t.Fatalf("status = %#v", status)
	}
}

func TestStatusPreservesMissingContainerSemantics(t *testing.T) {
	runner := Runner{runFunc: func(context.Context, ...string) ([]byte, error) {
		return []byte("No such object: palworld"), errors.New("exit status 1")
	}}
	status, err := runner.Status(t.Context())
	if err != nil || status.Exists || status.Status != "missing" || status.LifecycleAvailable {
		t.Fatalf("status = %#v, %v", status, err)
	}
}
