package crypto

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	encryptedFileMagic       = "RBSENC1"
	encryptChunkSize         = 1024 * 1024
	encryptedNonceSize       = 12
	encryptedNoncePrefixSize = 4
)

func EncryptFile(inputPath, outputPath string, key []byte) error {
	return EncryptFileWithContext(context.Background(), inputPath, outputPath, key)
}

func EncryptFileWithContext(ctx context.Context, inputPath, outputPath string, key []byte) error {
	return transformFile(ctx, inputPath, outputPath, key, encryptChunk)
}

func DecryptFile(inputPath, outputPath string, key []byte) error {
	return DecryptFileWithContext(context.Background(), inputPath, outputPath, key)
}

func DecryptFileWithContext(ctx context.Context, inputPath, outputPath string, key []byte) error {
	return transformFile(ctx, inputPath, outputPath, key, decryptChunk)
}

type chunkTransformer func(context.Context, cipher.AEAD, io.Reader, io.Writer) error

func transformFile(ctx context.Context, inputPath, outputPath string, key []byte, transformer chunkTransformer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(inputPath) == "" || strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("input and output paths are required")
	}
	if len(key) == 0 {
		return fmt.Errorf("encryption key is required")
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input file %q: %w", inputPath, err)
	}
	defer inputFile.Close()

	aead, err := newFileAEAD(key)
	if err != nil {
		return err
	}

	tempPath, cleanup, err := createTempOutput(outputPath)
	if err != nil {
		return err
	}
	defer cleanup()

	outputFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open output file %q: %w", tempPath, err)
	}

	writer := bufio.NewWriter(outputFile)
	transformErr := transformer(ctx, aead, inputFile, writer)
	flushErr := writer.Flush()
	closeErr := outputFile.Close()
	if transformErr != nil {
		return transformErr
	}
	if flushErr != nil {
		return fmt.Errorf("flush output file %q: %w", tempPath, flushErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close output file %q: %w", tempPath, closeErr)
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("rename output file %q -> %q: %w", tempPath, outputPath, err)
	}

	cleanup = func() {}
	return nil
}

func newFileAEAD(key []byte) (cipher.AEAD, error) {
	checksum := sha256.Sum256(key)
	block, err := aes.NewCipher(checksum[:])
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create aes-gcm cipher: %w", err)
	}
	if aead.NonceSize() != encryptedNonceSize {
		return nil, fmt.Errorf("unexpected aes-gcm nonce size %d", aead.NonceSize())
	}

	return aead, nil
}

func DeriveAESKey(secret string) []byte {
	checksum := sha256.Sum256([]byte(secret))
	key := make([]byte, len(checksum))
	copy(key, checksum[:])
	return key
}

func AESEncrypt(plaintext string, key []byte) (string, error) {
	if len(key) == 0 {
		return "", fmt.Errorf("encryption key is required")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create aes-gcm cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate aes nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func AESDecrypt(ciphertext string, key []byte) (string, error) {
	if len(key) == 0 {
		return "", fmt.Errorf("encryption key is required")
	}

	payload, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create aes-gcm cipher: %w", err)
	}
	if len(payload) < aead.NonceSize() {
		return "", fmt.Errorf("ciphertext payload is too short")
	}

	nonce := payload[:aead.NonceSize()]
	encrypted := payload[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}

func encryptChunk(ctx context.Context, aead cipher.AEAD, input io.Reader, output io.Writer) error {
	if _, err := io.WriteString(output, encryptedFileMagic); err != nil {
		return fmt.Errorf("write encryption header: %w", err)
	}

	noncePrefix := make([]byte, encryptedNoncePrefixSize)
	if _, err := rand.Read(noncePrefix); err != nil {
		return fmt.Errorf("generate encryption nonce: %w", err)
	}
	if _, err := output.Write(noncePrefix); err != nil {
		return fmt.Errorf("write encryption nonce prefix: %w", err)
	}

	buffer := make([]byte, encryptChunkSize)
	lengthBuffer := make([]byte, 4)
	var counter uint64

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		readBytes, readErr := input.Read(buffer)
		if readBytes > 0 {
			nonce := makeChunkNonce(noncePrefix, counter)
			ciphertext := aead.Seal(nil, nonce, buffer[:readBytes], nil)
			if len(ciphertext) > int(^uint32(0)) {
				return fmt.Errorf("encrypted chunk exceeds size limit")
			}
			binary.BigEndian.PutUint32(lengthBuffer, uint32(len(ciphertext)))
			if _, err := output.Write(lengthBuffer); err != nil {
				return fmt.Errorf("write encrypted chunk length: %w", err)
			}
			if _, err := output.Write(ciphertext); err != nil {
				return fmt.Errorf("write encrypted chunk: %w", err)
			}
			counter++
		}

		if readErr == nil {
			continue
		}
		if readErr == io.EOF {
			break
		}
		return fmt.Errorf("read input file: %w", readErr)
	}

	return nil
}

func decryptChunk(ctx context.Context, aead cipher.AEAD, input io.Reader, output io.Writer) error {
	header := make([]byte, len(encryptedFileMagic))
	if _, err := io.ReadFull(input, header); err != nil {
		return fmt.Errorf("read encryption header: %w", err)
	}
	if string(header) != encryptedFileMagic {
		return fmt.Errorf("invalid encrypted file header")
	}

	noncePrefix := make([]byte, encryptedNoncePrefixSize)
	if _, err := io.ReadFull(input, noncePrefix); err != nil {
		return fmt.Errorf("read encryption nonce prefix: %w", err)
	}

	lengthBuffer := make([]byte, 4)
	var counter uint64

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		_, err := io.ReadFull(input, lengthBuffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				return fmt.Errorf("read encrypted chunk length: %w", err)
			}
			return fmt.Errorf("read encrypted chunk length: %w", err)
		}

		chunkLength := binary.BigEndian.Uint32(lengthBuffer)
		ciphertext := make([]byte, chunkLength)
		if _, err := io.ReadFull(input, ciphertext); err != nil {
			return fmt.Errorf("read encrypted chunk: %w", err)
		}

		nonce := makeChunkNonce(noncePrefix, counter)
		plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return fmt.Errorf("decrypt encrypted chunk: %w", err)
		}
		if _, err := output.Write(plaintext); err != nil {
			return fmt.Errorf("write decrypted chunk: %w", err)
		}
		counter++
	}

	return nil
}

func makeChunkNonce(prefix []byte, counter uint64) []byte {
	nonce := make([]byte, encryptedNonceSize)
	copy(nonce, prefix)
	binary.BigEndian.PutUint64(nonce[len(prefix):], counter)
	return nonce
}

func createTempOutput(outputPath string) (string, func(), error) {
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("create output directory %q: %w", outputDir, err)
	}

	tempFile, err := os.CreateTemp(outputDir, ".rbs-encrypted-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp output file in %q: %w", outputDir, err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", nil, fmt.Errorf("close temp output file %q: %w", tempPath, err)
	}

	cleanup := func() {
		_ = os.Remove(tempPath)
	}
	return tempPath, cleanup, nil
}
