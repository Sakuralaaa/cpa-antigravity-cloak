package cloak

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func rw(t *testing.T) *Rewriter {
	t.Helper()
	r, errs := New(DefaultConfig())
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	return r
}

func TestPIDesktop(t *testing.T) {
	in := "You are PI-Desktop, a coding agent. Answer in the user's language. Be concise, direct, and actionable.\n\nYou are operating in Plan mode as the same PI-Desktop agent, in a planning state."
	got := rw(t).RewriteText(in)
	want := "You are a helpful coding assistant. Answer in the user's language. Be concise, direct, and actionable.\n\nYou are operating in Plan mode as the same assistant agent, in a planning state."
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestPi(t *testing.T) {
	in := "You are a coding assistant running inside pi, a coding agent harness. You help users..."
	got := rw(t).RewriteText(in)
	if strings.Contains(got, "inside pi") || !strings.HasPrefix(got, DefaultIdentity) {
		t.Fatalf("got %q", got)
	}
}

func TestOmp(t *testing.T) {
	in := "RFC 2119 keywords: MUST, REQUIRED, SHOULD. `NEVER` = `MUST NOT`.\nYou are omp's trusted agent.\nRead xd://<tool> for full docs + JSON schema before first use.\n"
	got := rw(t).RewriteText(in)
	if strings.Contains(got, "RFC 2119") || strings.Contains(got, "xd://") || strings.Contains(got, "omp") {
		t.Fatalf("got %q", got)
	}
}

func TestClaudeCodexOpenCode(t *testing.T) {
	r := rw(t)
	for _, in := range []string{
		"You are a Claude agent, built on Anthropic's Claude Agent SDK.",
		"You are Claude Code, Anthropic's official CLI for Claude.",
		"You are Codex, an agent based on GPT-5.",
		"You are opencode, an interactive CLI tool. Use session_move to move.",
	} {
		got := r.RewriteText(in)
		if !strings.HasPrefix(got, DefaultIdentity) || strings.Contains(got, "session_move") || strings.Contains(got, "Claude Agent SDK") || strings.Contains(got, "Codex") {
			t.Fatalf("in %q got %q", in, got)
		}
	}
}

func TestKeepsAntigravityAndNoInvisible(t *testing.T) {
	in := "You are Antigravity, a powerful agentic AI coding assistant."
	if got := rw(t).RewriteText(in); got != in {
		t.Fatalf("got %q", got)
	}
	out := rw(t).RewriteText("You are PI-Desktop, a coding agent. Codex Claude Code")
	for _, r := range out {
		if r == '\u200b' || r == '\u034f' || r == '\u200c' || r == '\u200d' || r == '\u2060' || r == '\ufeff' {
			t.Fatalf("invisible char in %q", out)
		}
	}
}

func TestBody(t *testing.T) {
	body := []byte(`{"model":"x","request":{"systemInstruction":{"role":"user","parts":[{"text":"You are PI-Desktop, a coding agent. Be concise."},{"text":"plain"}]},"contents":[{"role":"user","parts":[{"text":"You are PI-Desktop"}]}]}}`)
	out, changed := rw(t).RewriteBody(body)
	if !changed {
		t.Fatal("not changed")
	}
	if got := gjson.GetBytes(out, "request.systemInstruction.parts.0.text").String(); got != "You are a helpful coding assistant. Be concise." {
		t.Fatalf("got %q", got)
	}
	if got := gjson.GetBytes(out, "request.contents.0.parts.0.text").String(); got != "You are PI-Desktop" {
		t.Fatalf("user content touched: %q", got)
	}
	if _, changed := rw(t).RewriteBody([]byte(`{"request":{"systemInstruction":{"parts":[{"text":"Be nice."}]}}}`)); changed {
		t.Fatal("unexpected change")
	}
}

func TestExtraRulesAndIdentity(t *testing.T) {
	r, errs := New(Config{Identity: "You are an engineer.", RewriteIdentity: true, ExtraRules: []Rule{{Pattern: `secret phrase`, Replacement: "x"}, {Pattern: `(`}}})
	if len(errs) != 1 {
		t.Fatalf("errs %v", errs)
	}
	if got := r.RewriteText("You are Foo. a secret phrase"); got != "You are an engineer. a x" {
		t.Fatalf("got %q", got)
	}
}

func TestToolDescriptions(t *testing.T) {
	body := []byte(`{"request":{"tools":[{"functionDeclarations":[{"name":"PluginCreate","description":"Create a PI-Desktop plugin from a template.","parametersJsonSchema":{"type":"object","properties":{"dir.x":{"type":"string","description":"PI-Desktop dir"}}}}]}]}}`)
	out, changed := rw(t).RewriteBody(body)
	if !changed {
		t.Fatal("not changed")
	}
	if strings.Contains(string(out), "PI-Desktop") {
		t.Fatalf("got %s", out)
	}
	if gjson.GetBytes(out, "request.tools.0.functionDeclarations.0.name").String() != "PluginCreate" {
		t.Fatal("name changed")
	}
	if gjson.GetBytes(out, "request.tools.0.functionDeclarations.0.description").String() != "Create a assistant plugin from a template." {
		t.Fatalf("got %s", out)
	}
}
