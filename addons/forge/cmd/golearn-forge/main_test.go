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

// TestUnavailableSurfacesFailClearly guards the fail-loud rule. A surface that
// has not landed must say so and name where it is tracked; exiting 0 with no
// output would be indistinguishable from a run that legitimately produced
// nothing, which is precisely the failure mode the epic forbids.
func TestUnavailableSurfacesFailClearly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"generate"}, &stdout, &stderr)

	if code == 0 {
		t.Error("exit code = 0, want non-zero for an unavailable surface")
	}
	msg := stderr.String()
	if !strings.Contains(msg, "not available") {
		t.Errorf("stderr does not say the surface is unavailable: %q", msg)
	}
	if !strings.Contains(msg, "#122") {
		t.Errorf("stderr does not name the tracking issue: %q", msg)
	}
	if stdout.Len() != 0 {
		t.Errorf("wrote to stdout on a failure path: %q", stdout.String())
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
