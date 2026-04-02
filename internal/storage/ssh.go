package storage

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHConfig struct {
	Host           string
	Port           int
	User           string
	PrivateKeyPath string
	BasePath       string
	Timeout        time.Duration
}

type SSHStorage struct {
	config SSHConfig
}

func NewSSHStorage(config SSHConfig) *SSHStorage {
	return &SSHStorage{config: config}
}

func (s *SSHStorage) Type() string {
	return "ssh"
}

func (s *SSHStorage) Upload(ctx context.Context, localPath, remotePath string) error {
	resolvedPath, err := s.resolveRemotePath(remotePath)
	if err != nil {
		return err
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer localFile.Close()

	client, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	session.Stdin = localFile
	command := fmt.Sprintf("mkdir -p %s && cat > %s", shellQuote(path.Dir(resolvedPath)), shellQuote(resolvedPath))
	if err := session.Run(command); err != nil {
		return fmt.Errorf("upload file over ssh: %w", err)
	}

	return nil
}

func (s *SSHStorage) Download(ctx context.Context, remotePath, localPath string) error {
	resolvedPath, err := s.resolveRemotePath(remotePath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create local download directory: %w", err)
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local download file: %w", err)
	}
	defer localFile.Close()

	client, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	session.Stdout = localFile
	if err := session.Run(fmt.Sprintf("cat %s", shellQuote(resolvedPath))); err != nil {
		return fmt.Errorf("download file over ssh: %w", err)
	}

	return nil
}

func (s *SSHStorage) List(ctx context.Context, prefix string) ([]StorageObject, error) {
	resolvedPath, err := s.resolveRemotePath(prefix)
	if err != nil {
		return nil, err
	}

	command := fmt.Sprintf("find %s -mindepth 1 -maxdepth 1 -printf '%%P\t%%s\t%%T@\t%%y\\n'", shellQuote(resolvedPath))
	output, err := s.runCommand(ctx, command)
	if err != nil {
		lowerErr := strings.ToLower(err.Error())
		if strings.Contains(lowerErr, "no such file or directory") || strings.Contains(lowerErr, "cannot access") {
			return []StorageObject{}, nil
		}
		return nil, err
	}

	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		return []StorageObject{}, nil
	}

	lines := strings.Split(trimmedOutput, "\n")
	objects := make([]StorageObject, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			return nil, fmt.Errorf("parse ssh list output: unexpected line %q", line)
		}

		size, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse ssh storage object size: %w", err)
		}
		modifiedAtFloat, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return nil, fmt.Errorf("parse ssh storage object modified time: %w", err)
		}

		objects = append(objects, StorageObject{
			Path:       path.Join(strings.TrimSpace(prefix), parts[0]),
			Size:       size,
			IsDir:      parts[3] == "d",
			ModifiedAt: int64(modifiedAtFloat),
		})
	}

	return objects, nil
}

func (s *SSHStorage) Delete(ctx context.Context, remotePath string) error {
	resolvedPath, err := s.resolveRemotePath(remotePath)
	if err != nil {
		return err
	}

	_, err = s.runCommand(ctx, fmt.Sprintf("rm -rf %s", shellQuote(resolvedPath)))
	if err != nil {
		return fmt.Errorf("delete path over ssh: %w", err)
	}

	return nil
}

func (s *SSHStorage) SpaceAvailable(ctx context.Context, remotePath string) (uint64, error) {
	resolvedPath, err := s.resolveRemotePath(remotePath)
	if err != nil {
		return 0, err
	}

	output, err := s.runCommand(ctx, fmt.Sprintf("mkdir -p %s && df -Pk %s | tail -n 1 | awk '{print $4}'", shellQuote(resolvedPath), shellQuote(resolvedPath)))
	if err != nil {
		return 0, err
	}

	availableBlocks, err := strconv.ParseUint(strings.TrimSpace(output), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse ssh available space: %w", err)
	}

	return availableBlocks * 1024, nil
}

