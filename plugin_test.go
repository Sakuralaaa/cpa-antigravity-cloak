package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestFlow(t *testing.T) {
	reg, err := handleMethod("plugin.register", []byte(`{"config_yaml":"`+base64.StdEncoding.EncodeToString([]byte("identity: You are an engineer.\nrules:\n  - pattern: foo\n    replacement: bar\n"))+`","schema_version":10}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reg), `"request_normalizer":true`) {
		t.Fatal(string(reg))
	}
	t.Log(string(reg))
	body := `{"request":{"systemInstruction":{"parts":[{"text":"You are PI-Desktop, a coding agent. foo"}]}}}`
	req, _ := json.Marshal(map[string]any{"FromFormat": "openai", "ToFormat": "antigravity", "Model": "m", "Body": []byte(body)})
	out, err := handleMethod("request.normalize", req)
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		OK     bool
		Result struct{ Body []byte }
	}
	json.Unmarshal(out, &env)
	if !env.OK || !strings.Contains(string(env.Result.Body), "You are an engineer. bar") {
		t.Fatal(string(out))
	}
	req2, _ := json.Marshal(map[string]any{"ToFormat": "claude", "Body": []byte(body)})
	out, _ = handleMethod("request.normalize", req2)
	json.Unmarshal(out, &env)
	if string(env.Result.Body) != body {
		t.Fatal("touched non-antigravity")
	}
}
