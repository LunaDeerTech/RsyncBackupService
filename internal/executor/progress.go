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

var progressTokenPattern = regexp.MustCompile(`^([0-9][0-9,]*(?:\.[0-9]+)?)([A-Za-z/]*)$`)

func ParseProgress2(line string) (ProgressSnapshot, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 4 {
		return ProgressSnapshot{}, false
	}

	transferred, err := parseProgressTransferred(fields[0])
	if err != nil {
		return ProgressSnapshot{}, false
	}
	percentage, err := strconv.Atoi(strings.TrimSuffix(fields[1], "%"))
	if err != nil {
		return ProgressSnapshot{}, false
	}
	bytesPerSecond, err := parseProgressSpeed(fields[2])
	if err != nil {
		return ProgressSnapshot{}, false
	}
	elapsed, err := parseProgressElapsed(fields[3])
	if err != nil {
		return ProgressSnapshot{}, false
	}

	return ProgressSnapshot{
		BytesTransferred: transferred,
		Percentage:       percentage,
		BytesPerSecond:   bytesPerSecond,
		Elapsed:          elapsed,
	}, true
}

func parseProgressTransferred(value string) (uint64, error) {
	amount, unit, err := parseProgressAmount(value)
	if err != nil {
		return 0, err
	}

	multiplier, ok := progressSizeMultiplier(unit)
	if !ok {
		return 0, fmt.Errorf("invalid progress size unit %q", unit)
	}

	return uint64(amount*multiplier + 0.5), nil
}

func parseProgressSpeed(value string) (float64, error) {
	amount, unit, err := parseProgressAmount(value)
	if err != nil {
		return 0, err
	}

	multiplier, ok := progressUnitMultiplier(unit)
	if !ok {
		return 0, fmt.Errorf("invalid progress speed unit %q", unit)
	}

	return amount * multiplier, nil
}

func parseProgressAmount(value string) (float64, string, error) {
	matches := progressTokenPattern.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil {
		return 0, "", fmt.Errorf("invalid progress token %q", value)
	}

	amount, err := strconv.ParseFloat(strings.ReplaceAll(matches[1], ",", ""), 64)
	if err != nil {
		return 0, "", err
	}

	return amount, matches[2], nil
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

func progressSizeMultiplier(unit string) (float64, bool) {
	switch strings.ToUpper(strings.TrimSpace(unit)) {
	case "", "B":
		return 1, true
	case "K", "KB":
		return 1024, true
	case "M", "MB":
		return 1024 * 1024, true
	case "G", "GB":
		return 1024 * 1024 * 1024, true
	case "T", "TB":
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