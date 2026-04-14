package reminder

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"obsidian-notify/internal/domain/task"
)

type RuleKind string

const (
	RuleKindNewTask                RuleKind = "new_task"
	RuleKindDueTasks               RuleKind = "due_tasks"
	RuleKindPromptDailyGoals       RuleKind = "prompt_daily_goals"
	RuleKindPromptDailySummary     RuleKind = "prompt_daily_summary"
	RuleKindPromptWeeklyGoals      RuleKind = "prompt_weekly_goals"
	RuleKindReviewWeeklyUnfinished RuleKind = "review_weekly_unfinished"
)

type LocalTime struct {
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
}

func ParseLocalTime(raw string) (LocalTime, error) {
	tm, err := time.Parse("15:04", raw)
	if err != nil {
		return LocalTime{}, fmt.Errorf("parse time %q: %w", raw, err)
	}
	return LocalTime{Hour: tm.Hour(), Minute: tm.Minute()}, nil
}

func (lt LocalTime) String() string {
	return fmt.Sprintf("%02d:%02d", lt.Hour, lt.Minute)
}

type ScheduleSlot struct {
	Weekdays []time.Weekday `json:"weekdays"`
	Time     LocalTime      `json:"time"`
}

func (s ScheduleSlot) Matches(now time.Time) bool {
	if now.Hour() != s.Time.Hour || now.Minute() != s.Time.Minute {
		return false
	}
	if len(s.Weekdays) == 0 {
		return true
	}
	for _, weekday := range s.Weekdays {
		if weekday == now.Weekday() {
			return true
		}
	}
	return false
}

type Schedule struct {
	Slots []ScheduleSlot `json:"slots"`
}

func (s Schedule) Matches(now time.Time) bool {
	for _, slot := range s.Slots {
		if slot.Matches(now) {
			return true
		}
	}
	return false
}

type Config interface {
	Kind() RuleKind
}

type NewTaskConfig struct{}

func (NewTaskConfig) Kind() RuleKind { return RuleKindNewTask }

type DueTasksConfig struct {
	Window task.DueWindow `json:"window"`
}

func (DueTasksConfig) Kind() RuleKind { return RuleKindDueTasks }

type PromptDailyGoalsConfig struct{}

func (PromptDailyGoalsConfig) Kind() RuleKind { return RuleKindPromptDailyGoals }

type PromptDailySummaryConfig struct{}

func (PromptDailySummaryConfig) Kind() RuleKind { return RuleKindPromptDailySummary }

type PromptWeeklyGoalsConfig struct{}

func (PromptWeeklyGoalsConfig) Kind() RuleKind { return RuleKindPromptWeeklyGoals }

type ReviewWeeklyUnfinishedConfig struct{}

func (ReviewWeeklyUnfinishedConfig) Kind() RuleKind { return RuleKindReviewWeeklyUnfinished }

type Rule struct {
	ID        string
	ChatID    int64
	VaultID   int64
	Kind      RuleKind
	Timezone  string
	Schedule  Schedule
	Config    Config
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func MarshalConfig(cfg Config) (json.RawMessage, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return data, nil
}

func UnmarshalConfig(kind RuleKind, data []byte) (Config, error) {
	switch kind {
	case RuleKindNewTask:
		return NewTaskConfig{}, nil
	case RuleKindDueTasks:
		cfg := DueTasksConfig{}
		if len(data) == 0 {
			return cfg, nil
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal %s config: %w", kind, err)
		}
		return cfg, nil
	case RuleKindPromptDailyGoals:
		return PromptDailyGoalsConfig{}, nil
	case RuleKindPromptDailySummary:
		return PromptDailySummaryConfig{}, nil
	case RuleKindPromptWeeklyGoals:
		return PromptWeeklyGoalsConfig{}, nil
	case RuleKindReviewWeeklyUnfinished:
		return ReviewWeeklyUnfinishedConfig{}, nil
	default:
		return nil, fmt.Errorf("unknown rule kind %q", kind)
	}
}

func ParseWeekday(raw string) (time.Weekday, error) {
	switch strings.ToLower(raw) {
	case "sun":
		return time.Sunday, nil
	case "mon":
		return time.Monday, nil
	case "tue":
		return time.Tuesday, nil
	case "wed":
		return time.Wednesday, nil
	case "thu":
		return time.Thursday, nil
	case "fri":
		return time.Friday, nil
	case "sat":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("bad weekday %q", raw)
	}
}
