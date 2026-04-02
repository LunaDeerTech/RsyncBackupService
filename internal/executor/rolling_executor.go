package executor

import (
	"context"
	"time"
)

func WithExecutionTimeout(ctx context.Context, maxSeconds int) (context.Context, context.CancelFunc) {
	if maxSeconds <= 0 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, time.Duration(maxSeconds)*time.Second)
}

func ExecuteRolling(ctx context.Context, runner Runner, request RollingExecutionRequest, onProgress func(ProgressSnapshot)) error {
	if runner == nil {
		runner = NewExecRunner()
	}

	commandSpecs, err := BuildRollingCommandSpecs(request)
	if err != nil {
		return err
	}

	for phaseIndex, commandSpec := range commandSpecs {
		window := newProgressWindow(5)
		if err := runner.Run(ctx, commandSpec, func(line string) {
			snapshot, ok := ParseProgress2(line)
			if !ok {
				return
			}

			phaseTransferred := snapshot.BytesTransferred
			phasePercentage := snapshot.Percentage
			averageBytesPerSecond := window.Add(time.Now().UTC(), snapshot.BytesTransferred)
			if averageBytesPerSecond <= 0 {
				averageBytesPerSecond = snapshot.BytesPerSecond
			}
			snapshot.AverageBytesPerSecond = averageBytesPerSecond

			totalSize := estimateTotalSize(snapshot)
			snapshot.EstimatedRemaining = EstimateRemaining(totalSize, snapshot.BytesTransferred, averageBytesPerSecond)
			snapshot.PhaseBytesTransferred = phaseTransferred
			snapshot.PhasePercentage = phasePercentage
			snapshot.EstimatedTotalSize = totalSize
			snapshot.BytesTransferred, snapshot.Percentage = aggregatePhaseProgress(phaseIndex, len(commandSpecs), totalSize, phaseTransferred, phasePercentage)
			snapshot.EstimatedRemaining = aggregatePhaseRemaining(phaseIndex, len(commandSpecs), phasePercentage, snapshot.Elapsed, snapshot.EstimatedRemaining)

			if onProgress != nil {
				onProgress(snapshot)
			}
		}); err != nil {
			return err
		}
	}

	return nil
}

func estimateTotalSize(snapshot ProgressSnapshot) uint64 {
	if snapshot.Percentage <= 0 {
		return snapshot.BytesTransferred
	}

	return uint64(float64(snapshot.BytesTransferred) / (float64(snapshot.Percentage) / 100.0))
}

func aggregatePhasePercentage(phaseIndex, phaseCount, phasePercentage int) int {
	if phaseCount <= 1 {
		return phasePercentage
	}

	progressFraction := (float64(phaseIndex) + float64(phasePercentage)/100.0) / float64(phaseCount)
	if progressFraction >= 1 {
		return 100
	}
	if progressFraction <= 0 {
		return 0
	}

	return int(progressFraction * 100)
}

func aggregatePhaseProgress(phaseIndex, phaseCount int, phaseTotalSize uint64, phaseTransferred uint64, phasePercentage int) (uint64, int) {
	if phaseCount <= 1 || phaseTotalSize == 0 {
		return phaseTransferred, phasePercentage
	}

	overallTotalSize := phaseTotalSize * uint64(phaseCount)
	overallTransferred := uint64(phaseIndex)*phaseTotalSize + phaseTransferred
	if overallTransferred > overallTotalSize {
		overallTransferred = overallTotalSize
	}
	percentage := int(float64(overallTransferred) * 100 / float64(overallTotalSize))
	if percentage > 100 {
		percentage = 100
	}

	return overallTransferred, percentage
}

func aggregatePhaseRemaining(phaseIndex, phaseCount, phasePercentage int, elapsed, phaseRemaining time.Duration) time.Duration {
	if phaseCount <= 1 || phasePercentage <= 0 {
		return phaseRemaining
	}

	estimatedPhaseTotal := time.Duration(float64(elapsed) / (float64(phasePercentage) / 100.0))
	if estimatedPhaseTotal < elapsed {
		estimatedPhaseTotal = elapsed
	}
	remainingPhases := phaseCount - phaseIndex - 1
	if remainingPhases <= 0 {
		return phaseRemaining
	}

	return phaseRemaining + time.Duration(remainingPhases)*estimatedPhaseTotal
}