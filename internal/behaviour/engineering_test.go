package behaviour

import (
	"context"
	"errors"
	"testing"

	stall "github.com/bboykarlito/libstall"
)

func TestPrepare(t *testing.T) {
	for name, b := range map[string]stall.Behaviour{"implement": Implement{}, "review": Review{}} {
		t.Run(name, func(t *testing.T) {
			if _, err := b.Prepare(context.Background(), stall.Task{Input: "  "}); err == nil {
				t.Fatal("accepted empty task")
			}
			task := stall.Task{Input: "Fix the parser"}
			request, err := b.Prepare(context.Background(), task)
			if err != nil || request.Task != task || request.Instructions == "" {
				t.Fatalf("request = %+v, err = %v", request, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := b.Prepare(ctx, task); !errors.Is(err, context.Canceled) {
				t.Fatalf("expected cancellation, got %v", err)
			}
			if err := b.Validate(ctx, request, stall.Result{}); !errors.Is(err, context.Canceled) {
				t.Fatalf("expected cancellation, got %v", err)
			}
		})
	}
}

func TestImplementationRequiresSummary(t *testing.T) {
	for _, output := range []string{"", "  "} {
		if err := (Implement{}).Validate(context.Background(), stall.Request{}, stall.Result{Output: output}); err == nil {
			t.Fatal("accepted empty summary")
		}
	}
	if err := (Implement{}).Validate(context.Background(), stall.Request{}, stall.Result{Output: "Added parser tests."}); err != nil {
		t.Fatal(err)
	}
}

func TestReviewVerdict(t *testing.T) {
	cases := []struct {
		name, output string
		valid        bool
	}{
		{"approved", `{"approved":true,"summary":"Checks look sound","findings":[]}`, true},
		{"rejected", `{"approved":false,"summary":"Bug found","findings":["panic"]}`, false},
		{"contradictory", `{"approved":true,"summary":"Looks good","findings":["panic"]}`, false},
		{"missing verdict", `{"summary":"Looks good"}`, false},
		{"missing summary", `{"approved":true}`, false},
		{"invalid JSON", "APPROVED", false},
		{"fenced JSON", "```json\n{}\n```", false},
		{"null", "null", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := (Review{}).Validate(context.Background(), stall.Request{}, stall.Result{Output: tc.output})
			if (err == nil) != tc.valid {
				t.Fatalf("valid = %v, error = %v", tc.valid, err)
			}
		})
	}
}
