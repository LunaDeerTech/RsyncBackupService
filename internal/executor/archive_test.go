package executor

import (
	"strings"
	"testing"
)

func TestBuildArchiveCommandWithoutSplit(t *testing.T) {
	spec := BuildArchiveCommand("/srv/source", "/tmp/archive", nil)

	if spec.Name != "tar" {
		t.Fatalf("expected tar command, got %q", spec.Name)
	}
	expectedArgs := []string{"czf", "/tmp/archive.tar.gz", "-C", "/srv/source", "."}
	if len(spec.Args) != len(expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, spec.Args)
	}
	for index, expectedArg := range expectedArgs {
		if spec.Args[index] != expectedArg {
			t.Fatalf("expected arg %d to be %q, got %q", index, expectedArg, spec.Args[index])
		}
	}
}

func TestBuildSplitArchiveCommand(t *testing.T) {
	volumeSize := "1G"
	spec := BuildArchiveCommand("/srv/source", "/tmp/archive", &volumeSize)

	if spec.Name != "sh" {
		t.Fatalf("expected shell command for split archive, got %q", spec.Name)
	}
	joinedArgs := strings.Join(spec.Args, " ")
	if !strings.Contains(joinedArgs, "split -b 1G") {
		t.Fatalf("expected split archive command to contain volume size, got %v", spec.Args)
	}
	if !strings.Contains(joinedArgs, "/tmp/archive.tar.gz.part_") {
		t.Fatalf("expected split archive command to use archive part prefix, got %v", spec.Args)
	}
}