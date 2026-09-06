// Copyright 2026 dezeat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"strings"
	"testing"
)

// The tests below encode #125's acceptance criteria directly. The criterion is
// the specification, so these are written against it rather than against the
// implementation that happens to satisfy it today.

// TestStartsAndReportsWithoutProviderCall covers the acceptance criterion
// "golearn-forge can start and report help/configuration without a provider
// call". No provider exists yet to call, so what is actually asserted is the
// durable half: these commands succeed, produce output, and complete without
// any configured provider.
func TestStartsAndReportsWithoutProviderCall(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"help"},
		{"--help"},
		{"-h"},
		{"config"},
		{"--version"},
	} {
		name := "no-args"
		if len(args) > 0 {
			name = strings.Join(args, " ")
		}
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr.String())
			}
			if stdout.Len() == 0 {
				t.Error("wrote nothing to stdout")
			}
			if stderr.Len() != 0 {
				t.Errorf("wrote to stderr on a success path: %s", stderr.String())
			}
		})
	}
}

// TestGenerateRefusesAnIncompleteRequestBeforeDoingAnything guards the
// fail-loud rule at the surface that now exists.
//
// It replaces an earlier guard that asserted `generate` reported itself
// unavailable and named #122. That guard was correct while the pipeline was
// absent and became a false specification the moment it landed — the rule it
// protected is "never exit 0 having silently done nothing", and this asserts
// the same rule against the shipped behavior.
func TestGenerateRefusesAnIncompleteRequestBeforeDoingAnything(t *testing.T) {
	cases := map[string][]string{
		"no arguments at all": {"generate"},
		"topic without model": {"generate", "--topic", "Go concurrency"},
		"model without topic": {"generate", "--model", "qwen3:4b"},
		"unparseable count":   {"generate", "--topic", "Go", "--model", "m", "--count", "many"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, &stdout, &stderr)

			if code == 0 {
				t.Error("exit code = 0; an incomplete request must never look like a successful run")
			}
			if stderr.Len() == 0 {
				t.Error("nothing was written to stderr")
			}
			if !strings.HasPrefix(stderr.String(), "error:") {
				t.Errorf("stderr should lead with the error: %q", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("wrote to stdout on a failure path: %q", stdout.String())
			}
		})
	}
}

// A dry run must reach no provider, open no database and generate nothing —
// it is the cheap way to check a command line before committing minutes to it.
func TestADryRunReportsThePlanWithoutGenerating(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"generate", "--topic", "Go concurrency", "--model", "qwen3:4b",
		"--count", "2", "--dry-run",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"dry run", "Go concurrency", "qwen3:4b"} {
		if !strings.Contains(out, want) {
			t.Errorf("plan does not mention %q:\n%s", want, out)
		}
	}
	// The plan must be honest about what is not wired, or a reader would
	// assume grounding ran.
	if !strings.Contains(out, "grounding") {
		t.Errorf("the plan should state the grounding status:\n%s", out)
	}
}

// TestUnknownCommandFailsWithUsage keeps the CLI honest about its own surface.
func TestUnknownCommandFailsWithUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"definitely-not-a-command"}, &stdout, &stderr)

	if code == 0 {
		t.Error("exit code = 0, want non-zero for an unknown command")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Error("an unknown command should print usage so the user can recover")
	}
}

// TestReportNamesTheOfflineGuarantee is a documentation guard for D-015. A user
// who runs help must be able to learn that the plain golearn binary stays
// offline; that promise is the whole reason two binaries exist.
func TestReportNamesTheOfflineGuarantee(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help failed: %s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "no network path") {
		t.Error("help does not state that the golearn binary has no network path (D-015)")
	}
}
