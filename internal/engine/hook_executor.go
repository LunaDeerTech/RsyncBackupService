package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

func executeHookCommands(ctx context.Context, commands []model.HookCommand, db *store.DB, auditLogger *audit.Logger, phase string, instanceID, taskID int64) error {
	for i, cmd := range commands {
		command := strings.TrimSpace(cmd.Command)
		if command == "" {
			continue
		}

		slog.Info("executing hook command",
			"phase", phase,
			"index", i+1,
			"location", cmd.Location,
			"command", command,
			"task_id", taskID,
		)

		if err := executeOneHookCommand(ctx, cmd, db); err != nil {
			writeHookAudit(ctx, auditLogger, instanceID, taskID, phase, i+1, cmd, err)
			return fmt.Errorf("%s command #%d failed: %w", phase, i+1, err)
		}
		writeHookAudit(ctx, auditLogger, instanceID, taskID, phase, i+1, cmd, nil)
	}
	return nil
}

func executeOneHookCommand(ctx context.Context, cmd model.HookCommand, db *store.DB) error {
	location := strings.TrimSpace(cmd.Location)
	command := strings.TrimSpace(cmd.Command)

	if location == "" || location == "local" {
		return executeLocalCommand(ctx, command)
	}

	remoteID, err := strconv.ParseInt(location, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid hook command location %q: %w", location, err)
	}

	remote, err := db.GetRemoteConfigByID(remoteID)
	if err != nil {
		return fmt.Errorf("load remote config %d: %w", remoteID, err)
	}

	client, err := service.DialSSHClient(ctx, *remote)
	if err != nil {
		return fmt.Errorf("connect to remote %q: %w", remote.Name, err)
	}
	defer client.Close()

	stdout, stderr, err := runSSHCommand(ctx, client, command)
	if err != nil {
		slog.Error("remote hook command failed",
			"remote", remote.Name,
			"command", command,
			"stdout", stdout,
			"stderr", stderr,
			"error", err,
		)
		return fmt.Errorf("remote command on %q: %w", remote.Name, err)
	}
	if stdout = strings.TrimSpace(stdout); stdout != "" {
		slog.Info("remote hook command output", "remote", remote.Name, "stdout", stdout)
	}

	return nil
}

func executeLocalCommand(ctx context.Context, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
		slog.Info("local hook command output", "command", command, "output", trimmed)
	}
	if err != nil {
		return fmt.Errorf("local command failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func writeHookAudit(ctx context.Context, logger *audit.Logger, instanceID, taskID int64, phase string, index int, cmd model.HookCommand, err error) {
	if logger == nil {
		return
	}

	action := audit.ActionHookCommandSuccess
	detail := map[string]any{
		"task_id":  taskID,
		"phase":    phase,
		"index":    index,
		"location": cmd.Location,
		"command":  cmd.Command,
	}
	if err != nil {
		action = audit.ActionHookCommandFail
		detail["error"] = err.Error()
	}

	if logErr := logger.LogAction(ctx, instanceID, 0, action, detail); logErr != nil {
		slog.Error("write hook audit log failed", "task_id", taskID, "phase", phase, "index", index, "error", logErr)
	}
}
