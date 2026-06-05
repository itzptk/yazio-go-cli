package yazio_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakefileUsesPathGoByDefault(t *testing.T) {
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Skip("make is not installed")
	}

	fakeBin := t.TempDir()
	fakeGo := filepath.Join(fakeBin, "go")
	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake go: %v", err)
	}

	cmd := exec.Command(makePath, "--no-print-directory", "-n", "test")
	cmd.Env = append(envWithout("GO", "MAKEFLAGS", "MAKEOVERRIDES", "MFLAGS"), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n test failed: %v\n%s", err, out)
	}

	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		t.Fatalf("make -n test produced no command output")
	}

	goCommand := filepath.Clean(fields[0])
	if goCommand != "go" && goCommand != fakeGo {
		t.Fatalf("make test should use go from PATH by default, got command %q from output:\n%s", fields[0], out)
	}
}

func envWithout(names ...string) []string {
	blocked := make(map[string]struct{}, len(names))
	for _, name := range names {
		blocked[name] = struct{}{}
	}

	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		name, _, found := strings.Cut(entry, "=")
		if found {
			if _, ok := blocked[name]; ok {
				continue
			}
		}
		if _, ok := blocked[entry]; ok {
			continue
		}
		env = append(env, entry)
	}
	return env
}
