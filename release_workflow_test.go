package yazio_test

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseWorkflowWritesChecksumsForDownloadableBasenames(t *testing.T) {
	workflowBytes, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(workflowBytes)
	oldCommand := "sha256sum dist/*.tar.gz dist/*.zip > dist/checksums.txt"
	if strings.Contains(workflow, oldCommand) {
		t.Fatalf("release workflow still writes checksum entries with dist/ prefixes via %q", oldCommand)
	}

	wantCommand := "(cd dist && sha256sum *.tar.gz *.zip > checksums.txt)"
	if got := strings.Count(workflow, wantCommand); got != 2 {
		t.Fatalf("release workflow should generate checksums from inside dist in both build paths; found %d occurrences of %q", got, wantCommand)
	}
}
