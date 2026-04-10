package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
	"rsync-backup-service/internal/store"
)

func TestVerifySSHConnectionSuccess(t *testing.T) {
	clientKeyPEM, clientSigner := newSignerPEM(t)
	_, hostSigner := newSignerPEM(t)

	listener, done := startSSHTestServer(t, hostSigner, clientSigner.PublicKey())
	defer listener.Close()
	defer func() { <-done }()

	privateKeyPath := filepathJoin(t.TempDir(), "client.pem")
	if err := os.WriteFile(privateKeyPath, clientKeyPEM, 0o600); err != nil {
		t.Fatalf("WriteFile(client.pem) error = %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := VerifySSHConnection(ctx, model.RemoteConfig{
		Type:           "ssh",
		Host:           "127.0.0.1",
		Port:           port,
		Username:       "backup",
		PrivateKeyPath: privateKeyPath,
	})
	if err != nil {
		t.Fatalf("VerifySSHConnection() error = %v", err)
	}
}

func TestVerifySSHConnectionRejectsLoosePermissions(t *testing.T) {
	privateKeyPEM, _ := newSignerPEM(t)
	privateKeyPath := filepathJoin(t.TempDir(), "client.pem")
	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0o644); err != nil {
		t.Fatalf("WriteFile(client.pem) error = %v", err)
	}

	err := VerifySSHConnection(context.Background(), model.RemoteConfig{
		Type:           "ssh",
		Host:           "127.0.0.1",
		Port:           22,
		Username:       "backup",
		PrivateKeyPath: privateKeyPath,
	})
	if err == nil {
		t.Fatal("VerifySSHConnection() error = nil, want permission error")
	}
	if !strings.Contains(err.Error(), "0600") {
		t.Fatalf("VerifySSHConnection() error = %q, want permission error", err.Error())
	}
}

func TestRemoteConfigServiceUsesOpenListTester(t *testing.T) {
	dataDir := t.TempDir()
	db := newRemoteConfigServiceTestDB(t, dataDir)
	service := NewRemoteConfigService(db, dataDir, nil)
	tested := false
	service.SetOpenListTester(func(ctx context.Context, remote model.RemoteConfig) error {
		tested = true
		if !openlist.IsRemoteConfig(remote) {
			t.Fatalf("remote type = %q, want openlist", remote.Type)
		}
		return nil
	})

	encoded, err := openlist.EncodeStoredConfig("secret-pass", "")
	if err != nil {
		t.Fatalf("EncodeStoredConfig() error = %v", err)
	}
	remote := &model.RemoteConfig{
		Name:          "ol-service",
		Type:          "openlist",
		Host:          "https://openlist.example.com",
		Username:      "admin",
		CloudProvider: stringPtrForServiceTest("openlist"),
		CloudConfig:   encoded,
	}
	if err := db.CreateRemoteConfig(remote); err != nil {
		t.Fatalf("CreateRemoteConfig() error = %v", err)
	}

	if err := service.TestRemoteConfigConnection(context.Background(), remote.ID); err != nil {
		t.Fatalf("TestRemoteConfigConnection() error = %v", err)
	}
	if !tested {
		t.Fatal("openlist tester was not invoked")
	}
}

func stringPtrForServiceTest(value string) *string {
	return &value
}

func newSignerPEM(t *testing.T) ([]byte, ssh.Signer) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	signer, err := ssh.ParsePrivateKey(privateKeyPEM)
	if err != nil {
		t.Fatalf("ssh.ParsePrivateKey() error = %v", err)
	}

	return privateKeyPEM, signer
}

func startSSHTestServer(t *testing.T, hostSigner ssh.Signer, authorizedKey ssh.PublicKey) (net.Listener, <-chan struct{}) {
	t.Helper()

	serverConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), authorizedKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		},
	}
	serverConfig.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		sshConn, channels, requests, err := ssh.NewServerConn(conn, serverConfig)
		if err != nil {
			return
		}
		defer sshConn.Close()
		go ssh.DiscardRequests(requests)

		for newChannel := range channels {
			if newChannel.ChannelType() != "session" {
				_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel")
				continue
			}

			channel, requests, err := newChannel.Accept()
			if err != nil {
				continue
			}

			go func(in <-chan *ssh.Request, channel ssh.Channel) {
				defer channel.Close()
				for request := range in {
					switch request.Type {
					case "exec":
						var payload struct {
							Command string
						}
						_ = ssh.Unmarshal(request.Payload, &payload)
						ok := payload.Command == "echo ok"
						_ = request.Reply(ok, nil)
						if !ok {
							_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: 1}))
							return
						}
						_, _ = channel.Write([]byte("ok\n"))
						_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: 0}))
						return
					default:
						_ = request.Reply(false, nil)
					}
				}
			}(requests, channel)
		}
	}()

	return listener, done
}

func filepathJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, part := range parts[1:] {
		result = result + string(os.PathSeparator) + part
	}
	return result
}

func newRemoteConfigServiceTestDB(t *testing.T, dataDir string) *store.DB {
	t.Helper()

	db, err := store.New(dataDir)
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	})
	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate() error = %v", err)
	}
	return db
}
