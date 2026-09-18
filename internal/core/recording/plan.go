package recording

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	// Embed the IANA timezone database so Windows builds can resolve names such
	// as Asia/Shanghai even when the host has no system tzdata installed.
	_ "time/tzdata"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
	"gorm.io/gorm"
)

const (
	RecordTypeContinuous = "continuous"
	RecordTypeEvent      = "event"
)

type SchedulePeriod struct {
	Days  []int  `json:"days"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type WeeklySchedule []SchedulePeriod

func (s *WeeklySchedule) Scan(value any) error        { return orm.JSONUnmarshal(value, s) }
func (s WeeklySchedule) Value() (driver.Value, error) { return json.Marshal(s) }

type EventConfig struct {
	EventTypes           []string `json:"event_types"`
	MinConfidence        float32  `json:"min_confidence"`
	PostRecordSeconds    int      `json:"post_record_seconds"`
	MergeIntervalSeconds int      `json:"merge_interval_seconds"`
}

func (e *EventConfig) Scan(value any) error        { return orm.JSONUnmarshal(value, e) }
func (e EventConfig) Value() (driver.Value, error) { return json.Marshal(e) }

type RecordingPlan struct {
	ID             int64          `gorm:"primaryKey" json:"id"`
	Name           string         `gorm:"notNull;uniqueIndex" json:"name"`
	Enabled        bool           `gorm:"notNull;default:true" json:"enabled"`
	Timezone       string         `gorm:"notNull;default:'Asia/Shanghai'" json:"timezone"`
	RecordType     string         `gorm:"notNull;index" json:"record_type"`
	AIEnabled      bool           `gorm:"column:ai_enabled;notNull;default:false" json:"ai_enabled"`
	WeeklySchedule WeeklySchedule `gorm:"column:weekly_schedule;notNull;type:jsonb" json:"weekly_schedule"`
	RetentionDays  int            `gorm:"notNull;default:7" json:"retention_days"`
	EventConfig    EventConfig    `gorm:"column:event_config;notNull;type:jsonb" json:"event_config"`
	CreatedAt      orm.Time       `gorm:"notNull;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      orm.Time       `gorm:"notNull;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (*RecordingPlan) TableName() string { return "recording_plans" }

type ChannelRecordingPlan struct {
	ChannelID string        `gorm:"column:channel_id;primaryKey" json:"channel_id"`
	PlanID    int64         `gorm:"column:plan_id;notNull;index" json:"plan_id"`
	CreatedAt orm.Time      `gorm:"notNull;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt orm.Time      `gorm:"notNull;default:CURRENT_TIMESTAMP" json:"updated_at"`
	Plan      RecordingPlan `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
}

func (*ChannelRecordingPlan) TableName() string { return "channel_recording_plans" }

type PlanInput struct {
	Name           string         `json:"name"`
	Enabled        bool           `json:"enabled"`
	Timezone       string         `json:"timezone"`
	RecordType     string         `json:"record_type"`
	AIEnabled      bool           `json:"ai_enabled"`
	WeeklySchedule WeeklySchedule `json:"weekly_schedule"`
	RetentionDays  int            `json:"retention_days"`
	EventConfig    EventConfig    `json:"event_config"`
}

func validatePlan(in *PlanInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return errors.New("name is required")
	}
	if in.Timezone == "" {
		in.Timezone = "Asia/Shanghai"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	if in.RecordType != RecordTypeContinuous && in.RecordType != RecordTypeEvent {
		return errors.New("record_type must be continuous or event")
	}
	if in.RetentionDays < 1 || in.RetentionDays > 3650 {
		return errors.New("retention_days must be between 1 and 3650")
	}
	if len(in.WeeklySchedule) == 0 {
		return errors.New("weekly_schedule is required")
	}
	for i, p := range in.WeeklySchedule {
		if len(p.Days) == 0 {
			return fmt.Errorf("weekly_schedule[%d].days is required", i)
		}
		for _, day := range p.Days {
			if day < 1 || day > 7 {
				return fmt.Errorf("weekly_schedule[%d] contains invalid day", i)
			}
		}
		if _, err := parseClock(p.Start); err != nil {
			return fmt.Errorf("weekly_schedule[%d].start: %w", i, err)
		}
		if _, err := parseClock(p.End); err != nil {
			return fmt.Errorf("weekly_schedule[%d].end: %w", i, err)
		}
		if p.Start == p.End {
			return fmt.Errorf("weekly_schedule[%d] start and end cannot be equal", i)
		}
	}
	if in.RecordType == RecordTypeEvent {
		if in.EventConfig.PostRecordSeconds < 1 || in.EventConfig.PostRecordSeconds > 3600 {
			return errors.New("event_config.post_record_seconds must be between 1 and 3600")
		}
		if in.EventConfig.MinConfidence < 0 || in.EventConfig.MinConfidence > 1 {
			return errors.New("event_config.min_confidence must be between 0 and 1")
		}
	}
	return nil
}

func parseClock(value string) (int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, errors.New("must use HH:mm")
	}
	return t.Hour()*60 + t.Minute(), nil
}

