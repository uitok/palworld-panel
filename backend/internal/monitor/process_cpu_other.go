//go:build !windows

package monitor

import (
	"context"
	"errors"
)

func platformProcessTreeStats(context.Context, int) (ProcessStats, error) {
	return ProcessStats{}, errors.New("Windows process-tree collection is unavailable on this host")
}
