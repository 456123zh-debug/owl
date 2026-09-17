package api

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gowvp/owl/internal/conf"
	"github.com/gowvp/owl/internal/core/event"
	"github.com/gowvp/owl/internal/core/ipc"
	"github.com/gowvp/owl/internal/core/recording"
	"github.com/gowvp/owl/internal/core/sms"
	"github.com/gowvp/owl/pkg/gbs"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
)

type WebHookAPI struct {
	smsCore       sms.Core
	ipcCore       ipc.Core
	recordingCore recording.Core
	eventCore     event.Core
	conf          *conf.Bootstrap
	log           *slog.Logger
	gbs           *gbs.Server
	uc            *Usecase

	protocols map[string]ipc.Protocoler
}

func NewWebHookAPI(core sms.Core, conf *conf.Bootstrap, gbs *gbs.Server, ipcBundle IPCBundle, recordingCore recording.Core, eventCore event.Core) WebHookAPI {
	return WebHookAPI{
		smsCore:       core,
		ipcCore:       ipcBundle.Core,
		recordingCore: recordingCore,
		eventCore:     eventCore,
		conf:          conf,
		log:           slog.With("hook", "zlm"),
		gbs:           gbs,
		protocols:     ipcBundle.Protocols,
	}
}

func registerZLMWebhookAPI(r gin.IRouter, api WebHookAPI, handler ...gin.HandlerFunc) {
	{
		group := r.Group("/webhook", handler...)
		group.POST("/on_server_started", web.WrapH(api.onServerStarted))
		group.POST("/on_server_keepalive", web.WrapH(api.onServerKeepalive))
		group.POST("/on_stream_changed", web.WrapH(api.onStreamChanged))
		group.POST("/on_publish", web.WrapH(api.onPublish))
		group.POST("/on_play", web.WrapH(api.onPlay))
		group.POST("/on_stream_none_reader", web.WrapH(api.onStreamNoneReader))
		group.POST("/on_rtp_server_timeout", web.WrapH(api.onRTPServerTimeout))
		group.POST("/on_stream_not_found", web.WrapH(api.onStreamNotFound))
		group.POST("/on_record_ts", web.WrapH(api.onRecordTS))
		// 统一事件接收入口：兼容 Python AI 推送和 gowvp 间转发
		group.POST("/events", api.onWebhookEvents)
	}
}

// getChannelType 通过 app+stream 查询通道获取类型
// 支持自定义 app/stream 的 RTMP/RTSP 通道：先按 app+stream 查询，查不到再按 id=stream 查询
// 如果都找不到，则回退到使用 stream 前缀判断类型
func (w WebHookAPI) getChannelType(ctx context.Context, app, stream string) string {
	ch, err := w.ipcCore.GetChannelByAppStreamOrID(ctx, app, stream)
	if err == nil {
		return ch.GetType()
	}
	// 回退：使用 stream 前缀判断类型（兼容旧逻辑）
	return ipc.GetType(stream)
}

func (w WebHookAPI) onServerStarted(c *gin.Context, _ *struct{}) (DefaultOutput, error) {
	w.log.InfoContext(c.Request.Context(), "webhook onServerStarted")
	// 所有 rtmp 通道离线
	if err := w.ipcCore.BatchOfflineRTMP(context.Background()); err != nil {
		w.log.ErrorContext(c.Request.Context(), "webhook onServerStarted", "err", err)
	}

	return newDefaultOutputOK(), nil
}

// onServerKeepalive 服务器定时上报时间，上报间隔可配置，默认 10s 上报一次
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_16%E3%80%81on-server-keepalive
func (w WebHookAPI) onServerKeepalive(_ *gin.Context, in *onServerKeepaliveInput) (DefaultOutput, error) {
	// TODO: 仅支持默认
	w.smsCore.Keepalive(sms.DefaultMediaServerID)
	return newDefaultOutputOK(), nil
}

