package main

import (
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"

	"github.com/Sakuralaaa/cpa-antigravity-cloak/cloak"
	"gopkg.in/yaml.v3"
)

const (
	pluginID   = "antigravity-cloak"
	abiVersion = 1
	// Schema 1 is the original contract; request.normalize exists there and
	// every CPA v8 host accepts it.
	schemaVersion = 1

	methodRegister    = "plugin.register"
	methodReconfigure = "plugin.reconfigure"
	methodNormalize   = "request.normalize"
)

// pluginVersion is stamped at build time with -X main.pluginVersion=<tag>.
var pluginVersion = "0.1.0"

var current atomic.Pointer[cloak.Rewriter]

func init() {
	r, _ := cloak.New(cloak.DefaultConfig())
	current.Store(r)
}

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

// transformRequest mirrors pluginapi.RequestTransformRequest (no json tags upstream).
type transformRequest struct {
	FromFormat string
	ToFormat   string
	Model      string
	Stream     bool
	Body       []byte
}

// payloadResponse mirrors pluginapi.PayloadResponse.
type payloadResponse struct {
	Body []byte
}

type configField struct {
	Name        string
	Type        string
	Default     any `json:",omitempty"`
	Description string
}

type metadata struct {
	Name             string
	Version          string
	Author           string
	GitHubRepository string
	Logo             string `json:",omitempty"`
	ConfigFields     []configField
}

type registration struct {
	SchemaVersion uint32   `json:"schema_version"`
	Metadata      metadata `json:"metadata"`
	Capabilities  struct {
		RequestNormalizer bool `json:"request_normalizer"`
	} `json:"capabilities"`
}

type pluginConfig struct {
	Identity         string       `yaml:"identity"`
	RewriteIdentity  *bool        `yaml:"rewrite-identity"`
	NeutralizeBrands *bool        `yaml:"neutralize-brands"`
	ExtraBrands      []string     `yaml:"extra-brands"`
	Rules            []cloak.Rule `yaml:"rules"`
}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case methodRegister, methodReconfigure:
		if err := configure(request); err != nil {
			return nil, err
		}
		return okEnvelope(buildRegistration())
	case methodNormalize:
		return normalize(request)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func configure(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return err
		}
	}
	cfg := cloak.DefaultConfig()
	if len(req.ConfigYAML) > 0 {
		var pc pluginConfig
		if err := yaml.Unmarshal(req.ConfigYAML, &pc); err != nil {
			return err
		}
		if s := strings.TrimSpace(pc.Identity); s != "" {
			cfg.Identity = s
		}
		if pc.RewriteIdentity != nil {
			cfg.RewriteIdentity = *pc.RewriteIdentity
		}
		if pc.NeutralizeBrands != nil {
			cfg.NeutralizeBrands = *pc.NeutralizeBrands
		}
		cfg.ExtraBrands = pc.ExtraBrands
		cfg.ExtraRules = pc.Rules
	}
	r, errs := cloak.New(cfg)
	current.Store(r)
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func buildRegistration() registration {
	var reg registration
	reg.SchemaVersion = schemaVersion
	reg.Metadata = metadata{
		Name:             pluginID,
		Version:          pluginVersion,
		Author:           "Sakuralaaa",
		GitHubRepository: "https://github.com/Sakuralaaa/cpa-antigravity-cloak",
		ConfigFields: []configField{
			{Name: "identity", Type: "string", Default: cloak.DefaultIdentity, Description: "Neutral sentence that replaces the client's opening \"You are ...\" identity line."},
			{Name: "rewrite-identity", Type: "boolean", Default: true, Description: "Rewrite the opening \"You are ...\" sentence of every system-instruction part (Antigravity's own identity is kept)."},
			{Name: "neutralize-brands", Type: "boolean", Default: true, Description: "Replace agent product names (PI-Desktop, Claude Code, Codex, OpenCode, omp...) in the system instruction with a neutral word."},
			{Name: "extra-brands", Type: "array", Description: "More product names to neutralize."},
			{Name: "rules", Type: "array", Description: "Extra ordered Go RE2 rules: [{pattern, replacement}]. An empty replacement deletes the match."},
		},
	}
	reg.Capabilities.RequestNormalizer = true
	return reg
}

func normalize(raw []byte) ([]byte, error) {
	var req transformRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	body := req.Body
	if strings.EqualFold(strings.TrimSpace(req.ToFormat), "antigravity") {
		if r := current.Load(); r != nil {
			if out, changed := r.RewriteBody(body); changed {
				body = out
			}
		}
	}
	return okEnvelope(payloadResponse{Body: body})
}

func okEnvelope(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}
