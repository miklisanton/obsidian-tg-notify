package syncer

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"obsidian-notify/internal/adapter/config"
	"obsidian-notify/internal/app/ports"
	"obsidian-notify/internal/domain/notification"
	"obsidian-notify/internal/domain/reminder"
	"obsidian-notify/internal/domain/task"
)

type Parser interface {
	Parse(vault config.VaultConfig, absPath string, now time.Time) (task.ParsedFile, error)
}

type Service struct {
	tasks          ports.TaskRepository
	rules          ports.ReminderRepository
	notifications  ports.NotificationRepository
	sender         ports.TelegramSender
	parser         Parser
	matchThreshold float64
}

func NewService(tasks ports.TaskRepository, rules ports.ReminderRepository, notifications ports.NotificationRepository, sender ports.TelegramSender, parser Parser, matchThreshold float64) *Service {
	return &Service{tasks: tasks, rules: rules, notifications: notifications, sender: sender, parser: parser, matchThreshold: matchThreshold}
}

func (s *Service) SyncAll(ctx context.Context, vaults []config.VaultConfig) error {
	for _, vault := range vaults {
		if err := filepath.Walk(vault.RootPath, func(path string, _ fs.FileInfo, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					return nil
				}
				return err
			}
			if strings.HasPrefix(filepath.Base(path), ".") {
				info, statErr := os.Stat(path)
				if statErr == nil && info.IsDir() && path != vault.RootPath {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".md" {
				return nil
			}
			return s.SyncFile(ctx, vault, path, time.Now())
		}); err != nil {
			return fmt.Errorf("walk vault %s: %w", vault.RootPath, err)
		}
	}
	return nil
}

func (s *Service) SyncFile(ctx context.Context, vault config.VaultConfig, absPath string, now time.Time) error {
	parsed, err := s.parser.Parse(vault, absPath, now)
	if err != nil {
		return err
	}

	previous, err := s.tasks.ListFileTasks(ctx, vault.ID, parsed.Document.SourcePath)
	if err != nil {
		return err
	}

	if err := s.tasks.ReplaceFile(ctx, parsed.Document, parsed.Tasks); err != nil {
		return err
	}

	return s.notifyNewTasks(ctx, vault.ID, previous, parsed.Tasks)
}

func (s *Service) notifyNewTasks(ctx context.Context, vaultID int64, previous []task.Snapshot, current []task.Snapshot) error {
	rules, err := s.rules.ListActive(ctx)
	if err != nil {
		return err
	}
	hasRule := false
	for _, rule := range rules {
		if rule.Kind == reminder.RuleKindNewTask && rule.Enabled && rule.VaultID == vaultID {
			hasRule = true
			break
		}
	}
	if !hasRule {
		return nil
	}

	sort.Slice(current, func(i, j int) bool {
		return current[i].LineNumber < current[j].LineNumber
	})
	created := createdTasks(previous, current, s.matchThreshold)
	if len(created) == 0 {
		return nil
	}

	lines := make([]string, 0, len(created)+1)
	lines = append(lines, fmt.Sprintf("%d new task(s) in %s:", len(created), created[0].SourcePath))
	hashes := make([]string, 0, len(created))
	for _, item := range created {
		lines = append(lines, "- "+item.Body)
		hashes = append(hashes, shortFingerprint(item.Fingerprint))
	}
	message := strings.Join(lines, "\n")
	batchKey := strings.Join(hashes, ",")

	for _, rule := range rules {
		if rule.Kind != reminder.RuleKindNewTask || !rule.Enabled || rule.VaultID != vaultID {
			continue
		}
		intent := notification.Intent{
			DedupeKey: fmt.Sprintf("new_task:%s:%d:%s", rule.ID, rule.ChatID, batchKey),
			ChatID:    rule.ChatID,
			RuleID:    rule.ID,
			Kind:      rule.Kind,
			Text:      message,
		}
		if err := SendIntent(ctx, s.notifications, s.sender, intent, time.Now()); err != nil {
			return err
		}
	}

	return nil
}

func createdTasks(previous []task.Snapshot, current []task.Snapshot, matchThreshold float64) []task.Snapshot {
	matchedCurrent := alignedCurrentMatches(previous, current, matchThreshold)
	created := make([]task.Snapshot, 0, len(current))
	for index, item := range current {
		if item.Completed || matchedCurrent[index] {
			continue
		}
		created = append(created, item)
	}
	return created
}

