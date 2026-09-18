package adapter

import (
	"github.com/gowvp/owl/internal/core/recording"
	"github.com/gowvp/owl/internal/core/sms"
	"github.com/gowvp/owl/pkg/zlm"
	"os"
	"path/filepath"
	"time"
)

var _ recording.SMSProvider = (*SMSAdapter)(nil)

// SMSAdapter 实现 recording.SMSProvider 接口
// 将 sms.Core 的录制能力适配给 recording 领域使用
type SMSAdapter struct {
	smsCore sms.Core
}

func (a *SMSAdapter) StartRecording(app, stream string) error {
	ms, err := a.smsCore.GetDefaultMediaServer()
	if err != nil {
		return err
	}
	path := filepath.Join(ms.RecordingStorageDir, time.Now().Format("2006-01-02"), time.Now().Format("15-04-05.000"))
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	return a.smsCore.StartHLSRecord(ms, zlm.RecordControlRequest{Vhost: "__defaultVhost__", App: app, Stream: stream, CustomizedPath: path})
}

func (a *SMSAdapter) StopRecording(app, stream string) error {
	ms, err := a.smsCore.GetDefaultMediaServer()
	if err != nil {
		return err
	}
	return a.smsCore.StopHLSRecord(ms, zlm.RecordControlRequest{Vhost: "__defaultVhost__", App: app, Stream: stream})
}

// NewSMSAdapter 创建 SMS 适配器，返回 recording.SMSProvider 接口
// Wire 通过此函数自动绑定 sms.Core -> recording.SMSProvider
func NewSMSAdapter(smsCore sms.Core) recording.SMSProvider {
	return &SMSAdapter{smsCore: smsCore}
}

// ListRecordingStreams 批量获取所有在线流的录制状态
// 调用 ZLM getMediaList 一次获取全部流，提取 isRecordingHLS 状态
// 返回 map key 格式为 "app/stream"
func (a *SMSAdapter) ListRecordingStreams() (map[string]bool, error) {
	ms, err := a.smsCore.GetDefaultMediaServer()
	if err != nil {
		return nil, err
	}
	resp, err := a.smsCore.GetMediaList(ms)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(resp.Data))
	for _, item := range resp.Data {
		key := item.App + "/" + item.Stream
		if item.IsRecordingHLS {
			result[key] = true
		} else if _, exists := result[key]; !exists {
			result[key] = false
		}
	}
	return result, nil
}
