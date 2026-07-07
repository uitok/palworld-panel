package docker

import "testing"

func TestPalServerPortUsesStartupArg(t *testing.T) {
	got := palServerPort([]string{"-players=24", "-port=9001"}, 8211)
	if got != 9001 {
		t.Fatalf("expected startup port, got %d", got)
	}
}

func TestPalServerPortFallsBackOnInvalidArg(t *testing.T) {
	for _, args := range [][]string{
		{"-port=0"},
		{"-port=70000"},
		{"-port=not-a-port"},
		nil,
	} {
		if got := palServerPort(args, 8211); got != 8211 {
			t.Fatalf("expected fallback for %#v, got %d", args, got)
		}
	}
}
