package api

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gowvp/owl/internal/conf"
	"github.com/gowvp/owl/internal/core/recording"
	"github.com/gowvp/owl/internal/core/recording/stores/recordingdb"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/system"
	"github.com/ixugo/goddd/pkg/web"
	"gorm.io/gorm"
)

// RecordingAPI 为 http 提供业务方法
type RecordingAPI struct {
	recordingCore recording.Core
	conf          *conf.Bootstrap
}

// NewRecordingStore 创建录像存储层
func NewRecordingStore(db *gorm.DB) recording.Storer {
	return recordingdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
}

// NewRecordingCore 创建录像管理核心服务
// 依赖 recording.SMSProvider 接口而非 sms.Core，避免循环依赖
func NewRecordingCore(
	store recording.Storer,
	cfg *conf.Bootstrap,
	provider recording.SMSProvider,
	ipcProvider recording.IPCProvider,
	playProvider recording.PlayProvider,
) recording.Core {
	core := recording.NewCore(store,
		recording.WithConfig(&cfg.Server.Recording),
		recording.WithSMSProvider(provider),
		recording.WithIPCProvider(ipcProvider),
		recording.WithPlayProvider(playProvider),
	)

	// 启动清理协程
	go core.StartCleanupWorker()

	// 启动录制同步协程（平台重启/流中断恢复）
	core.StartRecordingSyncLoop(context.Background())

	return core
}

func NewRecordingAPI(core recording.Core, conf *conf.Bootstrap) RecordingAPI {
	return RecordingAPI{recordingCore: core, conf: conf}
}

// recordingIDInput 录像 ID 路径参数
type recordingPlayOutput struct {
	URL string `json:"url"`
}

func RegisterRecording(g gin.IRouter, api RecordingAPI, handler ...gin.HandlerFunc) {
	{
		group := g.Group("/recordings", handler...)
		group.GET("/play", web.WrapH(api.getPlayURL))
		group.GET("/timeline", web.WrapH(api.getTimeline))
		group.GET("/monthly", web.WrapH(api.getMonthlyStats))
		group.DELETE("", web.WrapH(api.deleteRange))
		group.GET("/download", api.downloadRange)
		// Internal HLS route consumed by players; it is intentionally omitted from OpenAPI.
		group.GET("/channels/:cid/index.m3u8", api.channelPlaylist)
	}

	// 静态文件服务，用于访问录像 MP4 文件
	// 路径格式: /static/recordings/xxx.mp4?token=xxx
	// Gin Static 支持 HTTP Range 请求，实现边下载边播放（秒播）
	if api.conf != nil && api.conf.Server.Recording.StorageDir != "" {
		slog.Info("注册录像静态文件服务", "path", "/static/recordings", "dir", api.conf.Server.Recording.StorageDir)
		g.Group("/static", handler...).Static("/recordings", api.conf.Server.Recording.StorageDir)
	}
}

func (a RecordingAPI) getPlayURL(c *gin.Context, in *recording.RangeInput) (*recordingPlayOutput, error) {
	if in.CID == "" || in.StartMs <= 0 || in.EndMs <= in.StartMs {
		return nil, reason.ErrBadRequest.Withf("cid, start_ms and end_ms are required")
	}
	path := fmt.Sprintf("/recordings/channels/%s/index.m3u8?start_ms=%d&end_ms=%d", url.PathEscape(in.CID), in.StartMs, in.EndMs)
	if token, ok := c.Get(web.KeyTokenString); ok && token != "" {
		path += "&token=" + url.QueryEscape(fmt.Sprint(token))
	}
	return &recordingPlayOutput{URL: web.WithContext(c.Request).BaseURLJoin(path)}, nil
}

// getTimeline 获取时间轴数据
func (a RecordingAPI) getTimeline(c *gin.Context, in *recording.TimelineInput) (any, error) {
	items, err := a.recordingCore.GetTimeline(c.Request.Context(), in)
	return gin.H{"items": items}, err
}

func (a RecordingAPI) deleteRange(c *gin.Context, in *recording.RangeInput) (*recording.DeleteRangeOutput, error) {
	return a.recordingCore.DeleteRange(c.Request.Context(), in)
}