// onPublish rtsp/rtmp/rtp 推流鉴权事件。
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_7%E3%80%81on-publish
func (w WebHookAPI) onPublish(c *gin.Context, in *onPublishInput) (*onPublishOutput, error) {
	ctx := c.Request.Context()
	w.log.Info("webhook onPublish", "app", in.App, "stream", in.Stream, "schema", in.Schema, "mediaServerID", in.MediaServerID)
	publishOutput := &onPublishOutput{DefaultOutput: newDefaultOutputOK()}
	if ch, err := w.ipcCore.GetChannelByAppStreamOrID(ctx, in.App, in.Stream); err == nil {
		record := !ch.Ext.IsNoneRecord() && !w.conf.Server.Recording.Disabled
		publishOutput.EnableHlsFmp4 = &record
		publishOutput.EnableHls = new(false)
		publishOutput.EnableMp4 = new(false)
		if record {
			sessionRoot := filepath.Join(w.conf.Server.Recording.StorageDir,
				time.Now().Format("2006-01-02"), time.Now().Format("15-04-05.000"))
			publishOutput.HlsSavePath = &sessionRoot
		}
	}

	// 通过 app+stream 查询通道获取类型，支持自定义 app/stream
	channelType := w.getChannelType(ctx, in.App, in.Stream)

	// 获取协议适配器，检查是否实现了 OnPublisher 接口
	protocol, ok := w.protocols[channelType]
	if !ok {
		return publishOutput, nil
	}

	publisher, ok := protocol.(ipc.OnPublisher)
	if !ok {
		// 协议不需要推流鉴权，直接通过
		return publishOutput, nil
	}

	// 解析参数
	params, err := url.ParseQuery(in.Params)
	if err != nil {
		return &onPublishOutput{Code: 1, Msg: err.Error()}, nil
	}

	// 将 url.Values 转换为 map[string]string
	paramsMap := make(map[string]string)
	for k, v := range params {
		if len(v) > 0 {
			paramsMap[k] = v[0]
		}
	}
	paramsMap["media_server_id"] = in.MediaServerID

	// 调用协议适配器的 OnPublish 方法
	allowed, err := publisher.OnPublish(ctx, in.App, in.Stream, paramsMap)
	if err != nil {
		return &onPublishOutput{Code: 1, Msg: err.Error()}, nil
	}
	if !allowed {
		return &onPublishOutput{Code: 1, Msg: "鉴权失败"}, nil
	}

	return publishOutput, nil
}

// onStreamChanged rtsp/rtmp 流注册或注销时触发此事件；此事件对回复不敏感。
// 流注册时自动启动录制，流注销时停止录制并更新通道状态
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_12%E3%80%81on-stream-changed
func (w WebHookAPI) onStreamChanged(c *gin.Context, in *onStreamChangedInput) (DefaultOutput, error) {
	ctx := c.Request.Context()
	w.log.InfoContext(ctx, "webhook onStreamChanged", "app", in.App, "stream", in.Stream, "schema", in.Schema, "mediaServerID", in.MediaServerID, "regist", in.Regist)

	stream := in.Stream
	app := in.App

	// 通过 app+stream 查询通道获取类型，支持自定义 app/stream
	channelType := w.getChannelType(ctx, app, stream)

	if in.Regist {
		ch, err := w.ipcCore.GetChannelByAppStreamOrID(ctx, app, stream)
		if err != nil {
			w.log.WarnContext(ctx, "获取通道信息失败", "stream", stream, "err", err)
			return newDefaultOutputOK(), nil
		}

		// 派生的 RTMP/HLS 流会分别注册和注销，不能用它们覆盖源流状态。
		// RTSP 主流注册成功才表示拉流代理已经真正连通设备。
		if ch.IsRTSP() && in.Schema == "rtsp" {
			if _, err := w.ipcCore.UpdateChannelConfigAndOnline(ctx, ch.ID, true, func(*ipc.StreamConfig) {}); err != nil {
				w.log.WarnContext(ctx, "更新 RTSP 通道在线状态失败", "stream", stream, "err", err)
			}
		}

		return newDefaultOutputOK(), nil
	}

	// RTSP 的派生协议注销不代表源设备离线，只处理 RTSP 主流注销。
	if channelType == ipc.TypeRTSP && in.Schema != "rtsp" {
		return newDefaultOutputOK(), nil
	}

	// 流注销时通过 Protocoler 接口统一处理所有协议的状态更新
	// 每个协议适配器在 OnStreamChanged 中处理自己的状态逻辑
	protocol, ok := w.protocols[channelType]
	if ok {
		if err := protocol.OnStreamChanged(ctx, app, stream); err != nil {
			slog.ErrorContext(ctx, "webhook onStreamChanged", "err", err)
		}
	}
	return newDefaultOutputOK(), nil
}

// onPlay rtsp/rtmp/http-flv/ws-flv/hls 播放触发播放器身份验证事件。
// 播放流时会触发此事件。如果流不存在，则首先触发 on_play 事件，然后触发 on_stream_not_found 事件。
// 播放rtsp流时，如果该流开启了rtsp专用认证（on_rtsp_realm），则不会触发on_play事件。
// https://docs.zlmediakit.com/guide/media_server/web_hook_api.html#_6-on-play
func (w WebHookAPI) onPlay(c *gin.Context, in *onPublishInput) (DefaultOutput, error) {
	ctx := c.Request.Context()
	w.log.InfoContext(ctx, "webhook onPlay", "app", in.App, "stream", in.Stream, "schema", in.Schema)

	// 更新通道的播放状态（所有协议统一处理）
	if _, err := w.ipcCore.UpdateChannelPlaying(ctx, in.Stream, true); err != nil {
		w.log.WarnContext(ctx, "更新播放状态失败", "stream", in.Stream, "err", err)
	}

	return newDefaultOutputOK(), nil
}

