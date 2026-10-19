package pi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	stall "github.com/bboykarlito/libstall"
)

func TestFakePiProcess(t *testing.T) {
	scenario := os.Getenv("STALL_FAKE_PI")
	if scenario == "" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var command map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &command); err != nil {
			os.Exit(2)
		}
		kind := command["type"].(string)
		if (kind == "set_auto_retry" || kind == "set_auto_compaction") && command["enabled"] != false {
			os.Exit(4)
		}
		response := map[string]any{"type": "response", "id": command["id"], "success": true}
		if scenario == "reject" && kind == "prompt" {
			response["success"] = false
			response["error"] = "rejected"
		}
		if kind == "get_session_stats" {
			response["data"] = map[string]any{"tokens": map[string]int{"input": 10, "output": 5, "cacheRead": 3, "cacheWrite": 2}}
			if scenario == "missing-stats" {
				response["data"] = map[string]any{}
			}
		}
		_ = json.NewEncoder(os.Stdout).Encode(response)
		if kind != "prompt" {
			continue
		}
		message, _ := command["message"].(string)
		if !strings.Contains(message, "Instructions:\nimplement") || !strings.Contains(message, "Task:\n/task") {
			os.Exit(3)
		}
		if scenario == "reject" {
			continue
		}
		if scenario == "exit" {
			fmt.Fprintln(os.Stderr, "fake crash")
			os.Exit(1)
		}
		if scenario == "malformed" {
			fmt.Println("not-json")
			continue
		}
		if scenario == "ui" {
			fmt.Println(`{"type":"extension_ui_request","method":"confirm","id":"dialog"}`)
			continue
		}
		if scenario == "no-response" {
			fmt.Println(`{"type":"agent_settled"}`)
			continue
		}
		fmt.Println(`{"type":"message_end","message":{"role":"user","content":"input"}}`)
		stop := "stop"
		if scenario == "model-error" {
			stop = "error"
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "message_end", "message": map[string]any{
			"role": "assistant", "content": []map[string]string{{"type": "text", "text": "done\u2028text"}}, "stopReason": stop, "errorMessage": "provider failure",
		}})
		fmt.Println(`{"type":"agent_end"}`)
		if scenario == "cancel" {
			continue
		} // Wait for termination, never settle.
		fmt.Println(`{"type":"agent_settled"}`)
	}
	os.Exit(0)
}

func fakeHarness(t *testing.T, scenario string) *Harness {
	t.Helper()
	h, err := New(ReviewConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	h.command = func(ctx context.Context, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFakePiProcess$", "--")
		cmd.Args = append(cmd.Args, args...)
		cmd.Env = append(os.Environ(), "STALL_FAKE_PI="+scenario)
		return cmd
	}
	return h
}

func TestExecute(t *testing.T) {
	for _, scenario := range []string{"success", "reject", "exit", "malformed", "model-error", "missing-stats", "ui", "no-response"} {
		t.Run(scenario, func(t *testing.T) {
			h := fakeHarness(t, scenario)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := h.Execute(ctx, stall.Request{Instructions: "implement", Task: stall.Task{Input: "/task"}})
			if scenario == "success" {
				if err != nil {
					t.Fatal(err)
				}
				if result.Output != "done\u2028text" || result.Usage.InputTokens != 15 || result.Usage.OutputTokens != 5 {
					t.Fatalf("unexpected result: %+v", result)
				}
			} else if err == nil {
				t.Fatal("expected failure")
			}
			if scenario == "model-error" && (result.Output == "" || result.Usage.OutputTokens != 5) {
				t.Fatalf("lost partial result: %+v", result)
			}
			if scenario == "exit" && !strings.Contains(err.Error(), "fake crash") {
				t.Fatalf("missing stderr: %v", err)
			}
		})
	}
}

func TestExecuteCancellation(t *testing.T) {
	h := fakeHarness(t, "cancel")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := h.Execute(ctx, stall.Request{Instructions: "implement", Task: stall.Task{Input: "/task"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
}

func TestNewSnapshotsTools(t *testing.T) {
	config := ReviewConfig(t.TempDir())
	h, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	config.Tools[0] = "bash"
	if h.config.Tools[0] != "read" {
		t.Fatal("configuration not snapshotted")
	}
}
