package recording

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gowvp/owl/internal/conf"
)

// 验证文件系统兜底清理的进展回报：有超龄目录可删返回 true，只剩今日目录返回 false
func TestCleanupDiskByFilesystemProgress(t *testing.T) {
	t.Chdir(t.TempDir())

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	for _, d := range []string{yesterday, today} {
		dir := filepath.Join("recordings", d)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "a.mp4"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// 阈值 0 使磁盘使用率恒判超标
	c := Core{conf: &conf.ServerRecording{DiskUsageThreshold: 0}}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	absStorageDir := filepath.Join(wd, "recordings")

	if !c.cleanupDiskByFilesystem(absStorageDir) {
		t.Fatal("有超龄目录可删，应返回 true")
	}
	if _, err := os.Stat(filepath.Join(absStorageDir, yesterday)); !os.IsNotExist(err) {
		t.Fatalf("超龄目录应删除, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(absStorageDir, today)); err != nil {
		t.Fatalf("今日目录应保留: %v", err)
	}

	if c.cleanupDiskByFilesystem(absStorageDir) {
		t.Fatal("只剩今日目录无可删项，应返回 false")
	}
}
