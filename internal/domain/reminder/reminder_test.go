package reminder

import (
	"testing"
	"time"
)

func TestScheduleMatches(t *testing.T) {
	t.Parallel()

	schedule := Schedule{Slots: []ScheduleSlot{{Weekdays: []time.Weekday{time.Wednesday}, Time: LocalTime{Hour: 8, Minute: 0}}}}
	when := time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC)
	if !schedule.Matches(when) {
		t.Fatal("expected match")
	}
	if schedule.Matches(when.Add(time.Hour)) {
		t.Fatal("unexpected match")
	}
}
