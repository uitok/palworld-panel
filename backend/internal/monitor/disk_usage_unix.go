//go:build !windows

package monitor

import "golang.org/x/sys/unix"

func platformDiskUsage(path string) (int64, int64, error) {
	var stats unix.Statfs_t
	if err := unix.Statfs(path, &stats); err != nil {
		return 0, 0, err
	}
	blockSize := int64(stats.Bsize)
	return int64(stats.Bavail) * blockSize, int64(stats.Blocks) * blockSize, nil
}
