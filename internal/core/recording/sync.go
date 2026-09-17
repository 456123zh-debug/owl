package recording

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	// syncDefaultInterval 默认录制同步周期
	syncDefaultInterval = 5 * time.Minute
	// syncInitialDelay 首次同步延迟，等待设备注册和 catalog 完成
	syncInitialDelay = 30 * time.Second
	// snapshotConcurrency 取快照触发拉流的最大并发数，防止大量 INVITE 冲击设备
	snapshotConcurrency = 5
	// snapshotTimeout 单次快照请求超时，ZLM getSnap 等待设备推流的最长时间
	snapshotTimeout = 30 * time.Second
)

// StartRecordingSyncLoop 启动录制同步协程
// 延迟 syncInitialDelay 后首次执行，之后每 syncInterval 同步一次
// 确保录制状态与通道配置一致，覆盖平台重启、ZLM 重启、流中断等异常场景
func (c Core) StartRecordingSyncLoop(ctx context.Context) {
	if !c.IsEnabled() {
		slog.Info("recording sync disabled (recording not enabled)")
		return
	}
	if c.ipcProvider == nil || c.smsProvider == nil {
		slog.Warn("recording sync disabled: ipcProvider or smsProvider not configured")
		return
	}

	interval := c.syncInterval
	if interval <= 0 {
		interval = syncDefaultInterval
	}

	slog.Info("recording sync loop started", "initial_delay", syncInitialDelay.String(), "interval", interval.String())

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(syncInitialDelay):
			c.syncRecordingTasks(ctx)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("recording sync loop stopped")
				return
			case <-ticker.C:
				c.syncRecordingTasks(ctx)
			}
		}
	}()
}

// syncRecordingTasks 执行一次录制状态同步
// 1. 查询所有在线通道（含 RecordMode）
// 2. 一次 getMediaList 批量获取 ZLM 上所有源流
// 3. 对应录制但尚无源流的通道触发拉流。持久化参数在 on_publish 决定。
func (c Core) syncRecordingTasks(ctx context.Context) {
	allChannels, err := c.ipcProvider.ListOnlineChannels(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "sync: 查询在线通道失败", "err", err)
		return
	}

	if len(allChannels) == 0 {
		slog.DebugContext(ctx, "sync: 无在线通道")
		return
	}

	recordingMap, err := c.smsProvider.ListRecordingStreams()
	if err != nil {
		slog.ErrorContext(ctx, "sync: 获取流录制状态失败", "err", err)
		return
	}

	var needTrigger []ChannelInfo
	var skipped int

	for _, ch := range allChannels {
		if ch.App == "" || ch.Stream == "" {
			skipped++
			continue
		}

		key := ch.App + "/" + ch.Stream
		_, streamExists := recordingMap[key]
		shouldRecord := ch.RecordMode != "none"

		if shouldRecord && !streamExists {
			needTrigger = append(needTrigger, ch)
		} else {
			skipped++
		}
	}

	triggered := c.triggerStreamsBatch(ctx, needTrigger)

	slog.Info("sync: 录制同步完成",
		"total", len(allChannels),
		"skipped", skipped,
		"triggered", triggered,
	)
}

// triggerStreamsBatch 批量取快照触发拉流
// 最多 snapshotConcurrency(5) 个协程并行，每个快照请求最长 snapshotTimeout(30s)
func (c Core) triggerStreamsBatch(ctx context.Context, channels []ChannelInfo) int {
	if c.playProvider == nil || len(channels) == 0 {
		return 0
	}

	sem := make(chan struct{}, snapshotConcurrency)
	var wg sync.WaitGroup
	var triggered int
	var mu sync.Mutex

	for _, ch := range channels {
		wg.Add(1)
		go func(ch ChannelInfo) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			snapCtx, cancel := context.WithTimeout(ctx, snapshotTimeout)
			defer cancel()

			if err := c.playProvider.TriggerStream(snapCtx, ch); err != nil {
				slog.DebugContext(ctx, "sync: 快照拉流失败（设备可能离线）",
					"channel", ch.ID, "type", ch.Type, "err", err)
				return
			}
			mu.Lock()
			triggered++
			mu.Unlock()
		}(ch)
	}

	wg.Wait()
	return triggered
}
