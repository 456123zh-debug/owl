package rtspadapter

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/internal/core/sms"
	"github.com/ixugo/goddd/pkg/web"
)

var _ ipc.Protocoler = (*Adapter)(nil)

// Adapter RTSP 协议适配器
// 处理 RTSP 拉流的状态管理
type Adapter struct {
	ipcCore ipc.Core
	smsCore sms.Core
}

// DeleteDevice implements ipc.Protocoler.
func (a *Adapter) DeleteDevice(ctx context.Context, device *ipc.Device) error {
	return nil
}

func NewAdapter(ipcCore ipc.Core, smsCore sms.Core) *Adapter {
	a := &Adapter{
		ipcCore: ipcCore,
		smsCore: smsCore,
	}
	go a.runHealthChecks()
	return a
}

const (
	healthCheckInterval = 30 * time.Second
	healthCheckTimeout  = 3 * time.Second
	offlineThreshold    = 3
)

// runHealthChecks 使用轻量 RTSP OPTIONS 独立维护设备在线状态。
// 播放代理是否存在不再等同于设备是否在线。
func (a *Adapter) runHealthChecks() {
	failures := make(map[string]int)
	check := func() {
		channels, _, err := a.ipcCore.ListChannels(context.Background(), &ipc.FindChannelInput{
			PagerFilter: web.NewPagerFilterMaxSize(),
			Type:        ipc.TypeRTSP,
		})
		if err != nil {
			slog.Warn("RTSP 在线探测查询通道失败", "err", err)
			return
		}

		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		var mu sync.Mutex
		for _, ch := range channels {
			ch := ch
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				online := probeRTSP(ch.Config.SourceURL)
				mu.Lock()
				if online {
					failures[ch.ID] = 0
				} else {
					failures[ch.ID]++
				}
				failed := failures[ch.ID]
				mu.Unlock()

				wantOnline := online || failed < offlineThreshold && ch.IsOnline
				if wantOnline == ch.IsOnline {
					return
				}
				if _, err := a.ipcCore.UpdateChannelConfigAndOnline(context.Background(), ch.ID, wantOnline, func(*ipc.StreamConfig) {}); err != nil {
					slog.Warn("更新 RTSP 探活状态失败", "channel", ch.ID, "err", err)
				}
			}()
		}
		wg.Wait()
	}

	time.Sleep(5 * time.Second)
	check()
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()
	for range ticker.C {
		check()
	}
}

// probeRTSP 收到任意 RTSP 响应（包括 401）都说明设备和 RTSP 服务可达。
func probeRTSP(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Scheme, "rtsp") || u.Hostname() == "" {
		return false
	}
	port := u.Port()
	if port == "" {
		port = "554"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(u.Hostname(), port), healthCheckTimeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(healthCheckTimeout))
	if _, err := fmt.Fprintf(conn, "OPTIONS %s RTSP/1.0\r\nCSeq: 1\r\nUser-Agent: owl-healthcheck\r\n\r\n", rawURL); err != nil {
		return false
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && strings.HasPrefix(line, "RTSP/")
}

// InitDevice implements ipc.Protocoler.
func (a *Adapter) InitDevice(ctx context.Context, device *ipc.Device) error {
	return nil
}

// OnStreamChanged implements ipc.Protocoler.
// RTSP 拉流断开时更新通道状态（IsOnline=false, IsPlaying=false）
func (a *Adapter) OnStreamChanged(ctx context.Context, app, stream string) error {
	// 通过 app+stream 查询通道，支持自定义 app/stream
	ch, err := a.ipcCore.GetChannelByAppStreamOrID(ctx, app, stream)
	if err != nil {
		slog.WarnContext(ctx, "RTSP 通道未找到", "app", app, "stream", stream, "err", err)
		return nil
	}
	if _, err := a.ipcCore.UpdateChannelOnlineAndPlaying(ctx, ch.Stream, false, false); err != nil {
		slog.WarnContext(ctx, "更新 RTSP 通道状态失败", "app", app, "stream", stream, "err", err)
	}
	return nil
}

// OnStreamNotFound implements ipc.Protocoler.
// 当流不存在时，从 Channel 获取配置并启动拉流代理
func (a *Adapter) OnStreamNotFound(ctx context.Context, app string, stream string) error {
	// 通过 app+stream 查询通道，支持自定义 app/stream
	ch, err := a.ipcCore.GetChannelByAppStreamOrID(ctx, app, stream)
	if err != nil {
		return err
	}

	svr, err := a.smsCore.GetMediaServer(ctx, sms.DefaultMediaServerID)
	if err != nil {
		return err
	}
	resp, err := a.smsCore.CreateStreamProxy(svr, sms.AddStreamProxyRequest{
		App:     ch.App,
		Stream:  ch.Stream,
		URL:     ch.Config.SourceURL,
		RTPType: ch.Config.Transport,
	})
	if err != nil {
		return err
	}

	// 更新 StreamKey 和 IsOnline（用于后续关闭拉流代理）
	_, err = a.ipcCore.UpdateChannelConfigAndOnline(ctx, ch.ID, true, func(cfg *ipc.StreamConfig) {
		cfg.StreamKey = resp.Data.Key
	})

	return err
}

// QueryCatalog implements ipc.Protocoler.
func (a *Adapter) QueryCatalog(ctx context.Context, device *ipc.Device) error {
	return nil
}

// StartPlay implements ipc.Protocoler.
func (a *Adapter) StartPlay(ctx context.Context, device *ipc.Device, channel *ipc.Channel) (*ipc.PlayResponse, error) {
	return nil, nil
}

// StopPlay implements ipc.Protocoler.
func (a *Adapter) StopPlay(ctx context.Context, device *ipc.Device, channel *ipc.Channel) error {
	return nil
}

// ValidateDevice implements ipc.Protocoler.
func (a *Adapter) ValidateDevice(ctx context.Context, device *ipc.Device) error {
	return nil
}

// PTZControl implements ipc.Protocoler.
// RTSP 协议不支持云台控制
func (a *Adapter) PTZControl(ctx context.Context, device *ipc.Device, channel *ipc.Channel, cmd ipc.PTZCommand) error {
	return nil
}