func (p *RecordingPlan) ActiveAt(now time.Time) bool {
	if p == nil || !p.Enabled {
		return false
	}
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return false
	}
	local := now.In(loc)
	day := int(local.Weekday())
	if day == 0 {
		day = 7
	}
	minute := local.Hour()*60 + local.Minute()
	previous := day - 1
	if previous == 0 {
		previous = 7
	}
	for _, period := range p.WeeklySchedule {
		start, e1 := parseClock(period.Start)
		end, e2 := parseClock(period.End)
		if e1 != nil || e2 != nil {
			continue
		}
		if start < end && containsDay(period.Days, day) && minute >= start && minute < end {
			return true
		}
		if start > end && ((containsDay(period.Days, day) && minute >= start) || (containsDay(period.Days, previous) && minute < end)) {
			return true
		}
	}
	return false
}

func containsDay(days []int, day int) bool {
	for _, d := range days {
		if d == day {
			return true
		}
	}
	return false
}

func (c Core) CreatePlan(ctx context.Context, in *PlanInput) (*RecordingPlan, error) {
	if err := validatePlan(in); err != nil {
		return nil, reason.ErrBadRequest.Withf(err.Error())
	}
	if in.RecordType == RecordTypeEvent && !in.AIEnabled {
		return nil, reason.ErrBadRequest.WithMsg("event record type requires ai_enabled=true")
	}
	p := &RecordingPlan{Name: in.Name, Enabled: in.Enabled, Timezone: in.Timezone, RecordType: in.RecordType, AIEnabled: in.AIEnabled, WeeklySchedule: in.WeeklySchedule, RetentionDays: in.RetentionDays, EventConfig: in.EventConfig}
	err := c.store.Recording().Session(ctx, func(db *gorm.DB) error { return db.Create(p).Error })
	return p, err
}

func (c Core) UpdatePlan(ctx context.Context, id int64, in *PlanInput) (*RecordingPlan, error) {
	if err := validatePlan(in); err != nil {
		return nil, reason.ErrBadRequest.Withf(err.Error())
	}
	p := &RecordingPlan{ID: id}
	err := c.store.Recording().Session(ctx, func(db *gorm.DB) error {
		return db.Model(p).Updates(map[string]any{"name": in.Name, "enabled": in.Enabled, "timezone": in.Timezone, "record_type": in.RecordType, "ai_enabled": in.AIEnabled, "weekly_schedule": in.WeeklySchedule, "retention_days": in.RetentionDays, "event_config": in.EventConfig}).First(p).Error
	})
	return p, err
}

func (c Core) ListPlans(ctx context.Context) ([]RecordingPlan, error) {
	var out []RecordingPlan
	err := c.store.Recording().Session(ctx, func(db *gorm.DB) error { return db.Order("id ASC").Find(&out).Error })
	return out, err
}

func (c Core) GetPlan(ctx context.Context, id int64) (*RecordingPlan, error) {
	var out RecordingPlan
	err := c.store.Recording().Session(ctx, func(db *gorm.DB) error { return db.First(&out, id).Error })
	return &out, err
}

