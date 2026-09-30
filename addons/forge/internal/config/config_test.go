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

package config_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/config"
)

// credentialShapes are patterns that must never appear in user-facing Forge
// output. The list is deliberately about *shape* rather than about known key
// names: a guard that only knows today's providers stops working the moment a
// new one is added, which is exactly when it is needed most.
//
// This grows with #123, when real provider credentials first exist. It is
// written now so the invariant is in place before the risk is.
var credentialShapes = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`),     // OpenAI-style
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{16,}`), // Anthropic-style
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._-]{16,}`),
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*\S+`),
}

// TestReportLeaksNoCredentialShapes is the redaction guard. Forge's config
// output is the surface most likely to grow a credential by accident, because
// showing "which provider am I using" and showing "with what key" are one
// careless line apart.
func TestReportLeaksNoCredentialShapes(t *testing.T) {
	var sb strings.Builder
	if err := config.Report(&sb, "test"); err != nil {
		t.Fatalf("Report: %v", err)
	}
	out := sb.String()

	for _, re := range credentialShapes {
		if m := re.FindString(out); m != "" {
			t.Errorf("config output matches credential shape %q (matched %q)", re, m)
		}
	}
}

// TestReportSucceedsWithProviderEnvSet is the adversarial half. Setting
// provider environment variables must not change the report: if Report ever
// starts echoing its environment, this catches it before a user pastes the
// output into a bug report.
func TestReportSucceedsWithProviderEnvSet(t *testing.T) {
	const sentinel = "sk-test-DO-NOT-LEAK-0123456789abcdef"
	for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(key, sentinel)
	}

	var sb strings.Builder
	if err := config.Report(&sb, "test"); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if strings.Contains(sb.String(), sentinel) {
		t.Error("config output echoed a provider environment variable")
	}
	if strings.Contains(sb.String(), "DO-NOT-LEAK") {
		t.Error("config output contains the sentinel in some transformed form")
	}
}

// TestReportFailsWhenTheWriterFails checks the error path is real rather than
// decorative: Report returns an error, so something must be able to produce it.
func TestReportFailsWhenTheWriterFails(t *testing.T) {
	if err := config.Report(failingWriter{}, "test"); err == nil {
		t.Error("Report returned nil for a writer that always fails")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

// TestCapabilitiesTrackEveryPendingSurface keeps the seam honest. Every
// capability must name a tracking issue, so a user reading "pending" can find
// out when it lands rather than guessing.
func TestCapabilitiesTrackEveryPendingSurface(t *testing.T) {
	caps := config.Capabilities()
	if len(caps) == 0 {
		t.Fatal("no capabilities reported")
	}
	for _, c := range caps {
		if c.Name == "" {
			t.Error("capability with an empty name")
		}
		if !strings.HasPrefix(c.Tracked, "#") {
			t.Errorf("capability %q does not name a tracking issue (got %q)", c.Name, c.Tracked)
		}
	}
}
