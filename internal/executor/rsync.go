package executor

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
)

type RollingExecutionRequest struct {
	Instance         model.BackupInstance
	Target           model.StorageTarget
	SourceSSHKeyPath string
	TargetSSHKeyPath string
	SnapshotPath     string
	TargetLinkDest   string
	RelayCacheDir    string
	RelayLinkDest    string
	ExcludePatterns  []string
}

func BuildRollingCommandSpecs(request RollingExecutionRequest) ([]CommandSpec, error) {
	trimmedSnapshotPath := strings.TrimSpace(request.SnapshotPath)
	if trimmedSnapshotPath == "" {
		return nil, fmt.Errorf("snapshot path is required")
	}

	commonArgs := func(linkDest string) []string {
		args := []string{"-a", "--delete", "--human-readable", "--info=progress2", "--protect-args"}
		if trimmedLinkDest := strings.TrimSpace(linkDest); trimmedLinkDest != "" {
			args = append(args, "--link-dest="+trimmedLinkDest)
		}
		for _, excludePattern := range request.ExcludePatterns {
			trimmedPattern := strings.TrimSpace(excludePattern)
			if trimmedPattern != "" {
				args = append(args, "--exclude", trimmedPattern)
			}
		}
		return args
	}

	sourceIsRemote := isRemoteSource(request.Instance)
	targetIsRemote := isRemoteTarget(request.Target)

	switch {
	case !sourceIsRemote && !targetIsRemote:
		args := append(commonArgs(request.TargetLinkDest), withTrailingSlash(request.Instance.SourcePath), withTrailingSlash(trimmedSnapshotPath))
		return []CommandSpec{{Name: "rsync", Args: args}}, nil
	case !sourceIsRemote && targetIsRemote:
		args := commonArgs(request.TargetLinkDest)
		args = append(args, buildRemoteShellArgs(request.TargetSSHKeyPath, request.Target.Port)...)
		args = append(args, withTrailingSlash(request.Instance.SourcePath), withTrailingSlash(remoteLocation(request.Target.User, request.Target.Host, trimmedSnapshotPath)))
		return []CommandSpec{{Name: "rsync", Args: args}}, nil
	case sourceIsRemote && !targetIsRemote:
		args := commonArgs(request.TargetLinkDest)
		args = append(args, buildRemoteShellArgs(request.SourceSSHKeyPath, request.Instance.SourcePort)...)
		args = append(args, withTrailingSlash(remoteLocation(request.Instance.SourceUser, request.Instance.SourceHost, request.Instance.SourcePath)), withTrailingSlash(trimmedSnapshotPath))
		return []CommandSpec{{Name: "rsync", Args: args}}, nil
	default:
		trimmedRelayCacheDir := strings.TrimSpace(request.RelayCacheDir)
		if trimmedRelayCacheDir == "" {
			return nil, fmt.Errorf("relay cache directory is required for remote-to-remote rolling backups")
		}

		pullArgs := commonArgs(request.RelayLinkDest)
		pullArgs = append(pullArgs, buildRemoteShellArgs(request.SourceSSHKeyPath, request.Instance.SourcePort)...)
		pullArgs = append(pullArgs, withTrailingSlash(remoteLocation(request.Instance.SourceUser, request.Instance.SourceHost, request.Instance.SourcePath)), withTrailingSlash(trimmedRelayCacheDir))

		pushArgs := commonArgs(request.TargetLinkDest)
		pushArgs = append(pushArgs, buildRemoteShellArgs(request.TargetSSHKeyPath, request.Target.Port)...)
		pushArgs = append(pushArgs, withTrailingSlash(trimmedRelayCacheDir), withTrailingSlash(remoteLocation(request.Target.User, request.Target.Host, trimmedSnapshotPath)))

		return []CommandSpec{
			{Name: "rsync", Args: pullArgs},
			{Name: "rsync", Args: pushArgs},
		}, nil
	}
}

func MapRsyncExitCode(code int) error {
	switch code {
	case 0:
		return nil
	case 1:
		return errors.New("rsync syntax or usage error")
	case 2:
		return errors.New("rsync protocol incompatibility")
	case 3:
		return errors.New("rsync encountered errors selecting input or output files")
	case 10:
		return errors.New("rsync failed to open a socket connection")
	case 11:
		return errors.New("rsync file I/O error")
	case 12:
		return errors.New("rsync protocol data stream error or unexpected remote exit")
	case 23:
		return errors.New("rsync partial transfer due to error")
	case 24:
		return errors.New("rsync partial transfer because source files vanished")
	case 30:
		return errors.New("rsync timeout in data send or receive")
	case 35:
		return errors.New("rsync daemon connection timed out")
	default:
		return fmt.Errorf("rsync exited with code %d", code)
	}
}

func buildRemoteShellArgs(privateKeyPath string, port int) []string {
	sshCommand := buildSSHCommand(privateKeyPath, port)
	if sshCommand == "" {
		return nil
	}

	return []string{"-e", sshCommand}
}

func buildSSHCommand(privateKeyPath string, port int) string {
	parts := []string{
		"ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}

	trimmedPrivateKeyPath := strings.TrimSpace(privateKeyPath)
	if trimmedPrivateKeyPath != "" {
		parts = append(parts, "-i", shellQuote(trimmedPrivateKeyPath))
	}
	if port > 0 {
		parts = append(parts, "-p", strconv.Itoa(port))
	}

	return strings.Join(parts, " ")
}

func remoteLocation(user, host, remotePath string) string {
	trimmedUser := strings.TrimSpace(user)
	trimmedHost := strings.TrimSpace(host)
	trimmedRemotePath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(remotePath)))

	return fmt.Sprintf("%s@%s:%s", trimmedUser, trimmedHost, trimmedRemotePath)
}

func withTrailingSlash(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}
	if strings.HasSuffix(trimmedPath, "/") {
		return trimmedPath
	}

	return trimmedPath + "/"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}