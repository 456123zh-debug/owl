package api

import (
	"strings"
	"testing"
	"time"

	"github.com/gowvp/owl/internal/core/recording"
	"github.com/ixugo/goddd/pkg/orm"
)

func TestGenerateFMP4PlaylistTimelineBoundaries(t *testing.T) {
	start := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	segments := []*recording.Recording{
		{Path: "pull/cam/2026-09-17/session-a/0001.m4s", StartedAt: orm.Time{Time: start}, EndedAt: orm.Time{Time: start.Add(2 * time.Second)}, Duration: 2},
		{Path: "pull/cam/2026-09-17/session-a/0002.m4s", StartedAt: orm.Time{Time: start.Add(2 * time.Second)}, EndedAt: orm.Time{Time: start.Add(4 * time.Second)}, Duration: 2},
		{Path: "pull/cam/2026-09-17/session-b/0001.m4s", StartedAt: orm.Time{Time: start.Add(10 * time.Second)}, EndedAt: orm.Time{Time: start.Add(13 * time.Second)}, Duration: 3},
	}

	got := (RecordingAPI{}).generateFMP4Playlist(segments, "Bearer token")
	for _, want := range []string{
		"#EXT-X-VERSION:7",
		"#EXT-X-TARGETDURATION:3",
		"#EXT-X-MAP:URI=\"/static/recordings/pull/cam/2026-09-17/session-a/init.mp4?token=Bearer+token\"",
		"#EXT-X-DISCONTINUITY",
		"#EXT-X-MAP:URI=\"/static/recordings/pull/cam/2026-09-17/session-b/init.mp4?token=Bearer+token\"",
		"#EXT-X-ENDLIST",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("playlist missing %q:\n%s", want, got)
		}
	}
	if count := strings.Count(got, "#EXT-X-DISCONTINUITY"); count != 1 {
		t.Fatalf("discontinuity count = %d, want 1:\n%s", count, got)
	}
}
