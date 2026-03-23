package obsidian

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"obsidian-notify/internal/adapter/config"
	"obsidian-notify/internal/domain/task"
)

var weeklyNamePattern = regexp.MustCompile(`^W(\d{2})-(\d{4}).*\.md$`)

type Classification struct {
	SourceKind    task.SourceKind
	SourcePath    string
	DailyDate     *task.Date
	ISOWeekYear   *int
	ISOWeekNumber *int
	WeeklyArea    *string
}

type Classifier struct {
	dailyPathPattern *regexp.Regexp
	weeklyGoalsDir   string
}

func NewClassifier(app config.AppConfig) *Classifier {
	dailyDir := regexp.QuoteMeta(strings.Trim(app.DailyNotesDir, "/"))
	return &Classifier{
		dailyPathPattern: regexp.MustCompile(`^` + dailyDir + `/(\d{4}-\d{2}-\d{2})\.md$`),
		weeklyGoalsDir:   strings.Trim(app.WeeklyGoalsDir, "/"),
	}
}

func (c *Classifier) Classify(root string, absPath string) (Classification, error) {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return Classification{}, fmt.Errorf("relative path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if matches := c.dailyPathPattern.FindStringSubmatch(rel); matches != nil {
		date := task.MustParseDate(matches[1])
		return Classification{SourceKind: task.SourceKindDaily, SourcePath: rel, DailyDate: &date}, nil
	}
	weeklyPrefix := c.weeklyGoalsDir + "/"
	if strings.HasPrefix(rel, weeklyPrefix) {
		pieces := strings.Split(rel, "/")
		classification := Classification{SourceKind: task.SourceKindWeekly, SourcePath: rel}
		if len(pieces) >= 3 {
			area := pieces[1]
			classification.WeeklyArea = &area
		}
		if matches := weeklyNamePattern.FindStringSubmatch(filepath.Base(rel)); matches != nil {
			week := parseInt(matches[1])
			year := parseInt(matches[2])
			classification.ISOWeekNumber = &week
			classification.ISOWeekYear = &year
		}
		return classification, nil
	}
	return Classification{SourceKind: task.SourceKindOther, SourcePath: rel}, nil
}

func (c *Classifier) ListWeeklyAreas(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, c.weeklyGoalsDir))
	if err != nil {
		return nil, err
	}
	areas := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		areas = append(areas, entry.Name())
	}
	sort.Strings(areas)
	return areas, nil
}

func parseInt(raw string) int {
	value := 0
	for _, ch := range raw {
		value = value*10 + int(ch-'0')
	}
	return value
}
