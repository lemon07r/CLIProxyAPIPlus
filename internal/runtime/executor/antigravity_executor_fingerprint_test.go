package executor

import (
	"strings"
	"testing"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/tidwall/gjson"
)

func resetAntigravityAccountVersionsForTest() {
	antigravityAccountVersionsMu.Lock()
	defer antigravityAccountVersionsMu.Unlock()
	antigravityAccountVersions = make(map[string]string)
}

func TestResolveUserAgent_UsesPerAccountFingerprintWithoutDowngrade(t *testing.T) {
	resetAntigravityAccountVersionsForTest()

	antigravityAccountVersionsMu.Lock()
	antigravityAccountVersions["acct-1"] = "9.9.9"
	antigravityAccountVersionsMu.Unlock()

	auth := &cliproxyauth.Auth{ID: "acct-1"}
	got1 := resolveUserAgent(auth)
	got2 := resolveUserAgent(auth)

	if got1 != got2 {
		t.Fatalf("resolveUserAgent() should be stable per account, got %q then %q", got1, got2)
	}
	if !strings.HasPrefix(got1, "antigravity/9.9.9 ") {
		t.Fatalf("resolveUserAgent() = %q, want version prefix %q", got1, "antigravity/9.9.9 ")
	}
}

func TestGenerateStableSessionID_UsesStructuredStablePrefix(t *testing.T) {
	payload := []byte(`{"request":{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}}`)

	got1 := generateStableSessionID(payload, "acct-1", "claude-opus-4-6", "quiet-reef-12345")
	got2 := generateStableSessionID(payload, "acct-1", "claude-opus-4-6", "quiet-reef-12345")

	prefix1, _, ok1 := strings.Cut(got1, ":seed-")
	prefix2, _, ok2 := strings.Cut(got2, ":seed-")
	if !ok1 || !ok2 {
		t.Fatalf("session IDs should contain seed suffix, got %q and %q", got1, got2)
	}
	if prefix1 != prefix2 {
		t.Fatalf("stable session prefix mismatch: %q vs %q", prefix1, prefix2)
	}
	if !strings.Contains(got1, ":claude-opus-4-6:quiet-reef-12345:seed-") {
		t.Fatalf("session ID should embed model and project, got %q", got1)
	}
}

func TestGenerateStableSessionID_FallsBackWithoutStableInputs(t *testing.T) {
	got := generateStableSessionID([]byte(`{"request":{"contents":[]}}`), "", "", "")
	if !strings.HasPrefix(got, "-") {
		t.Fatalf("fallback session ID should start with '-', got %q", got)
	}
	if strings.Contains(got, ":seed-") {
		t.Fatalf("fallback session ID should not use structured seed form, got %q", got)
	}
}

func TestGeminiToAntigravity_InjectsStructuredSessionID(t *testing.T) {
	payload := []byte(`{"request":{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}}`)
	out := geminiToAntigravity("claude-opus-4-6", payload, "quiet-reef-12345", "acct-1")

	if got := gjson.GetBytes(out, "project").String(); got != "quiet-reef-12345" {
		t.Fatalf("project = %q, want %q", got, "quiet-reef-12345")
	}
	sessionID := gjson.GetBytes(out, "request.sessionId").String()
	if !strings.Contains(sessionID, ":claude-opus-4-6:quiet-reef-12345:seed-") {
		t.Fatalf("session ID = %q, want structured fingerprint format", sessionID)
	}
}