func (s *SSHStorage) TestConnection(ctx context.Context) error {
	if _, err := s.runCommand(ctx, s.testConnectionCommand()); err != nil {
		return err
	}

	return nil
}

func (s *SSHStorage) testConnectionCommand() string {
	trimmedBasePath := strings.TrimSpace(s.config.BasePath)
	if trimmedBasePath == "" {
		return "true"
	}

	return fmt.Sprintf("mkdir -p %s && test -d %s", shellQuote(trimmedBasePath), shellQuote(trimmedBasePath))
}

func (s *SSHStorage) runCommand(ctx context.Context, command string) (string, error) {
	client, err := s.connect(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(command); err != nil {
		trimmedStderr := strings.TrimSpace(stderr.String())
		if trimmedStderr == "" {
			return "", fmt.Errorf("run ssh command: %w", err)
		}
		return "", fmt.Errorf("run ssh command: %w: %s", err, trimmedStderr)
	}

	return stdout.String(), nil
}

func (s *SSHStorage) connect(ctx context.Context) (*ssh.Client, error) {
	sshConfig, address, err := s.buildSSHClientConfig()
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: s.timeout()}
	netConn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial ssh server: %w", err)
	}

	conn, channels, requests, err := ssh.NewClientConn(netConn, address, sshConfig)
	if err != nil {
		_ = netConn.Close()
		return nil, fmt.Errorf("establish ssh client connection: %w", err)
	}

	return ssh.NewClient(conn, channels, requests), nil
}

func (s *SSHStorage) buildSSHClientConfig() (*ssh.ClientConfig, string, error) {
	trimmedHost := strings.TrimSpace(s.config.Host)
	trimmedUser := strings.TrimSpace(s.config.User)
	trimmedPrivateKeyPath := strings.TrimSpace(s.config.PrivateKeyPath)
	if trimmedHost == "" {
		return nil, "", fmt.Errorf("ssh host is required")
	}
	if trimmedUser == "" {
		return nil, "", fmt.Errorf("ssh user is required")
	}
	if trimmedPrivateKeyPath == "" {
		return nil, "", fmt.Errorf("ssh private key path is required")
	}

	signer, err := loadSSHSigner(trimmedPrivateKeyPath)
	if err != nil {
		return nil, "", err
	}

	port := s.config.Port
	if port == 0 {
		port = 22
	}

	return &ssh.ClientConfig{
		User:            trimmedUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         s.timeout(),
	}, net.JoinHostPort(trimmedHost, strconv.Itoa(port)), nil
}

func loadSSHSigner(privateKeyPath string) (ssh.Signer, error) {
	privateKeyInfo, err := os.Stat(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("ssh private key file is unreadable")
	}
	if !privateKeyInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("ssh private key is invalid")
	}
	if privateKeyInfo.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("ssh private key permissions must be 0600")
	}

	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("ssh private key file is unreadable")
	}

	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("ssh private key is invalid")
	}

	return signer, nil
}

func (s *SSHStorage) resolveRemotePath(candidate string) (string, error) {
	trimmedBasePath := strings.TrimSpace(s.config.BasePath)
	trimmedCandidate := strings.TrimSpace(candidate)
	if trimmedBasePath == "" {
		if trimmedCandidate == "" {
			return ".", nil
		}
		return path.Clean(trimmedCandidate), nil
	}

	resolvedBasePath := path.Clean(trimmedBasePath)
	resolvedPath := resolvedBasePath
	if trimmedCandidate != "" && trimmedCandidate != "." {
		resolvedPath = path.Clean(path.Join(resolvedBasePath, trimmedCandidate))
	}

	if resolvedPath != resolvedBasePath && !strings.HasPrefix(resolvedPath, resolvedBasePath+"/") {
		return "", fmt.Errorf("path %q escapes ssh storage base path", candidate)
	}

	return resolvedPath, nil
}

func (s *SSHStorage) timeout() time.Duration {
	if s.config.Timeout > 0 {
		return s.config.Timeout
	}

	return 10 * time.Second
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}