// getMonthlyStats 获取月度录像统计
func (a RecordingAPI) getMonthlyStats(c *gin.Context, in *recording.MonthlyStatsInput) (*recording.MonthlyStatsOutput, error) {
	return a.recordingCore.GetMonthlyStats(c.Request.Context(), in)
}

func (a RecordingAPI) downloadRange(c *gin.Context) {
	in := recording.RangeInput{CID: c.Query("cid")}
	in.StartMs, _ = strconv.ParseInt(c.Query("start_ms"), 10, 64)
	in.EndMs, _ = strconv.ParseInt(c.Query("end_ms"), 10, 64)
	if in.CID == "" || in.StartMs <= 0 || in.EndMs <= in.StartMs {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "cid, start_ms and end_ms are required"})
		return
	}
	items, _, err := a.recordingCore.ListRecordings(c.Request.Context(), &recording.FindRecordingInput{CID: in.CID, Page: 1, Size: 10000, StartMs: in.StartMs, EndMs: in.EndMs})
	if err != nil || len(items) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "no recordings found in time range"})
		return
	}
	segments := items[:0]
	for _, item := range items {
		if strings.EqualFold(filepath.Ext(item.Path), ".m4s") {
			segments = append(segments, item)
		}
	}
	if len(segments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "no HLS-fMP4 recordings found"})
		return
	}

	tmpDir, err := os.MkdirTemp("", "owl-recording-export-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	defer os.RemoveAll(tmpDir)
	manifest := filepath.Join(tmpDir, "input.m3u8")
	output := filepath.Join(tmpDir, fmt.Sprintf("%s-%d-%d.mp4", in.CID, in.StartMs, in.EndMs))
	if err = os.WriteFile(manifest, []byte(a.generateLocalFMP4Playlist(segments)), 0o600); err != nil {
		c.JSON(500, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	ffmpeg := filepath.Join(system.Getwd(), "ffmpeg.exe")
	if _, statErr := os.Stat(ffmpeg); statErr != nil {
		ffmpeg = "ffmpeg"
	}
	cmd := exec.CommandContext(c.Request.Context(), ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-protocol_whitelist", "file,crypto,data", "-allowed_extensions", "ALL", "-i", manifest, "-map", "0", "-c", "copy", "-movflags", "+faststart", output)
	if data, runErr := cmd.CombinedOutput(); runErr != nil {
		slog.ErrorContext(c.Request.Context(), "export recording failed", "err", runErr, "output", string(data))
		c.JSON(500, gin.H{"code": 1, "msg": "export recording failed"})
		return
	}
	c.FileAttachment(output, filepath.Base(output))
}

// channelPlaylist 生成 HLS m3u8 播放列表
// 根据通道 ID 和时间范围聚合原生 fMP4 分片。
// 路径: /recordings/channels/:cid/index.m3u8?start_ms=xxx&end_ms=xxx&token=xxx
func (a RecordingAPI) channelPlaylist(c *gin.Context) {
	cid := c.Param("cid")
	if cid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "cid is required"})
		return
	}

	startMs, _ := strconv.ParseInt(c.Query("start_ms"), 10, 64)
	endMs, _ := strconv.ParseInt(c.Query("end_ms"), 10, 64)
	token := c.Query("token")

	if startMs <= 0 || endMs <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "start_ms and end_ms are required"})
		return
	}

	// 获取时间范围内的录像列表（需要完整路径信息）
	recordings, _, err := a.recordingCore.ListRecordings(c.Request.Context(), &recording.FindRecordingInput{
		CID:  cid,
		Page: 1, Size: 10000,
		StartMs: startMs, EndMs: endMs,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	if len(recordings) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "no recordings found in time range"})
		return
	}
	segments := recordings[:0]
	for _, item := range recordings {
		if strings.EqualFold(filepath.Ext(item.Path), ".m4s") {
			segments = append(segments, item)
		}
	}
	if len(segments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "no HLS-fMP4 recordings found in time range"})
		return
	}

	m3u8Content := a.generateFMP4Playlist(segments, token)

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, m3u8Content)
}

