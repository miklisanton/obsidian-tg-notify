package obsidian

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"obsidian-notify/internal/adapter/config"
	"obsidian-notify/internal/domain/task"
)

var checkboxPattern = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[( |x|X)\]\s*(.*)$`)

type Parser struct {
	classifier         *Classifier
	dailySummaryHeader string
}

func NewParser(classifier *Classifier) *Parser {
	return &Parser{classifier: classifier, dailySummaryHeader: "Today's summary"}
}

func NewParserWithConfig(app config.AppConfig) *Parser {
	return &Parser{classifier: NewClassifier(app), dailySummaryHeader: app.DailySummaryHeader}
}

func (p *Parser) Parse(vault config.VaultConfig, absPath string, now time.Time) (task.ParsedFile, error) {
	classification, err := p.classifier.Classify(vault.RootPath, absPath)
	if err != nil {
		return task.ParsedFile{}, err
	}

	file, err := os.Open(absPath)
	if err != nil {
		return task.ParsedFile{}, fmt.Errorf("open %s: %w", absPath, err)
	}
	defer file.Close()

	parsed := task.ParsedFile{
		Document: task.DocumentSnapshot{
			VaultID:       vault.ID,
			SourcePath:    classification.SourcePath,
			SourceKind:    classification.SourceKind,
			DailyDate:     classification.DailyDate,
			ISOWeekYear:   classification.ISOWeekYear,
			ISOWeekNumber: classification.ISOWeekNumber,
			WeeklyArea:    classification.WeeklyArea,
			SyncedAt:      now,
		},
	}

	scanner := bufio.NewScanner(file)
	section := ""
	sectionStartLine := 0
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if strings.HasPrefix(text, "## ") {
			section = strings.TrimSpace(strings.TrimPrefix(text, "## "))
			sectionStartLine = line
			continue
		}
		if section == p.dailySummaryHeader && strings.TrimSpace(text) != "" {
			parsed.Document.HasDailySummary = true
		}
		matches := checkboxPattern.FindStringSubmatch(text)
		if matches == nil {
			continue
		}
		body := matches[2]
		parsedBody, dueDate, doneDate := parseTaskMetadata(body)
		normalized := task.NormalizeTaskBody(parsedBody)
		if normalized == "" {
			continue
		}
		relativeLine := line
		if sectionStartLine > 0 {
			relativeLine = line - sectionStartLine
		}
		parsed.Document.HasNonBlankTask = true
		parsed.Tasks = append(parsed.Tasks, task.Snapshot{
			VaultID:       vault.ID,
			SourcePath:    classification.SourcePath,
			SourceKind:    classification.SourceKind,
			LineNumber:    relativeLine,
			Section:       section,
			Fingerprint:   task.BuildFingerprint(classification.SourcePath, section, relativeLine, normalized),
			Body:          normalized,
			Completed:     strings.EqualFold(matches[1], "x"),
			DueDate:       dueDate,
			DoneDate:      doneDate,
			DailyDate:     classification.DailyDate,
			ISOWeekYear:   classification.ISOWeekYear,
			ISOWeekNumber: classification.ISOWeekNumber,
			WeeklyArea:    classification.WeeklyArea,
			SeenAt:        now,
			UpdatedAt:     now,
		})
	}
	if err := scanner.Err(); err != nil {
		return task.ParsedFile{}, err
	}
	_ = filepath.Ext(absPath)
	return parsed, nil
}
func parseTaskMetadata(body string) (string, *task.Date, *task.Date) {
	trimmed := strings.TrimSpace(body)
	var dueDate *task.Date
	var doneDate *task.Date
	if index := strings.Index(trimmed, "📅 "); index >= 0 && len(trimmed) >= index+12 {
		candidate := strings.TrimSpace(trimmed[index+len("📅 "):])
		if len(candidate) >= 10 {
			date, err := task.ParseDate(candidate[:10])
			if err == nil {
				dueDate = &date
				trimmed = strings.TrimSpace(strings.Replace(trimmed, "📅 "+candidate[:10], "", 1))
			}
		}
	}
	if index := strings.Index(trimmed, "✅ "); index >= 0 && len(trimmed) >= index+12 {
		candidate := strings.TrimSpace(trimmed[index+len("✅ "):])
		if len(candidate) >= 10 {
			date, err := task.ParseDate(candidate[:10])
			if err == nil {
				doneDate = &date
				trimmed = strings.TrimSpace(strings.Replace(trimmed, "✅ "+candidate[:10], "", 1))
			}
		}
	}
	return strings.TrimSpace(trimmed), dueDate, doneDate
}
