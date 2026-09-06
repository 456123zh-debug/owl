//go:build windows

package recording

import "golang.org/x/sys/windows"

// getDiskUsage 获取指定路径所在磁盘的使用率（百分比）
func getDiskUsage(path string) (float64, error) {
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	if err := windows.GetDiskFreeSpaceEx(p, &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes); err != nil {
		return 0, err
	}
	if totalNumberOfBytes == 0 {
		return 0, nil
	}
	return float64(totalNumberOfBytes-totalNumberOfFreeBytes) / float64(totalNumberOfBytes) * 100, nil
}
