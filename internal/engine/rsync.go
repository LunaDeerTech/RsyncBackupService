package engine

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"rsync-backup-service/internal/model"
)

var (
	ErrRsyncPartialTransfer = errors.New("rsync partial transfer")
	ErrRsyncSourceVanished  = errors.New("rsync source files vanished")
	ErrRsyncRemoteToRemote  = errors.New("rsync does not support direct ssh-to-ssh transfer")

	progressPattern = regexp.MustCompile(`^([0-9][0-9,]*(?:\.[0-9]+)?|[0-9]+(?:\.[0-9]+)?[A-Za-z]+)\s+(\d+)%\s+(\S+)\s+(\S+)`)
	statPatterns    = map[string]*regexp.Regexp{
		"total_files":    regexp.MustCompile(`^Number of files:\s+([0-9,]+)$`),
		"transfer_files": regexp.MustCompile(`^Number of regular files transferred:\s+([0-9,]+)$`),
		"total_size":     regexp.MustCompile(`^Total file size:\s+([0-9,]+) bytes$`),
		"transfer_size":  regexp.MustCompile(`^Total transferred file size:\s+([0-9,]+) bytes$`),
		"speed":          regexp.MustCompile(`([0-9][0-9,]*(?:\.[0-9]+)?)\s+bytes/sec$`),
	}
)

type RsyncConfig struct {
	SourcePath   string
	SourceType   string
	SourceRemote *model.RemoteConfig
	DestPath     string
	DestType     string
	DestRemote   *model.RemoteConfig
	LinkDestPath string
	ExtraArgs    []string
}

type RsyncResult struct {
	ExitCode int
	Stats    RsyncStats
	Stdout   string
	Stderr   string
}

type RsyncStats struct {
	TotalSize     int64
	TransferSize  int64
	TotalFiles    int
	TransferFiles int
	Speed         string
	Duration      time.Duration
}

type ProgressInfo struct {
	BytesTransferred int64
	Percentage       int
	Speed            string
	Remaining        string
}

type commandFactory func(context.Context, string, ...string) *exec.Cmd

type RsyncExecutor struct {
	commandName string
	newCommand  commandFactory
}

func NewRsyncExecutor() *RsyncExecutor {
	return &RsyncExecutor{
		commandName: "rsync",
		newCommand:  exec.CommandContext,
	}
}

func BuildRsyncArgs(cfg RsyncConfig) []string {
	if err := validateRsyncConfig(cfg); err != nil {
		return nil
	}

	args := []string{"-avz", "--delete", "--stats", "--info=progress2"}

	if sshArg, ok := buildSSHArg(cfg); ok {
		args = append(args, sshArg)
	}
	if linkDest := strings.TrimSpace(cfg.LinkDestPath); linkDest != "" {
		args = append(args, "--link-dest="+linkDest)
	}
	if len(cfg.ExtraArgs) > 0 {
		args = append(args, cfg.ExtraArgs...)
	}

	source := buildEndpoint(cfg.SourceType, strings.TrimSpace(cfg.SourcePath), cfg.SourceRemote)
	dest := buildEndpoint(cfg.DestType, strings.TrimSpace(cfg.DestPath), cfg.DestRemote)
	args = append(args, source, dest)

	return args
}

