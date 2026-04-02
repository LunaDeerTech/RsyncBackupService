package executor

import (
	"math"
	"testing"
	"time"
)

func TestParseProgress2Line(t *testing.T) {
	progress, ok := ParseProgress2("1,234,567  45%  12.34MB/s  0:01:23")
	if !ok {
		t.Fatal("expected progress2 line to parse")
	}
	if progress.BytesTransferred != 1234567 {
		t.Fatalf("expected transferred bytes 1234567, got %d", progress.BytesTransferred)
	}
	if progress.Percentage != 45 {
		t.Fatalf("expected percentage 45, got %d", progress.Percentage)
	}

	wantSpeed := 12.34 * 1024 * 1024
	if math.Abs(progress.BytesPerSecond-wantSpeed) > 128 {
		t.Fatalf("expected speed about %.2f, got %.2f", wantSpeed, progress.BytesPerSecond)
	}
	if progress.Elapsed != 83*time.Second {
		t.Fatalf("expected elapsed duration 83s, got %s", progress.Elapsed)
	}
}

func TestEstimateRemaining(t *testing.T) {
	remaining := EstimateRemaining(1000, 400, 200)
	if remaining != 3*time.Second {
		t.Fatalf("expected remaining duration 3s, got %s", remaining)
	}

	if remaining := EstimateRemaining(1000, 1000, 200); remaining != 0 {
		t.Fatalf("expected zero duration when transfer is complete, got %s", remaining)
	}
}

func TestProgressWindowUsesRecentSamples(t *testing.T) {
	window := newProgressWindow(3)

	if avg := window.Add(time.Unix(0, 0), 100); avg != 0 {
		t.Fatalf("expected zero average for first sample, got %.2f", avg)
	}
	if avg := window.Add(time.Unix(1, 0), 300); avg != 200 {
		t.Fatalf("expected average 200B/s after second sample, got %.2f", avg)
	}
	if avg := window.Add(time.Unix(2, 0), 700); avg != 300 {
		t.Fatalf("expected average 300B/s after third sample, got %.2f", avg)
	}
	if avg := window.Add(time.Unix(3, 0), 1300); avg != 500 {
		t.Fatalf("expected sliding-window average 500B/s, got %.2f", avg)
	}
}