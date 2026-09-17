package recording

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/gowvp/owl/internal/conf"
)

type mockSMS struct{ streams map[string]bool }

func (m *mockSMS) ListRecordingStreams() (map[string]bool, error) { return m.streams, nil }

type mockIPC struct{ channels []ChannelInfo }

func (m *mockIPC) ListOnlineChannels(context.Context) ([]ChannelInfo, error) { return m.channels, nil }

type mockPlay struct{ triggered atomic.Int32 }

func (m *mockPlay) TriggerStream(context.Context, ChannelInfo) error {
	m.triggered.Add(1)
	return nil
}

func TestSyncTriggersOnlyMissingRecordedStreams(t *testing.T) {
	sms := &mockSMS{streams: map[string]bool{"pull/online": true}}
	ipc := &mockIPC{channels: []ChannelInfo{
		{ID: "online", App: "pull", Stream: "online", RecordMode: "always"},
		{ID: "missing", App: "pull", Stream: "missing", RecordMode: "always"},
		{ID: "disabled", App: "pull", Stream: "disabled", RecordMode: "none"},
	}}
	play := &mockPlay{}
	core := NewCore(nil,
		WithConfig(&conf.ServerRecording{StorageDir: "/tmp/test"}),
		WithSMSProvider(sms), WithIPCProvider(ipc), WithPlayProvider(play))

	core.syncRecordingTasks(context.Background())
	if got := play.triggered.Load(); got != 1 {
		t.Fatalf("triggered %d streams, want 1", got)
	}
}