func (e *RsyncExecutor) Execute(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
	if err := validateRsyncConfig(cfg); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if e == nil {
		e = NewRsyncExecutor()
	}
	if e.commandName == "" {
		e.commandName = "rsync"
	}
	if e.newCommand == nil {
		e.newCommand = exec.CommandContext
	}

	args := BuildRsyncArgs(cfg)
	cmd := e.newCommand(ctx, e.commandName, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create rsync stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create rsync stderr pipe: %w", err)
	}

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start rsync: %w", err)
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	var stdoutErr error
	var stderrErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		stdoutErr = streamRsyncStdout(stdoutPipe, &stdoutBuf, progressCb)
	}()
	go func() {
		defer wg.Done()
		_, stderrErr = io.Copy(&stderrBuf, stderrPipe)
	}()

	processDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		case <-processDone:
		}
	}()

	waitErr := cmd.Wait()
	close(processDone)
	wg.Wait()

	result := &RsyncResult{
		ExitCode: exitCodeFromProcessState(cmd),
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
	}
	result.Stats = ParseStats(strings.Join([]string{result.Stdout, result.Stderr}, "\n"))
	result.Stats.Duration = time.Since(startTime)

	if stdoutErr != nil {
		return result, fmt.Errorf("read rsync stdout: %w", stdoutErr)
	}
	if stderrErr != nil {
		return result, fmt.Errorf("read rsync stderr: %w", stderrErr)
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if waitErr != nil {
		return result, mapRsyncExitError(result.ExitCode, waitErr, result.Stderr)
	}

	return result, nil
}

func ParseProgress(line string) (*ProgressInfo, bool) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
	if trimmed == "" {
		return nil, false
	}

	matches := progressPattern.FindStringSubmatch(trimmed)
	if len(matches) != 5 {
		return nil, false
	}

	bytesTransferred, err := parseProgressBytes(matches[1])
	if err != nil {
		return nil, false
	}
	percentage, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, false
	}

	return &ProgressInfo{
		BytesTransferred: bytesTransferred,
		Percentage:       percentage,
		Speed:            matches[3],
		Remaining:        matches[4],
	}, true
}

func ParseStats(output string) RsyncStats {
	stats := RsyncStats{}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if matches := statPatterns["total_files"].FindStringSubmatch(trimmed); len(matches) == 2 {
			stats.TotalFiles = parseStatInt(matches[1])
			continue
		}
		if matches := statPatterns["transfer_files"].FindStringSubmatch(trimmed); len(matches) == 2 {
			stats.TransferFiles = parseStatInt(matches[1])
			continue
		}
		if matches := statPatterns["total_size"].FindStringSubmatch(trimmed); len(matches) == 2 {
			stats.TotalSize = int64(parseStatInt(matches[1]))
			continue
		}
		if matches := statPatterns["transfer_size"].FindStringSubmatch(trimmed); len(matches) == 2 {
			stats.TransferSize = int64(parseStatInt(matches[1]))
			continue
		}
		if matches := statPatterns["speed"].FindStringSubmatch(trimmed); len(matches) == 2 {
			stats.Speed = matches[1] + " bytes/sec"
		}
	}

	return stats
}

func validateRsyncConfig(cfg RsyncConfig) error {
	sourceType := normalizeRsyncType(cfg.SourceType)
	destType := normalizeRsyncType(cfg.DestType)

	if sourceType == "" || destType == "" {
		return fmt.Errorf("rsync source and destination types are required")
	}
	if strings.TrimSpace(cfg.SourcePath) == "" || strings.TrimSpace(cfg.DestPath) == "" {
		return fmt.Errorf("rsync source and destination paths are required")
	}
	if sourceType != "local" && sourceType != "ssh" {
		return fmt.Errorf("unsupported rsync source type %q", cfg.SourceType)
	}
	if destType != "local" && destType != "ssh" {
		return fmt.Errorf("unsupported rsync destination type %q", cfg.DestType)
	}
	if sourceType == "ssh" && cfg.SourceRemote == nil {
		return fmt.Errorf("ssh source requires remote config")
	}
	if destType == "ssh" && cfg.DestRemote == nil {
		return fmt.Errorf("ssh destination requires remote config")
	}
	if sourceType == "ssh" && destType == "ssh" {
		return ErrRsyncRemoteToRemote
	}

	return nil
}

