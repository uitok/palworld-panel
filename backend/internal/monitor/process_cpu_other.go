//go:build !windows

package monitor

import (
	"context"
	"errors"
)

func platformProcessCPUPercent(context.Context, int) (float64, error) {
	return 0, errors.New("Windows process CPU collection is unavailable on this host")
}
