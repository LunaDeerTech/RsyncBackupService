package engine

import (
	"testing"
	"time"
)

func TestCronExprNext(t *testing.T) {
	testCases := []struct {
		name string
		expr string
		from time.Time
		want time.Time
	}{
		{
			name: "daily at two am",
			expr: "0 2 * * *",
			from: time.Date(2026, 4, 7, 1, 10, 23, 0, time.UTC),
			want: time.Date(2026, 4, 7, 2, 0, 0, 0, time.UTC),
		},
		{
			name: "every thirty minutes",
			expr: "*/30 * * * *",
			from: time.Date(2026, 4, 7, 10, 7, 45, 0, time.UTC),
			want: time.Date(2026, 4, 7, 10, 30, 0, 0, time.UTC),
		},
		{
			name: "weekday morning window",
			expr: "15 9 * * 1-5",
			from: time.Date(2026, 4, 10, 8, 59, 0, 0, time.UTC),
			want: time.Date(2026, 4, 10, 9, 15, 0, 0, time.UTC),
		},
		{
			name: "month boundary",
			expr: "0 0 1 * *",
			from: time.Date(2026, 4, 30, 23, 59, 0, 0, time.UTC),
			want: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "weekday seven means sunday",
			expr: "0 0 * * 7",
			from: time.Date(2026, 4, 11, 23, 10, 0, 0, time.UTC),
			want: time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "day of month or weekday semantics",
			expr: "0 0 13 * 5",
			from: time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
			want: time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			expr, err := ParseCron(testCase.expr)
			if err != nil {
				t.Fatalf("ParseCron() error = %v", err)
			}

			got := expr.Next(testCase.from)
			if !got.Equal(testCase.want) {
				t.Fatalf("Next(%s) = %s, want %s", testCase.from.Format(time.RFC3339), got.Format(time.RFC3339), testCase.want.Format(time.RFC3339))
			}
		})
	}
}

func TestParseCronRejectsInvalidExpressions(t *testing.T) {
	testCases := []string{
		"",
		"0 0 * *",
		"61 * * * *",
		"* 24 * * *",
		"0 0 0 * *",
		"0 0 * 13 *",
		"0 0 * * 8",
		"*/0 * * * *",
		"1-5-7 * * * *",
	}

	for _, expr := range testCases {
		t.Run(expr, func(t *testing.T) {
			if _, err := ParseCron(expr); err == nil {
				t.Fatalf("ParseCron(%q) error = nil, want non-nil", expr)
			}
		})
	}
}
