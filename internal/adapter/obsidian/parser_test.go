package obsidian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"obsidian-notify/internal/adapter/config"
)

func TestParserDailyNote(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dailyDir := filepath.Join(root, "Daily")
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dailyDir, "2026-03-21.md")
	content := "## TODO\n- [ ] Brainstorm app 📅 2026-03-22\n- [ ]    \n- [x] Finish docs ✅ 2026-03-21\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	parser := NewParser(NewClassifier(config.AppConfig{DailyNotesDir: "Daily", WeeklyGoalsDir: "Weakly goals"}))
	parsed, err := parser.Parse(config.VaultConfig{ID: 1, Name: "personal", RootPath: root}, path, time.Now())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Document.SourceKind != "daily" {
		t.Fatalf("bad source kind: %s", parsed.Document.SourceKind)
	}
	if !parsed.Document.HasNonBlankTask {
		t.Fatal("expected non blank tasks")
	}
	if len(parsed.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(parsed.Tasks))
	}
	if parsed.Tasks[0].Body != "Brainstorm app" {
		t.Fatalf("bad task body: %q", parsed.Tasks[0].Body)
	}
	if parsed.Tasks[0].LineNumber != 1 {
		t.Fatalf("expected relative line 1, got %d", parsed.Tasks[0].LineNumber)
	}
	if parsed.Tasks[0].DueDate == nil || string(*parsed.Tasks[0].DueDate) != "2026-03-22" {
		t.Fatalf("bad due date: %+v", parsed.Tasks[0].DueDate)
	}
	if parsed.Tasks[1].DoneDate == nil || string(*parsed.Tasks[1].DoneDate) != "2026-03-21" {
		t.Fatalf("bad done date: %+v", parsed.Tasks[1].DoneDate)
	}
}

func TestParserSupportsTasksPluginListPrefixes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dailyDir := filepath.Join(root, "Daily")
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dailyDir, "2026-03-22.md")
	content := strings.Join([]string{
		"## TODO",
		"- [ ] hyphen",
		"* [ ] star",
		"+ [ ] plus",
		"1. [ ] numbered dot",
		"2) [ ] numbered paren",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	parser := NewParser(NewClassifier(config.AppConfig{DailyNotesDir: "Daily", WeeklyGoalsDir: "Weakly goals"}))
	parsed, err := parser.Parse(config.VaultConfig{ID: 1, Name: "personal", RootPath: root}, path, time.Now())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Tasks) != 5 {
		t.Fatalf("expected 5 tasks, got %d", len(parsed.Tasks))
	}
}

func TestClassifierWeeklyNote(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "Weakly goals", "Physical", "W10-2026.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	classification, err := NewClassifier(config.AppConfig{DailyNotesDir: "Daily", WeeklyGoalsDir: "Weakly goals"}).Classify(root, path)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if classification.SourceKind != "weekly" {
		t.Fatalf("bad source kind: %s", classification.SourceKind)
	}
	if classification.WeeklyArea == nil || *classification.WeeklyArea != "Physical" {
		t.Fatalf("bad weekly area: %+v", classification.WeeklyArea)
	}
	if classification.ISOWeekNumber == nil || *classification.ISOWeekNumber != 10 {
		t.Fatalf("bad week: %+v", classification.ISOWeekNumber)
	}
	if classification.ISOWeekYear == nil || *classification.ISOWeekYear != 2026 {
		t.Fatalf("bad year: %+v", classification.ISOWeekYear)
	}
}