func (a RecordingAPI) generateFMP4Playlist(segments []*recording.Recording, token string) string {
	if len(segments) == 0 {
		return ""
	}

	sort.Slice(segments, func(i, j int) bool { return segments[i].StartedAt.Before(segments[j].StartedAt.Time) })
	targetDuration := 1
	for _, segment := range segments {
		targetDuration = max(targetDuration, int(math.Ceil(segment.Duration)))
	}

	mediaURL := func(relativePath string) string {
		u := "/static/recordings/" + strings.TrimLeft(filepath.ToSlash(relativePath), "/")
		if token != "" {
			u += "?token=" + url.QueryEscape(token)
		}
		return u
	}

	var out strings.Builder
	fmt.Fprintf(&out, "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n", targetDuration)
	var previous *recording.Recording
	previousInit := ""
	for _, segment := range segments {
		initPath := a.findInitSegment(segment.Path)
		newTimeline := previous == nil || initPath != previousInit
		if previous != nil {
			expected := previous.EndedAt.Time
			gap := segment.StartedAt.Sub(expected)
			// Callback wall-clock values have second precision. Only treat a large
			// jump as a reset; ordinary rounding must not split a healthy timeline.
			if gap > 5*time.Second || gap < -5*time.Second {
				newTimeline = true
			}
		}
		if previous != nil && newTimeline {
			out.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		if newTimeline {
			fmt.Fprintf(&out, "#EXT-X-MAP:URI=\"%s\"\n", mediaURL(initPath))
		}
		fmt.Fprintf(&out, "#EXT-X-PROGRAM-DATE-TIME:%s\n#EXTINF:%.6f,\n%s\n",
			segment.StartedAt.UTC().Format(time.RFC3339Nano), segment.Duration, mediaURL(segment.Path))
		previous, previousInit = segment, initPath
	}
	out.WriteString("#EXT-X-ENDLIST\n")
	return out.String()
}

// generateLocalFMP4Playlist builds a temporary FFmpeg input manifest. Each
// media session keeps its own init segment and discontinuity boundary.
func (a RecordingAPI) generateLocalFMP4Playlist(segments []*recording.Recording) string {
	sort.Slice(segments, func(i, j int) bool { return segments[i].StartedAt.Before(segments[j].StartedAt.Time) })
	targetDuration := 1
	for _, segment := range segments {
		targetDuration = max(targetDuration, int(math.Ceil(segment.Duration)))
	}
	fileURI := func(path string) string {
		absolute, _ := filepath.Abs(path)
		return (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(absolute)}).String()
	}
	var out strings.Builder
	fmt.Fprintf(&out, "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n", targetDuration)
	var previous *recording.Recording
	previousInit := ""
	for _, segment := range segments {
		initRelative := a.findInitSegment(segment.Path)
		newTimeline := previous == nil || initRelative != previousInit
		if previous != nil {
			gap := segment.StartedAt.Sub(previous.EndedAt.Time)
			if gap > 5*time.Second || gap < -5*time.Second {
				newTimeline = true
			}
		}
		if previous != nil && newTimeline {
			out.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		if newTimeline {
			fmt.Fprintf(&out, "#EXT-X-MAP:URI=\"%s\"\n", fileURI(a.recordingCore.GetFullPath(initRelative)))
		}
		fmt.Fprintf(&out, "#EXTINF:%.6f,\n%s\n", segment.Duration, fileURI(a.recordingCore.GetFullPath(segment.Path)))
		previous, previousInit = segment, initRelative
	}
	out.WriteString("#EXT-X-ENDLIST\n")
	return out.String()
}

func (a RecordingAPI) findInitSegment(segmentPath string) string {
	if a.conf == nil || a.conf.Server.Recording.StorageDir == "" {
		return filepath.ToSlash(filepath.Join(filepath.Dir(segmentPath), "init.mp4"))
	}
	full := a.recordingCore.GetFullPath(segmentPath)
	root := filepath.Clean(a.conf.Server.Recording.StorageDir)
	for dir := filepath.Dir(full); pathInside(dir, root); dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "init.mp4")
		if _, err := os.Stat(candidate); err == nil {
			if relative, err := filepath.Rel(root, candidate); err == nil {
				return filepath.ToSlash(relative)
			}
		}
		if filepath.Clean(dir) == root {
			break
		}
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(segmentPath), "init.mp4"))
}