func (c Core) DeletePlan(ctx context.Context, id int64) error {
	return c.store.Recording().Session(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("plan_id = ?", id).Delete(&ChannelRecordingPlan{}).Error; err != nil {
				return err
			}
			return tx.Delete(&RecordingPlan{}, id).Error
		})
	})
}

func (c Core) BindPlan(ctx context.Context, channelID string, planID int64) (*ChannelRecordingPlan, error) {
	var out ChannelRecordingPlan
	err := c.store.Recording().Session(ctx, func(db *gorm.DB) error {
		var count int64
		if err := db.Model(&RecordingPlan{}).Where("id = ?", planID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		out = ChannelRecordingPlan{ChannelID: channelID, PlanID: planID}
		return db.Save(&out).Preload("Plan").First(&out, "channel_id = ?", channelID).Error
	})
	return &out, err
}

func (c Core) UnbindPlan(ctx context.Context, channelID string) error {
	return c.store.Recording().Session(ctx, func(db *gorm.DB) error {
		return db.Where("channel_id = ?", channelID).Delete(&ChannelRecordingPlan{}).Error
	})
}

func (c Core) GetChannelPlan(ctx context.Context, channelID string) (*ChannelRecordingPlan, error) {
	var out ChannelRecordingPlan
	err := c.store.Recording().Session(ctx, func(db *gorm.DB) error { return db.Preload("Plan").First(&out, "channel_id = ?", channelID).Error })
	return &out, err
}

func (c Core) ResolvePlan(ctx context.Context, channelID string, now time.Time) (*RecordingPlan, bool) {
	binding, err := c.GetChannelPlan(ctx, channelID)
	if err != nil {
		return nil, false
	}
	return &binding.Plan, binding.Plan.ActiveAt(now)
}

func (c Core) MatchEvent(ctx context.Context, channelID, eventType string, confidence float32, now time.Time) (*RecordingPlan, bool) {
	p, active := c.ResolvePlan(ctx, channelID, now)
	if !active || !p.AIEnabled || confidence < p.EventConfig.MinConfidence {
		return p, false
	}
	if len(p.EventConfig.EventTypes) == 0 {
		return p, true
	}
	types := append([]string(nil), p.EventConfig.EventTypes...)
	sort.Strings(types)
	i := sort.SearchStrings(types, eventType)
	return p, i < len(types) && types[i] == eventType
}

// TriggerEvent starts an event recording or extends its stop deadline. Calls for
// multiple detections from the same frame collapse into one recording window.
func (c Core) TriggerEvent(ctx context.Context, channelID, app, stream, eventType string, confidence float32, now time.Time) (bool, error) {
	p, active := c.ResolvePlan(ctx, channelID, now)
	if !active || !p.AIEnabled {
		return false, nil
	}
	// Continuous recording already contains the video. AI events are stored by
	// the event domain and must not start a second recording session.
	if p.RecordType == RecordTypeContinuous {
		return true, nil
	}
	_, matched := c.MatchEvent(ctx, channelID, eventType, confidence, now)
	if !matched {
		return false, nil
	}
	key := app + "/" + stream
	delay := time.Duration(p.EventConfig.PostRecordSeconds+p.EventConfig.MergeIntervalSeconds) * time.Second
	c.eventState.mu.Lock()
	defer c.eventState.mu.Unlock()
	if timer := c.eventState.timers[key]; timer != nil {
		timer.Stop()
	} else if err := c.smsProvider.StartRecording(app, stream); err != nil {
		return false, err
	}
	c.eventState.timers[key] = time.AfterFunc(delay, func() {
		plan, active := c.ResolvePlan(context.Background(), channelID, time.Now())
		if !active || plan.RecordType != RecordTypeContinuous {
			_ = c.smsProvider.StopRecording(app, stream)
		}
		c.eventState.mu.Lock()
		delete(c.eventState.timers, key)
		c.eventState.mu.Unlock()
	})
	return true, nil
}

func (c Core) IsEventRecording(app, stream string) bool {
	if c.eventState == nil {
		return false
	}
	c.eventState.mu.Lock()
	defer c.eventState.mu.Unlock()
	_, ok := c.eventState.timers[app+"/"+stream]
	return ok
}
