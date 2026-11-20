package testsupport

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// VerifyInjector runs the Minecraft 1.21.1 session library under the pinned launcher agent.
func VerifyInjector(t *testing.T, root, source, profileID, nickname, token string) {
	t.Helper()
	java := os.Getenv("MINECRAFT_AUTH_SMOKE_JAVA")
	agent := os.Getenv("MINECRAFT_AUTH_SMOKE_AGENT")
	classpath := os.Getenv("MINECRAFT_AUTH_SMOKE_CLASSPATH")
	if java == "" && agent == "" && classpath == "" {
		t.Skip("set MINECRAFT_AUTH_SMOKE_JAVA, MINECRAFT_AUTH_SMOKE_AGENT and MINECRAFT_AUTH_SMOKE_CLASSPATH")
	}
	if java == "" || agent == "" || classpath == "" {
		t.Fatal("injector smoke requires all three fixture paths")
	}
	data, err := os.ReadFile(agent)
	if err != nil {
		t.Fatal(err)
	}
	const pinnedSHA256 = "9c7f4343e6c82034958ffb48c14a2cb0c85928be7283103ce17da00c6d5a7b10"
	if fmt.Sprintf("%x", sha256.Sum256(data)) != pinnedSHA256 {
		t.Fatal("agent SHA-256 differs from the version pinned by Launcher")
	}
	probe, err := filepath.Abs(source)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, java, "-Dauthlibinjector.noLogFile", "-javaagent:"+agent+"="+root,
		"--class-path", classpath, "--source", "21", probe, profileID, nickname)
	command.Dir = t.TempDir()
	command.Stdin = strings.NewReader(token + "\n")
	command.Env = slices.DeleteFunc(os.Environ(), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return slices.Contains([]string{"JAVA_TOOL_OPTIONS", "_JAVA_OPTIONS", "JDK_JAVA_OPTIONS", "CLASSPATH"}, name)
	})
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "Injector session and signed texture passed") {
		t.Fatalf("Authlib Injector smoke failed: %v\n%s", err, strings.ReplaceAll(string(output), token, "[REDACTED]"))
	}
}
