package executor

import (
	"fmt"
	"strings"
)

type RestoreSyncRequest struct {
	SourcePath             string
	SourceHost             string
	SourcePort             int
	SourceUser             string
	SourceSSHKeyPath       string
	SourceRemote           bool
	DestinationPath        string
	DestinationHost        string
	DestinationPort        int
	DestinationUser        string
	DestinationSSHKeyPath  string
	DestinationRemote      bool
	RelayDir               string
	Overwrite              bool
}

func BuildRestoreCommandSpecs(request RestoreSyncRequest) ([]CommandSpec, error) {
	trimmedSourcePath := strings.TrimSpace(request.SourcePath)
	trimmedDestinationPath := strings.TrimSpace(request.DestinationPath)
	if trimmedSourcePath == "" {
		return nil, fmt.Errorf("restore source path is required")
	}
	if trimmedDestinationPath == "" {
		return nil, fmt.Errorf("restore destination path is required")
	}

	commonArgs := func(delete bool) []string {
		args := []string{"-a", "--human-readable", "--info=progress2", "--protect-args"}
		if delete {
			args = append(args, "--delete")
		}
		return args
	}

	switch {
	case !request.SourceRemote && !request.DestinationRemote:
		args := append(commonArgs(request.Overwrite), withTrailingSlash(trimmedSourcePath), withTrailingSlash(trimmedDestinationPath))
		return []CommandSpec{{Name: "rsync", Args: args}}, nil
	case !request.SourceRemote && request.DestinationRemote:
		args := commonArgs(request.Overwrite)
		args = append(args, buildRemoteShellArgs(request.DestinationSSHKeyPath, request.DestinationPort)...)
		args = append(args, withTrailingSlash(trimmedSourcePath), withTrailingSlash(remoteLocation(request.DestinationUser, request.DestinationHost, trimmedDestinationPath)))
		return []CommandSpec{{Name: "rsync", Args: args}}, nil
	case request.SourceRemote && !request.DestinationRemote:
		args := commonArgs(request.Overwrite)
		args = append(args, buildRemoteShellArgs(request.SourceSSHKeyPath, request.SourcePort)...)
		args = append(args, withTrailingSlash(remoteLocation(request.SourceUser, request.SourceHost, trimmedSourcePath)), withTrailingSlash(trimmedDestinationPath))
		return []CommandSpec{{Name: "rsync", Args: args}}, nil
	default:
		trimmedRelayDir := strings.TrimSpace(request.RelayDir)
		if trimmedRelayDir == "" {
			return nil, fmt.Errorf("relay directory is required for remote-to-remote restore")
		}

		pullArgs := commonArgs(true)
		pullArgs = append(pullArgs, buildRemoteShellArgs(request.SourceSSHKeyPath, request.SourcePort)...)
		pullArgs = append(pullArgs, withTrailingSlash(remoteLocation(request.SourceUser, request.SourceHost, trimmedSourcePath)), withTrailingSlash(trimmedRelayDir))

		pushArgs := commonArgs(request.Overwrite)
		pushArgs = append(pushArgs, buildRemoteShellArgs(request.DestinationSSHKeyPath, request.DestinationPort)...)
		pushArgs = append(pushArgs, withTrailingSlash(trimmedRelayDir), withTrailingSlash(remoteLocation(request.DestinationUser, request.DestinationHost, trimmedDestinationPath)))

		return []CommandSpec{{Name: "rsync", Args: pullArgs}, {Name: "rsync", Args: pushArgs}}, nil
	}
}