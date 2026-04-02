package executor

import (
	"fmt"
	"strings"
)

func BuildArchiveCommand(sourceDir, outputBase string, volumeSize *string) CommandSpec {
	trimmedSourceDir := strings.TrimSpace(sourceDir)
	trimmedOutputBase := strings.TrimSpace(outputBase)
	archivePath := trimmedOutputBase + ".tar.gz"

	if volumeSize == nil || strings.TrimSpace(*volumeSize) == "" {
		return CommandSpec{Name: "tar", Args: []string{"czf", archivePath, "-C", trimmedSourceDir, "."}}
	}

	return CommandSpec{
		Name: "sh",
		Args: []string{"-c", fmt.Sprintf("tar czf - -C %s . | split -b %s - %s", shellQuote(trimmedSourceDir), strings.TrimSpace(*volumeSize), shellQuote(archivePath+".part_"))},
	}
}

func BuildArchiveExtractCommand(archivePath, restoreTarget string) CommandSpec {
	return CommandSpec{Name: "tar", Args: []string{"xzf", strings.TrimSpace(archivePath), "-C", strings.TrimSpace(restoreTarget)}}
}

func BuildSplitArchiveExtractCommand(parts []string, restoreTarget string) CommandSpec {
	quotedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart == "" {
			continue
		}
		quotedParts = append(quotedParts, shellQuote(trimmedPart))
	}

	return CommandSpec{
		Name: "sh",
		Args: []string{"-c", fmt.Sprintf("cat %s | tar xzf - -C %s", strings.Join(quotedParts, " "), shellQuote(strings.TrimSpace(restoreTarget)))},
	}
}

func SplitArchivePartPath(basePath string, index int) string {
	return strings.TrimSpace(basePath) + ".part_" + splitArchiveSuffix(index)
}

func ArchivePartPaths(basePath string, volumeCount int) []string {
	if volumeCount <= 0 {
		return nil
	}

	paths := make([]string, 0, volumeCount)
	for index := 0; index < volumeCount; index++ {
		paths = append(paths, SplitArchivePartPath(basePath, index))
	}

	return paths
}

func splitArchiveSuffix(index int) string {
	if index < 0 {
		index = 0
	}

	width := 2
	limit := 26 * 26
	for index >= limit {
		width++
		limit *= 26
	}

	suffix := make([]byte, width)
	remaining := index
	for position := width - 1; position >= 0; position-- {
		suffix[position] = byte('a' + remaining%26)
		remaining /= 26
	}

	return string(suffix)
}