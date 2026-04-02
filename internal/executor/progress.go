package executor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ProgressSnapshot struct {
	BytesTransferred      uint64
	PhaseBytesTransferred uint64
	Percentage            int
	PhasePercentage       int
	BytesPerSecond        float64
	Elapsed               time.Duration
	EstimatedTotalSize    uint64
	AverageBytesPerSecond float64
	EstimatedRemaining    time.Duration
}

var progress2Pattern = regexp.MustCompile(`^\s*([0-9][0-9,]*)\s+([0-9]{1,3})%\s+([0-9]*\.?[0-9]+)([A-Za-z/]+)\s+([0-9:]+)(?:\s+.*)?$`)

func ParseProgress2(line string) (ProgressSnapshot, bool) {
	matches := progress2Pattern.FindStringSubmatch(strings.TrimSpace(line))
	if matches == nil {
		return ProgressSnapshot{}, false
	}

	transferred, err := strconv.ParseUint(strings.ReplaceAll(matches[1], ",", ""), 10, 64)
	if err != nil {
		return ProgressSnapshot{}, false
	}
	percentage, err := strconv.Atoi(matches[2])
	if err != nil {
		return ProgressSnapshot{}, false
	}
	speedValue, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return ProgressSnapshot{}, false
	}
	speedMultiplier, ok := progressUnitMultiplier(matches[4])
	if !ok {
		return ProgressSnapshot{}, false
	}
	elapsed, err := parseProgressElapsed(matches[5])
	if err != nil {
		return ProgressSnapshot{}, false
	}

	return ProgressSnapshot{
		BytesTransferred: transferred,
		Percentage:       percentage,
		BytesPerSecond:   speedValue * speedMultiplier,
		Elapsed:          elapsed,
	}, true
}

func EstimateRemaining(totalSize, transferred uint64, avgBytesPerSecond float64) time.Duration {
	if avgBytesPerSecond <= 0 || transferred >= totalSize {
		return 0
	}

	remainingSeconds := float64(totalSize-transferred) / avgBytesPerSecond
	if remainingSeconds <= 0 {
		return 0
	}

	return time.Duration(remainingSeconds * float64(time.Second))
}

type progressWindow struct {
	maxSamples int
	samples    []progressSample
}

type progressSample struct {
	at          time.Time
	transferred uint64
}

func newProgressWindow(maxSamples int) *progressWindow {
	if maxSamples <= 0 {
		maxSamples = 5
	}

	return &progressWindow{maxSamples: maxSamples}
}

func (w *progressWindow) Add(at time.Time, transferred uint64) float64 {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if len(w.samples) > 0 && transferred < w.samples[len(w.samples)-1].transferred {
		w.samples = w.samples[:0]
	}

	w.samples = append(w.samples, progressSample{at: at, transferred: transferred})
	if len(w.samples) > w.maxSamples {
		w.samples = append([]progressSample(nil), w.samples[len(w.samples)-w.maxSamples:]...)
	}
	if len(w.samples) < 2 {
		return 0
	}

	firstSample := w.samples[0]
	lastSample := w.samples[len(w.samples)-1]
	timeDelta := lastSample.at.Sub(firstSample.at).Seconds()
	if timeDelta <= 0 {
		return 0
	}
	if lastSample.transferred < firstSample.transferred {
		return 0
	}

	return float64(lastSample.transferred-firstSample.transferred) / timeDelta
}

func progressUnitMultiplier(unit string) (float64, bool) {
	switch strings.ToUpper(strings.TrimSpace(unit)) {
	case "B/S":
		return 1, true
	case "KB/S":
		return 1024, true
	case "MB/S":
		return 1024 * 1024, true
	case "GB/S":
		return 1024 * 1024 * 1024, true
	case "TB/S":
		return 1024 * 1024 * 1024 * 1024, true
	default:
		return 0, false
	}
}

func parseProgressElapsed(value string) (time.Duration, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid elapsed time %q", value)
	}

	parsed := make([]int, 0, len(parts))
	for _, part := range parts {
		currentValue, err := strconv.Atoi(part)
		if err != nil {
			return 0, fmt.Errorf("parse elapsed time %q: %w", value, err)
		}
		parsed = append(parsed, currentValue)
	}

	if len(parsed) == 2 {
		return time.Duration(parsed[0])*time.Minute + time.Duration(parsed[1])*time.Second, nil
	}

	return time.Duration(parsed[0])*time.Hour + time.Duration(parsed[1])*time.Minute + time.Duration(parsed[2])*time.Second, nil
}