// Package cloak rewrites the client fingerprints in an Antigravity request's
// system instruction that Google answers with a false 429 RESOURCE_EXHAUSTED.
//
// It deliberately never inserts zero-width or other invisible characters:
// reports from November 2026 tie that kind of obfuscation to 403 TOS_VIOLATION
// bans. Flagged text is deleted or reworded into plain, neutral English.
package cloak

import (
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// DefaultIdentity replaces a client's opening identity sentence.
const DefaultIdentity = "You are a helpful coding assistant."

// Rule is one ordered regex replacement (Go RE2 syntax).
type Rule struct {
	Pattern     string `yaml:"pattern" json:"pattern"`
	Replacement string `yaml:"replacement" json:"replacement"`
}

// Config controls the rewriter.
type Config struct {
	// Identity replaces the opening "You are ..." sentence of each system part.
	Identity string
	// RewriteIdentity turns the opening-sentence rewrite on or off.
	RewriteIdentity bool
	// NeutralizeBrands replaces known agent product names with a neutral word.
	NeutralizeBrands bool
	// ExtraBrands adds product names to neutralize.
	ExtraBrands []string
	// ExtraRules run after the built-in rules.
	ExtraRules []Rule
}

// DefaultConfig is the zero-setup behavior.
func DefaultConfig() Config {
	return Config{Identity: DefaultIdentity, RewriteIdentity: true, NeutralizeBrands: true}
}

type compiledRule struct {
	re          *regexp.Regexp
	replacement string
}

// Built-in sentence rules, each tied to a public reproduction.
var builtinRules = []Rule{
	// oh-my-pi first line (omp #18706).
	{Pattern: `(?m)^[ \t]*RFC 2119 keywords?:[^\n]*\n?`, Replacement: ""},
	// oh-my-pi tool-doc sentence (CLIProxyAPI #7562).
	{Pattern: `Read xd://<tool> for full docs \+ JSON schema before first use\.?`, Replacement: "Consult the tool documentation and JSON schema before first use."},
	// pi coding agent first line (sub2api #8791, magpie #6655).
	{Pattern: `You are a coding assistant running inside pi, a coding agent harness\.?`, Replacement: DefaultIdentity},
	// Claude Code subagent identity (CLIProxyAPI #7049).
	{Pattern: `You are a Claude agent, built on Anthropic['’]s Claude Agent SDK\.?`, Replacement: DefaultIdentity},
	// Claude Code main identity.
	{Pattern: `You are Claude Code, Anthropic['’]s official CLI for Claude\.?`, Replacement: DefaultIdentity},
	// Codex identity (CLIProxyAPI #7428).
	{Pattern: `You are Codex, (?:an agent|a coding agent) based on GPT-[0-9.]+[^.\n]*\.?`, Replacement: DefaultIdentity},
	// OpenCode exact-string trigger (opencode-antigravity-auth #621).
	{Pattern: `session_move`, Replacement: "session-move"},
}

// Product names that mark a request as coming from a third-party agent.
// Longer names come first so "Claude Agent SDK" wins over "Claude Code".
var builtinBrands = []string{
	"Claude Agent SDK", "Claude Code", "PI-Desktop", "Pi-Desktop", "pi-desktop",
	"oh-my-pi", "OpenCode", "OpenChamber", "Hermes Agent", "Kilo Code", "Roo Code",
	"ZCode", "Cline", "Codex CLI", "Codex",
}

// Rewriter applies a compiled Config.
type Rewriter struct {
	cfg     Config
	rules   []compiledRule
	brandRe *regexp.Regexp
	ompRe   *regexp.Regexp
}

// New compiles cfg. Invalid extra rules are skipped and reported.
func New(cfg Config) (*Rewriter, []error) {
	if strings.TrimSpace(cfg.Identity) == "" {
		cfg.Identity = DefaultIdentity
	}
	var errs []error
	r := &Rewriter{cfg: cfg}
	for _, rule := range append(append([]Rule(nil), builtinRules...), cfg.ExtraRules...) {
		if strings.TrimSpace(rule.Pattern) == "" {
			continue
		}
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		replacement := rule.Replacement
		if replacement == DefaultIdentity {
			replacement = cfg.Identity
		}
		r.rules = append(r.rules, compiledRule{re: re, replacement: replacement})
	}
	if cfg.NeutralizeBrands {
		brands := append(append([]string(nil), cfg.ExtraBrands...), builtinBrands...)
		var quoted []string
		for _, b := range brands {
			if b = strings.TrimSpace(b); b != "" {
				quoted = append(quoted, regexp.QuoteMeta(b))
			}
		}
		if len(quoted) > 0 {
			r.brandRe = regexp.MustCompile(`\b(?:` + strings.Join(quoted, "|") + `)\b`)
		}
		// "omp" only as a standalone product word, e.g. "You are omp's agent".
		r.ompRe = regexp.MustCompile(`\bomp\b`)
	}
	return r, errs
}

var (
	openingRe    = regexp.MustCompile(`^(\s*)(You are [^\n]*?(?:\.(?:\s|$)|\n|$))`)
	multiSpaceRe = regexp.MustCompile(`[ \t]{2,}`)
)

// RewriteText rewrites one system-instruction text part.
func (r *Rewriter) RewriteText(text string) string {
	if text == "" {
		return text
	}
	out := text
	for _, rule := range r.rules {
		out = rule.re.ReplaceAllString(out, rule.replacement)
	}
	if r.cfg.RewriteIdentity {
		out = r.rewriteOpening(out)
	}
	if r.brandRe != nil {
		out = r.brandRe.ReplaceAllString(out, "assistant")
		out = r.ompRe.ReplaceAllString(out, "assistant")
		out = multiSpaceRe.ReplaceAllString(out, " ")
	}
	return out
}

// rewriteOpening replaces the first "You are ..." sentence, unless it is
// Antigravity's own identity or already the neutral sentence.
func (r *Rewriter) rewriteOpening(text string) string {
	m := openingRe.FindStringSubmatchIndex(text)
	if m == nil {
		return text
	}
	sentence := text[m[4]:m[5]]
	trimmed := strings.TrimSpace(sentence)
	if strings.Contains(trimmed, "Antigravity") || trimmed == r.cfg.Identity {
		return text
	}
	tail := ""
	if strings.HasSuffix(sentence, "\n") {
		tail = "\n"
	} else if strings.HasSuffix(sentence, " ") || strings.HasSuffix(sentence, "\t") {
		tail = " "
	}
	return text[:m[4]] + r.cfg.Identity + tail + text[m[5]:]
}

var instructionPaths = []string{
	"request.systemInstruction", "request.system_instruction",
	"systemInstruction", "system_instruction",
}

// RewriteBody rewrites every system-instruction text in an Antigravity payload.
// It returns the body unchanged when nothing matched.
func (r *Rewriter) RewriteBody(body []byte) ([]byte, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, false
	}
	changed := false
	out := body
	for _, path := range instructionPaths {
		inst := gjson.GetBytes(out, path)
		if !inst.Exists() {
			continue
		}
		if inst.Type == gjson.String {
			if next := r.RewriteText(inst.String()); next != inst.String() {
				if updated, err := sjson.SetBytes(out, path, next); err == nil {
					out, changed = updated, true
				}
			}
			continue
		}
		parts := inst.Get("parts")
		if !parts.IsArray() {
			continue
		}
		for i, part := range parts.Array() {
			t := part.Get("text")
			if t.Type != gjson.String {
				continue
			}
			if next := r.RewriteText(t.String()); next != t.String() {
				p := path + ".parts." + itoa(i) + ".text"
				if updated, err := sjson.SetBytes(out, p, next); err == nil {
					out, changed = updated, true
				}
			}
		}
	}
	return out, changed
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}