func buildSSHArg(cfg RsyncConfig) (string, bool) {
	var remote *model.RemoteConfig
	if normalizeRsyncType(cfg.SourceType) == "ssh" {
		remote = cfg.SourceRemote
	} else if normalizeRsyncType(cfg.DestType) == "ssh" {
		remote = cfg.DestRemote
	}
	if remote == nil {
		return "", false
	}

	port := remote.Port
	if port <= 0 {
		port = 22
	}

	parts := []string{
		"ssh",
		"-i", strings.TrimSpace(remote.PrivateKeyPath),
		"-p", strconv.Itoa(port),
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "BatchMode=yes",
	}

	return "--rsh=" + strings.Join(parts, " "), true
}

func buildEndpoint(endpointType, path string, remote *model.RemoteConfig) string {
	formattedPath := ensureTrailingSlash(path)
	if normalizeRsyncType(endpointType) != "ssh" {
		return formattedPath
	}

	return fmt.Sprintf("%s@%s:%s", strings.TrimSpace(remote.Username), strings.TrimSpace(remote.Host), formattedPath)
}

func ensureTrailingSlash(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || strings.HasSuffix(trimmed, "/") {
		return trimmed
	}
	return trimmed + "/"
}

func normalizeRsyncType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func streamRsyncStdout(reader io.Reader, output *bytes.Buffer, progressCb func(ProgressInfo)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(scanRsyncLines)

	for scanner.Scan() {
		line := scanner.Text()
		if output.Len() > 0 {
			output.WriteByte('\n')
		}
		output.WriteString(line)

		if progress, ok := ParseProgress(line); ok && progressCb != nil {
			progressCb(*progress)
		}
	}

	return scanner.Err()
}

func scanRsyncLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	for index, value := range data {
		if value == '\n' || value == '\r' {
			return index + 1, bytes.TrimRight(data[:index], "\r"), nil
		}
	}

	if atEOF {
		return len(data), bytes.TrimRight(data, "\r"), nil
	}

	return 0, nil, nil
}

func parseProgressBytes(raw string) (int64, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), ",", ""))
	if normalized == "" {
		return 0, fmt.Errorf("empty size")
	}

	numberEnd := 0
	for numberEnd < len(normalized) {
		ch := normalized[numberEnd]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			numberEnd++
			continue
		}
		break
	}
	if numberEnd == 0 {
		return 0, fmt.Errorf("invalid size %q", raw)
	}

	value, err := strconv.ParseFloat(normalized[:numberEnd], 64)
	if err != nil {
		return 0, err
	}

	suffix := strings.TrimSpace(normalized[numberEnd:])
	multiplier, err := progressUnitMultiplier(suffix)
	if err != nil {
		return 0, err
	}

	return int64(math.Round(value * multiplier)), nil
}

func progressUnitMultiplier(suffix string) (float64, error) {
	normalized := strings.TrimSpace(strings.ToUpper(suffix))
	normalized = strings.TrimSuffix(normalized, "IB")
	normalized = strings.TrimSuffix(normalized, "B")

	switch normalized {
	case "":
		return 1, nil
	case "K":
		return 1024, nil
	case "M":
		return 1024 * 1024, nil
	case "G":
		return 1024 * 1024 * 1024, nil
	case "T":
		return 1024 * 1024 * 1024 * 1024, nil
	case "P":
		return 1024 * 1024 * 1024 * 1024 * 1024, nil
	case "E":
		return 1024 * 1024 * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unsupported size suffix %q", suffix)
	}
}

func parseStatInt(raw string) int {
	value, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(raw), ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return int(value)
}

func exitCodeFromProcessState(cmd *exec.Cmd) int {
	if cmd == nil || cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.ExitCode()
}

func mapRsyncExitError(exitCode int, waitErr error, stderr string) error {
	if waitErr == nil || exitCode == 0 {
		return nil
	}

	message := strings.TrimSpace(stderr)
	if message == "" {
		message = waitErr.Error()
	}

	switch exitCode {
	case 23:
		return fmt.Errorf("%w: %s", ErrRsyncPartialTransfer, message)
	case 24:
		return fmt.Errorf("%w: %s", ErrRsyncSourceVanished, message)
	default:
		return fmt.Errorf("rsync failed with exit code %d: %s", exitCode, message)
	}
}