// onStreamNoneReader 流无人观看时事件，用户可以通过此事件选择是否关闭无人看的流。
// 一个直播流注册上线了，如果一直没人观看也会触发一次无人观看事件，触发时的协议 schema 是随机的，
// 看哪种协议最晚注册(一般为 hls)。
// 后续从有人观看转为无人观看，触发协议 schema 为最后一名观看者使用何种协议。
// 目前 mp4/hls 录制不当做观看人数(mp4 录制可以通过配置文件 mp4_as_player 控制，
// 但是 rtsp/rtmp/rtp 转推算观看人数，也会触发该事件。
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_12%E3%80%81on-stream-changed
func (w WebHookAPI) onStreamNoneReader(c *gin.Context, in *onStreamNoneReaderInput) (onStreamNoneReaderOutput, error) {
	ctx := c.Request.Context()
	w.log.InfoContext(ctx, "webhook onStreamNoneReader", "app", in.App, "stream", in.Stream, "mediaServerID", in.MediaServerID)

	// 禁用录像时，直接关闭流
	if w.uc.Conf.Server.Recording.Disabled {
		// 更新通道的播放状态为未播放（所有协议统一处理）
		if _, err := w.ipcCore.UpdateChannelPlaying(ctx, in.Stream, false); err != nil {
			w.log.WarnContext(ctx, "更新播放状态失败", "stream", in.Stream, "err", err)
		}
		return onStreamNoneReaderOutput{Close: true}, nil
	}

	// 根据录像模式判断是否关闭流：
	// - none(不录制): 无人观看时关闭流
	// - always/ai(有录像计划): 无人观看时保持流不关闭
	ch, err := w.ipcCore.GetChannelByAppStreamOrID(ctx, in.App, in.Stream)
	if err != nil {
		// 找不到通道时默认关闭流
		w.log.WarnContext(ctx, "获取通道失败，默认关闭流", "stream", in.Stream, "err", err)
		return onStreamNoneReaderOutput{Close: true}, nil
	}

	// 如果录像模式为 none，则关闭流；否则保持流不关闭以继续录制
	shouldClose := ch.Ext.IsNoneRecord()
	w.log.InfoContext(ctx, "无人观看判断", "stream", in.Stream, "record_mode", ch.Ext.GetRecordMode(), "close", shouldClose)
	if shouldClose {
		// 更新通道的播放状态为未播放（所有协议统一处理）
		if _, err := w.ipcCore.UpdateChannelPlaying(ctx, in.Stream, false); err != nil {
			w.log.WarnContext(ctx, "更新播放状态失败", "stream", in.Stream, "err", err)
		}
		return onStreamNoneReaderOutput{Close: true}, nil
	}

	return onStreamNoneReaderOutput{Close: false}, nil
}

// onRTPServerTimeout RTP 服务器超时事件
// 调用 openRtpServer 接口，rtp server 长时间未收到数据,执行此 web hook,对回复不敏感
// https://docs.zlmediakit.com/zh/guide/media_server/web_hook_api.html#_17%E3%80%81on-rtp-server-timeout
func (w WebHookAPI) onRTPServerTimeout(c *gin.Context, in *onRTPServerTimeoutInput) (DefaultOutput, error) {
	w.log.InfoContext(c.Request.Context(), "webhook onRTPServerTimeout", "local_port", in.LocalPort, "ssrc", in.SSRC, "stream_id", in.StreamID, "mediaServerID", in.MediaServerID)
	return newDefaultOutputOK(), nil
}

// onStreamNotFound 流不存在事件
// TODO: 重启后立即播放，会出发 "channel not exist" 待处理
func (w WebHookAPI) onStreamNotFound(c *gin.Context, in *onStreamNotFoundInput) (DefaultOutput, error) {
	ctx := c.Request.Context()
	w.log.InfoContext(ctx, "webhook onStreamNotFound", "app", in.App, "stream", in.Stream, "schema", in.Schema, "mediaServerID", in.MediaServerID)

	stream := in.Stream
	app := in.App
	if !(in.Schema == "rtmp" || in.Schema == "rtsp") {
		return newDefaultOutputOK(), nil
	}

	// 通过 app+stream 查询通道获取类型，支持自定义 app/stream
	channelType := w.getChannelType(ctx, app, stream)
	protocol, ok := w.protocols[channelType]
	if ok {
		if err := protocol.OnStreamNotFound(ctx, app, stream); err != nil {
			slog.InfoContext(ctx, "webhook onStreamNotFound", "err", err)
		}
	}

	return newDefaultOutputOK(), nil
}

