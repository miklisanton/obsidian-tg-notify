package task

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SourceKind string

const (
	SourceKindDaily  SourceKind = "daily"
	SourceKindWeekly SourceKind = "weekly"
	SourceKindOther  SourceKind = "other"
)

type Date string

const dateLayout = "2006-01-02"

func ParseDate(raw string) (Date, error) {
	if _, err := time.Parse(dateLayout, raw); err != nil {
		return "", fmt.Errorf("parse date %q: %w", raw, err)
	}
	return Date(raw), nil
}

func MustParseDate(raw string) Date {
	date, err := ParseDate(raw)
	if err != nil {
		panic(err)
	}
	return date
}

func Today(now time.Time, loc *time.Location) Date {
	return Date(now.In(loc).Format(dateLayout))
}

func (d Date) Time(loc *time.Location) time.Time {
	tm, _ := time.ParseInLocation(dateLayout, string(d), loc)
	return tm
}

func (d Date) ISOWeek(loc *time.Location) (int, int) {
	year, week := d.Time(loc).ISOWeek()
	return year, week
}

type DocumentSnapshot struct {
	VaultID         int64
	SourcePath      string
	SourceKind      SourceKind
	DailyDate       *Date
	ISOWeekYear     *int
	ISOWeekNumber   *int
	WeeklyArea      *string
	HasNonBlankTask bool
	SyncedAt        time.Time
}

type Snapshot struct {
	VaultID       int64
	SourcePath    string
	SourceKind    SourceKind
	LineNumber    int
	Section       string
	Fingerprint   string
	Body          string
	Completed     bool
	DueDate       *Date
	DoneDate      *Date
	DailyDate     *Date
	ISOWeekYear   *int
	ISOWeekNumber *int
	WeeklyArea    *string
	SeenAt        time.Time
	UpdatedAt     time.Time
}

type ParsedFile struct {
	Document DocumentSnapshot
	Tasks    []Snapshot
}

type DueWindow string

const (
	DueWindowToday    DueWindow = "today"
	DueWindowTomorrow DueWindow = "tomorrow"
	DueWindowOverdue  DueWindow = "overdue"
)

type DueFilter struct {
	VaultID int64
	Now     time.Time
	Loc     *time.Location
	Window  DueWindow
}

type DueScope string

const (
	DueScopeAll    DueScope = "all"
	DueScopeDaily  DueScope = "daily"
	DueScopeWeekly DueScope = "weekly"
)

type DueRangeFilter struct {
	VaultID int64
	Scope   DueScope
	From    Date
	To      Date
}

func NormalizeTaskBody(raw string) string {
	trimmed := strings.TrimSpace(raw)
	parts := strings.Fields(trimmed)
	return strings.Join(parts, " ")
}

func BuildFingerprint(relPath string, section string, line int, body string) string {
	value := filepath.ToSlash(relPath) + "|" + strings.TrimSpace(section) + "|" + NormalizeTaskBody(body) + "|" + strconv.Itoa(line)
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
