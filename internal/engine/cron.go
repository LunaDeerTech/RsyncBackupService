package engine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CronExpr struct {
	Minutes  []int
	Hours    []int
	Days     []int
	Months   []int
	Weekdays []int

	minuteSet   map[int]struct{}
	hourSet     map[int]struct{}
	daySet      map[int]struct{}
	monthSet    map[int]struct{}
	weekdaySet  map[int]struct{}
	allDays     bool
	allWeekdays bool
}

func ParseCron(expr string) (*CronExpr, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("schedule_value must be a standard 5-field cron expression")
	}

	minutes, err := parseCronField(fields[0], 0, 59, false)
	if err != nil {
		return nil, fmt.Errorf("parse minute field: %w", err)
	}
	hours, err := parseCronField(fields[1], 0, 23, false)
	if err != nil {
		return nil, fmt.Errorf("parse hour field: %w", err)
	}
	days, err := parseCronField(fields[2], 1, 31, false)
	if err != nil {
		return nil, fmt.Errorf("parse day field: %w", err)
	}
	months, err := parseCronField(fields[3], 1, 12, false)
	if err != nil {
		return nil, fmt.Errorf("parse month field: %w", err)
	}
	weekdays, err := parseCronField(fields[4], 0, 7, true)
	if err != nil {
		return nil, fmt.Errorf("parse weekday field: %w", err)
	}

	cron := &CronExpr{
		Minutes:     minutes,
		Hours:       hours,
		Days:        days,
		Months:      months,
		Weekdays:    weekdays,
		minuteSet:   sliceToSet(minutes),
		hourSet:     sliceToSet(hours),
		daySet:      sliceToSet(days),
		monthSet:    sliceToSet(months),
		weekdaySet:  sliceToSet(weekdays),
		allDays:     len(days) == 31,
		allWeekdays: len(weekdays) == 7,
	}

	return cron, nil
}

func (c *CronExpr) Next(from time.Time) time.Time {
	if c == nil {
		return time.Time{}
	}

	candidate := from.Truncate(time.Minute).Add(time.Minute)
	deadline := candidate.AddDate(5, 0, 0)
	for !candidate.After(deadline) {
		if c.matches(candidate) {
			return candidate
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}
}

func (c *CronExpr) matches(ts time.Time) bool {
	if c == nil {
		return false
	}
	if _, ok := c.minuteSet[ts.Minute()]; !ok {
		return false
	}
	if _, ok := c.hourSet[ts.Hour()]; !ok {
		return false
	}
	if _, ok := c.monthSet[int(ts.Month())]; !ok {
		return false
	}

	_, dayMatch := c.daySet[ts.Day()]
	_, weekdayMatch := c.weekdaySet[int(ts.Weekday())]
	switch {
	case c.allDays && c.allWeekdays:
		return true
	case c.allDays:
		return weekdayMatch
	case c.allWeekdays:
		return dayMatch
	default:
		return dayMatch || weekdayMatch
	}
}

func parseCronField(field string, min, max int, normalizeWeekday bool) ([]int, error) {
	trimmed := strings.TrimSpace(field)
	if trimmed == "" {
		return nil, fmt.Errorf("empty field")
	}

	values := make(map[int]struct{})
	for _, part := range strings.Split(trimmed, ",") {
		segment := strings.TrimSpace(part)
		if segment == "" {
			return nil, fmt.Errorf("empty field segment")
		}

		expanded, err := expandCronSegment(segment, min, max, normalizeWeekday)
		if err != nil {
			return nil, err
		}
		for _, value := range expanded {
			values[value] = struct{}{}
		}
	}

	result := make([]int, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Ints(result)
	return result, nil
}

func expandCronSegment(segment string, min, max int, normalizeWeekday bool) ([]int, error) {
	base := strings.TrimSpace(segment)
	step := 1
	if strings.Contains(base, "/") {
		left, right, found := strings.Cut(base, "/")
		if !found || strings.TrimSpace(right) == "" {
			return nil, fmt.Errorf("invalid step segment %q", segment)
		}

		parsedStep, err := strconv.Atoi(strings.TrimSpace(right))
		if err != nil || parsedStep <= 0 {
			return nil, fmt.Errorf("invalid step value %q", right)
		}
		base = strings.TrimSpace(left)
		step = parsedStep
	}

	start := min
	end := max
	switch {
	case base == "" || base == "*":
	case strings.Contains(base, "-"):
		left, right, found := strings.Cut(base, "-")
		if !found {
			return nil, fmt.Errorf("invalid range segment %q", base)
		}

		parsedStart, err := strconv.Atoi(strings.TrimSpace(left))
		if err != nil {
			return nil, fmt.Errorf("invalid range start %q", left)
		}
		parsedEnd, err := strconv.Atoi(strings.TrimSpace(right))
		if err != nil {
			return nil, fmt.Errorf("invalid range end %q", right)
		}
		start = parsedStart
		end = parsedEnd
	default:
		parsedValue, err := strconv.Atoi(base)
		if err != nil {
			return nil, fmt.Errorf("invalid field value %q", base)
		}
		start = parsedValue
		end = parsedValue
	}

	if start < min || end > max || start > end {
		return nil, fmt.Errorf("range %q is out of bounds", base)
	}

	values := make([]int, 0, ((end-start)/step)+1)
	for value := start; value <= end; value += step {
		normalizedValue := value
		if normalizeWeekday && value == 7 {
			normalizedValue = 0
		}
		values = append(values, normalizedValue)
	}

	return values, nil
}

func sliceToSet(values []int) map[int]struct{} {
	set := make(map[int]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
