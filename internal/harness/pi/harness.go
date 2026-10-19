package pi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	stall "github.com/bboykarlito/libstall"
)

// Harness executes each request in a new ephemeral Pi RPC process.
// It provides neither a spend cap nor an OS sandbox. Callers should set a deadline.
type Harness struct {
	config  Config
	command func(context.Context, ...string) *exec.Cmd
}

var _ stall.Harness = (*Harness)(nil)

// New validates and snapshots configuration. Pi must be available on PATH when
// Execute is called; authentication remains managed by Pi.
func New(config Config) (*Harness, error) {
	if _, err := config.Args(); err != nil {
		return nil, err
	}
	config.Tools = append([]string(nil), config.Tools...)
	config.Skills = append([]string(nil), config.Skills...)
	return &Harness{config: config, command: func(ctx context.Context, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "pi", args...)
	}}, nil
}

// Execute preserves available output and usage on error. Cancellation kills and
// reaps the Pi process, but does not guarantee termination of all tool descendants.
func (h *Harness) Execute(ctx context.Context, request stall.Request) (result stall.Result, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	args, err := h.config.Args()
	if err != nil {
		return result, err
	}
	cmd := h.command(ctx, args...)
	cmd.Dir = h.config.WorkDir
	cmd.WaitDelay = time.Second
	stderr := &tailBuffer{}
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return result, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return result, err
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return result, err
	}
	// Unblock pipe IO even if a tool descendant retains inherited descriptors.
	stopClosing := context.AfterFunc(ctx, func() {
		_ = stdin.Close()
		_ = stdout.Close()
	})
	defer func() {
		stopClosing()
		stdin.Close()
		// RPC has no shutdown command. Completion is determined by the protocol,
		// not the exit status of this deliberately terminated process.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if err != nil && stderr.text != "" {
			err = fmt.Errorf("%w; pi stderr: %s", err, stderr.text)
		}
	}()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)
	client := rpcClient{scanner: scanner, encoder: json.NewEncoder(stdin), result: &result}
	for _, command := range []map[string]any{
		{"type": "set_auto_retry", "enabled": false},
		{"type": "set_auto_compaction", "enabled": false},
	} {
		if err := client.call(command); err != nil {
			return result, err
		}
	}
	// Prefixing the message avoids treating a task beginning with '/' as a Pi command.
	message := "Instructions:\n" + request.Instructions + "\n\nTask:\n" + request.Task.Input
	if err := client.call(map[string]any{"type": "prompt", "message": message}); err != nil {
		return result, err
	}
	for !client.settled {
		if _, err := client.next(); err != nil {
			return result, err
		}
	}
	// Statistics include tool-reported usage as well as assistant messages.
	if err := client.call(map[string]any{"type": "get_session_stats"}); err != nil {
		return result, err
	}
	if client.failure != nil {
		return result, client.failure
	}
	if !client.completed {
		return result, errors.New("pi settled without a completed assistant response")
	}
	return result, nil
}

type rpcEvent struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Success bool            `json:"success"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason   string `json:"stopReason"`
		ErrorMessage string `json:"errorMessage"`
	} `json:"message"`
}

type rpcClient struct {
	scanner            *bufio.Scanner
	encoder            *json.Encoder
	result             *stall.Result
	sequence           int
	settled, completed bool
	failure            error
}

func (c *rpcClient) call(command map[string]any) error {
	c.sequence++
	id := fmt.Sprint(c.sequence)
	command["id"] = id
	if err := c.encoder.Encode(command); err != nil {
		return fmt.Errorf("write pi command: %w", err)
	}
	for {
		event, err := c.next()
		if err != nil {
			return err
		}
		if event.Type != "response" || event.ID != id {
			continue
		}
		if !event.Success {
			return fmt.Errorf("pi %s rejected: %s", command["type"], event.Error)
		}
		if command["type"] == "get_session_stats" {
			var stats struct {
				Tokens *struct {
					Input      *int64 `json:"input"`
					Output     *int64 `json:"output"`
					CacheRead  *int64 `json:"cacheRead"`
					CacheWrite *int64 `json:"cacheWrite"`
				} `json:"tokens"`
			}
			if err := json.Unmarshal(event.Data, &stats); err != nil {
				return fmt.Errorf("decode pi statistics: %w", err)
			}
			if stats.Tokens == nil || stats.Tokens.Input == nil || stats.Tokens.Output == nil || stats.Tokens.CacheRead == nil || stats.Tokens.CacheWrite == nil {
				return errors.New("pi statistics missing token counts")
			}
			t := stats.Tokens
			if *t.Input < 0 || *t.Output < 0 || *t.CacheRead < 0 || *t.CacheWrite < 0 {
				return errors.New("pi reported negative token counts")
			}
			c.result.Usage = stall.Usage{InputTokens: *t.Input + *t.CacheRead + *t.CacheWrite, OutputTokens: *t.Output}
		}
		return nil
	}
}

func (c *rpcClient) next() (rpcEvent, error) {
	var event rpcEvent
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return event, fmt.Errorf("read pi event: %w", err)
		}
		return event, fmt.Errorf("pi RPC ended before completion: %w", io.ErrUnexpectedEOF)
	}
	// Decode the envelope first: non-assistant messages may have string content.
	var envelope struct {
		Type    string          `json:"type"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(c.scanner.Bytes(), &envelope); err != nil {
		return event, fmt.Errorf("decode pi event: %w", err)
	}
	if envelope.Type == "extension_ui_request" {
		return event, errors.New("unexpected Pi UI request in headless execution")
	}
	if envelope.Type == "message_end" {
		var role struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal(envelope.Message, &role); err != nil {
			return event, err
		}
		if role.Role != "assistant" {
			return event, nil
		}
	}
	if envelope.Type != "message_end" && envelope.Type != "response" && envelope.Type != "agent_settled" && envelope.Type != "extension_error" {
		return event, nil
	}
	if err := json.Unmarshal(c.scanner.Bytes(), &event); err != nil {
		return event, fmt.Errorf("decode pi event: %w", err)
	}
	switch event.Type {
	case "agent_settled":
		c.settled = true
	case "extension_error":
		return event, fmt.Errorf("pi extension error: %s", event.Error)
	case "message_end":
		var text strings.Builder
		for _, block := range event.Message.Content {
			if block.Type == "text" {
				text.WriteString(block.Text)
			}
		}
		c.result.Output = text.String()
		c.completed = event.Message.StopReason == "stop"
		switch event.Message.StopReason {
		case "error", "aborted", "length":
			c.failure = fmt.Errorf("pi response ended with %s: %s", event.Message.StopReason, event.Message.ErrorMessage)
		}
	}
	return event, nil
}

// tailBuffer bounds retained stderr while allowing the child to continue writing.
type tailBuffer struct{ text string }

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.text += string(p)
	if len(b.text) > 8192 {
		b.text = b.text[len(b.text)-8192:]
	}
	return len(p), nil
}