// onRecordTS 在 HLS-fMP4 分片落盘后将其时间和路径写入索引。
func (w WebHookAPI) onRecordTS(c *gin.Context, in *onRecordTSInput) (DefaultOutput, error) {
	ctx := c.Request.Context()
	ext := strings.ToLower(filepath.Ext(in.FilePath))
	if ext != ".mp4" && ext != ".m4s" {
		return newDefaultOutputOK(), nil
	}
	if !pathInside(in.FilePath, w.conf.Server.Recording.StorageDir) {
		return newDefaultOutputOK(), nil
	}
	// This ZLM build always names fMP4 media segments .mp4. Keep that name for
	// its live manifest and add a same-volume .m4s hard link for OWL VOD. The
	// two names share disk blocks, so this does not duplicate video data.
	if ext == ".mp4" {
		m4sPath := strings.TrimSuffix(in.FilePath, filepath.Ext(in.FilePath)) + ".m4s"
		if err := os.Link(in.FilePath, m4sPath); err != nil && !os.IsExist(err) {
			w.log.ErrorContext(ctx, "创建 m4s 硬链接失败", "source", in.FilePath, "link", m4sPath, "err", err)
			return newDefaultOutputOK(), nil
		}
		in.FilePath = m4sPath
		in.FileName = filepath.Base(m4sPath)
	}
	w.log.InfoContext(ctx, "webhook onRecordTS",
		"app", in.App,
		"stream", in.Stream,
		"file_path", in.FilePath,
		"file_size", in.FileSize,
		"time_len", in.TimeLen,
		"start_time", in.StartTime,
	)

	// 计算相对路径：从配置的存储目录开始
	// filepath.Clean 去除 "./" 前缀，避免 storageDir="./configs/recordings"
	// 与 ZLM 回调的绝对路径 "/opt/.../configs/recordings/..." 匹配失败
	relativePath := in.FilePath
	if w.conf.Server.Recording.StorageDir != "" {
		relativePath = relativeRecordingPath(in.FilePath, filepath.Clean(w.conf.Server.Recording.StorageDir), in.URL)
	}

	// 计算开始和结束时间
	startTime := time.Unix(in.StartTime, 0)
	endTime := startTime.Add(time.Duration(in.TimeLen * float64(time.Second)))

	// 通过 app+stream 查找 channel ID，支持自定义 app/stream
	var cid string
	ch, err := w.ipcCore.GetChannelByAppStreamOrID(ctx, in.App, in.Stream)
	if err == nil {
		cid = ch.ID
	} else {
		// 如果找不到通道，使用 stream 作为 CID 的标识
		cid = in.Stream
		w.log.WarnContext(ctx, "未找到对应通道，使用 stream 作为 CID", "app", in.App, "stream", in.Stream)
	}

	// 入库
	_, err = w.recordingCore.CreateRecording(ctx, &recording.AddRecordingInput{
		CID:       cid,
		App:       in.App,
		Stream:    in.Stream,
		StartedAt: orm.Time{Time: startTime},
		EndedAt:   orm.Time{Time: endTime},
		Duration:  in.TimeLen,
		Path:      strings.TrimLeft(filepath.ToSlash(filepath.Clean(relativePath)), "/"),
		Size:      in.FileSize,
	})
	if err != nil {
		w.log.ErrorContext(ctx, "录像入库失败", "err", err)
		// 仍返回成功，避免 ZLM 重试
	}

	return newDefaultOutputOK(), nil
}

func pathInside(path, root string) bool {
	absPath, err1 := filepath.Abs(path)
	absRoot, err2 := filepath.Abs(root)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// relativeRecordingPath 从 ZLM 回调的绝对路径中截取以存储目录开头的相对路径。
// 为什么按路径分隔符对齐匹配：子串匹配会误中 storageDir 为 "recordings" 时的
// "/data/oldrecordings/x.mp4"，截出错误相对路径；取最右侧出现（LastIndex）
// 以命中真实存储根；匹配不到时回退 fallback
func relativeRecordingPath(filePath, storageDir, fallback string) string {
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	storageDir = strings.Trim(strings.ReplaceAll(storageDir, "\\", "/"), "./")
	fallback = strings.ReplaceAll(fallback, "\\", "/")
	if strings.HasPrefix(filePath, storageDir+"/") {
		return strings.TrimPrefix(filePath, storageDir+"/")
	}
	if idx := strings.LastIndex(filePath, "/"+storageDir+"/"); idx >= 0 {
		return filePath[idx+len(storageDir)+2:]
	}
	return strings.TrimLeft(fallback, "/")
}