func alignedCurrentMatches(previous []task.Snapshot, current []task.Snapshot, matchThreshold float64) []bool {
	old := sortedSnapshots(previous)
	new := sortedSnapshots(current)
	rows := len(old)
	cols := len(new)
	matches := make([]bool, len(current))
	if rows == 0 || cols == 0 {
		return matches
	}

	scores := make([][]float64, rows+1)
	steps := make([][]byte, rows+1)
	for i := range scores {
		scores[i] = make([]float64, cols+1)
		steps[i] = make([]byte, cols+1)
	}

	for i := 1; i <= rows; i++ {
		for j := 1; j <= cols; j++ {
			bestScore := scores[i-1][j]
			bestStep := byte('u')
			if scores[i][j-1] > bestScore {
				bestScore = scores[i][j-1]
				bestStep = 'l'
			}
			if matchScore := taskMatchScore(old[i-1], new[j-1]); matchScore >= matchThreshold {
				diagScore := scores[i-1][j-1] + matchScore
				if diagScore > bestScore {
					bestScore = diagScore
					bestStep = 'd'
				}
			}
			scores[i][j] = bestScore
			steps[i][j] = bestStep
		}
	}

	i := rows
	j := cols
	for i > 0 && j > 0 {
		switch steps[i][j] {
		case 'd':
			if originalIndex := originalTaskIndex(current, new[j-1]); originalIndex >= 0 {
				matches[originalIndex] = true
			}
			i--
			j--
		case 'u':
			i--
		case 'l':
			j--
		default:
			j--
		}
	}

	return matches
}

func taskMatchScore(previous task.Snapshot, current task.Snapshot) float64 {
	textSimilarity := similarity(previous.Body, current.Body)
	if textSimilarity < 0.45 {
		return 0
	}
	score := 0.60 * textSimilarity
	if previous.Section == current.Section {
		score += 0.20
	}
	if sameDate(previous.DueDate, current.DueDate) {
		score += 0.10
	}
	if previous.Completed == current.Completed {
		score += 0.05
	}
	lineDistance := previous.LineNumber - current.LineNumber
	if lineDistance < 0 {
		lineDistance = -lineDistance
	}
	if lineDistance == 0 {
		score += 0.05
	} else if lineDistance == 1 {
		score += 0.03
	}
	return score
}

func sameDate(left *task.Date, right *task.Date) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func similarity(left string, right string) float64 {
	leftTokens := tokenSet(left)
	rightTokens := tokenSet(right)
	if len(leftTokens) == 0 && len(rightTokens) == 0 {
		return 1
	}
	intersection := 0
	union := make(map[string]struct{}, len(leftTokens)+len(rightTokens))
	for token := range leftTokens {
		union[token] = struct{}{}
		if _, ok := rightTokens[token]; ok {
			intersection++
		}
	}
	for token := range rightTokens {
		union[token] = struct{}{}
	}
	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}

func sortedSnapshots(items []task.Snapshot) []task.Snapshot {
	clone := append([]task.Snapshot(nil), items...)
	sort.Slice(clone, func(i, j int) bool {
		if clone[i].Section != clone[j].Section {
			return clone[i].Section < clone[j].Section
		}
		if clone[i].LineNumber != clone[j].LineNumber {
			return clone[i].LineNumber < clone[j].LineNumber
		}
		return clone[i].Body < clone[j].Body
	})
	return clone
}

func originalTaskIndex(current []task.Snapshot, target task.Snapshot) int {
	for index, item := range current {
		if item.Fingerprint == target.Fingerprint {
			return index
		}
	}
	return -1
}

func tokenSet(value string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, token := range strings.Fields(strings.ToLower(value)) {
		set[token] = struct{}{}
	}
	return set
}

func shortFingerprint(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func SendIntent(ctx context.Context, repo ports.NotificationRepository, sender ports.TelegramSender, intent notification.Intent, now time.Time) error {
	sent, err := repo.WasSent(ctx, intent.DedupeKey)
	if err != nil {
		return err
	}
	if sent {
		return nil
	}
	if err := sender.Send(ctx, intent.ChatID, intent.Text); err != nil {
		return err
	}
	return repo.RecordSent(ctx, intent, now)
}
