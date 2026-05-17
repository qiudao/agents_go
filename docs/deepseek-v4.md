# DeepSeek V4 Smoke Test

This note records how to verify DeepSeek V4 access for this project and for
other local projects. Do not paste API keys into this file or into command
output.

## Verified On 2026-05-17

Account/API access was verified against DeepSeek API with:

- `deepseek-v4-flash`
- `deepseek-v4-pro`
- OpenAI-compatible endpoint: `https://api.deepseek.com/chat/completions`
- Anthropic-compatible endpoint:
  `https://api.deepseek.com/anthropic/v1/messages`

The project provider path also worked with:

```go
NewAnthropicProvider(
    apiKey,
    "https://api.deepseek.com/anthropic",
    "deepseek-v4-pro",
)
```

Expected smoke response:

```text
deepseek-v4-ok
```

## Key Location

Use `DEEPSEEK_API_KEY` from either:

- environment variable `DEEPSEEK_API_KEY`
- local agents_go config: `~/.config/agents_go/keys.jsonl`

Never print or commit the key.

## Minimal Curl Checks

OpenAI-compatible API:

```bash
curl https://api.deepseek.com/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${DEEPSEEK_API_KEY}" \
  -d '{
    "model": "deepseek-v4-pro",
    "messages": [
      {"role": "user", "content": "Reply exactly: deepseek-v4-ok"}
    ],
    "stream": false
  }'
```

Anthropic-compatible API:

```bash
curl https://api.deepseek.com/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: ${DEEPSEEK_API_KEY}" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "deepseek-v4-pro",
    "max_tokens": 64,
    "messages": [
      {"role": "user", "content": "Reply exactly: deepseek-v4-ok"}
    ]
  }'
```

## Project Provider Smoke Test

For this repo, a temporary Go test can be used without changing production code.
Create a throwaway `deepseek_v4_smoke_test.go` with:

```go
package main

import (
    "context"
    "os"
    "strings"
    "testing"
    "time"
)

func TestDeepSeekV4Smoke(t *testing.T) {
    apiKey := os.Getenv("DEEPSEEK_API_KEY")
    if apiKey == "" {
        apiKey = loadConfig()["DEEPSEEK_API_KEY"]
    }
    if apiKey == "" {
        t.Fatal("DEEPSEEK_API_KEY is not configured")
    }

    provider := NewAnthropicProvider(apiKey, "https://api.deepseek.com/anthropic", "deepseek-v4-pro")
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    resp, err := provider.Chat(ctx, []Message{{
        Role: "user",
        Content: []ContentBlock{{
            Type: "text",
            Text: "Reply with exactly: deepseek-v4-ok",
        }},
    }}, nil)
    if err != nil {
        t.Fatalf("DeepSeek V4 request failed: %v", err)
    }

    var text strings.Builder
    for _, block := range resp.Content {
        if block.Type == "text" {
            text.WriteString(block.Text)
        }
    }
    t.Logf("input_tokens=%d content_blocks=%d response=%q", resp.InputTokens, len(resp.Content), text.String())
    if !strings.Contains(strings.ToLower(text.String()), "deepseek-v4-ok") {
        t.Fatalf("unexpected response: %q", text.String())
    }
}
```

Run only the provider-related files to avoid slow unrelated dependency builds:

```bash
go test -run TestDeepSeekV4Smoke -count=1 -timeout 90s -v \
  deepseek_v4_smoke_test.go provider.go provider_anthropic.go config.go
```

Delete the temporary test after the check.

## Dependency Notes

On the local Mac, key Go modules were present in the module cache after the
DeepSeek check, including:

- `github.com/anthropics/anthropic-sdk-go@v1.26.0`
- `google.golang.org/genai@v1.48.0`
- `cloud.google.com/go@v0.116.0`
- `google.golang.org/grpc@v1.66.2`

Running whole-package `go test` can trigger slow downloads and compilation for
Gemini/Google transitive dependencies. For a DeepSeek-only smoke test, compile
only the provider files shown above.

## tokyo0301 Notes

`tokyo0301` currently has `~/go/pkg/mod` but no `go` binary on `PATH`, so it
cannot run `go mod download` directly yet.

Network from `tokyo0301` to the Go module proxy was reachable during the check:

```bash
curl -I -L https://proxy.golang.org/google.golang.org/genai/@v/v1.48.0.mod
```

If a future project needs tokyo to download Go dependencies, first install or
unpack a Go toolchain under the ubuntu user, then run:

```bash
cd /path/to/project
go mod download
```

Alternative: download dependencies on the Mac and copy the relevant module cache
or build artifact to tokyo.
