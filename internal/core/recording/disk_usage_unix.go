//go:build !windows

package recording

import "syscall"

// getDiskUsage 获取指定路径所在磁盘的使用率（百分比）
func getDiskUsage(path string) (float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total == 0 {
		return 0, nil
	}
	return float64(total-free) / float64(total) * 100, nil
}
