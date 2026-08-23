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

// Package config reports the Forge binary's non-secret runtime configuration.
//
// It exists at this stage to give #125 an honest answer to "start and report
// configuration without a provider call", and to prove the one-way dependency
// direction D-015 requires: this package reads core contracts, and no core
// package knows it exists.
//
// It deliberately does not resolve provider credentials. Secret resolution is
// #123's story and is gated on #106; inventing a resolution order here would
// be exactly the silent policy invention the epic forbids.
package config

import (
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/dezeat/golearn/internal/adapters/sqlite"
)

// Capability is a Forge surface with the issue that delivers it. Reporting a
// surface as pending is more useful than omitting it: a user who asks what the
// binary can do gets told what is missing and where it is tracked, instead of
// discovering a silently absent feature at the point of use.
type Capability struct {
	Name    string
	Ready   bool
	Tracked string
}

// Capabilities lists the Forge V1 surfaces in the order FORGE.md §11 states
// them. Entries flip to Ready as their stories land.
func Capabilities() []Capability {
	return []Capability{
		{Name: "module and binary boundary", Ready: true, Tracked: "#125"},
		{Name: "run records, schema 0.2.0, drafts", Ready: false, Tracked: "#121"},
		{Name: "provider profiles and secrets", Ready: false, Tracked: "#123"},
		{Name: "web research and evidence records", Ready: false, Tracked: "#126"},
		{Name: "near-duplicate similarity gate", Ready: false, Tracked: "#124"},
		{Name: "bounded generation pipeline", Ready: false, Tracked: "#122"},
		{Name: "pack preview and accept/discard", Ready: false, Tracked: "#128"},
	}
}

// Report writes the non-secret configuration to w.
//
// Nothing written here may be, or may be derived from, a credential. The
// database path is a local filesystem path the user chose; provider endpoints
// and keys are absent because no provider is configurable yet.
//
// The report is assembled in memory and written once, so the single write is
// the only fallible operation and its error is actually checked. Streaming
// dozens of fmt.Fprintln calls would mean either ignoring dozens of errors or
// checking each one for no benefit.
func Report(w io.Writer, version string) error {
	var b strings.Builder

	fmt.Fprintf(&b, "golearn-forge %s (%s/%s, go %s)\n",
		version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	b.WriteString("\nDatabase\n")
	fmt.Fprintf(&b, "  path              %s\n", sqlite.DefaultDBPath())
	b.WriteString("  shared with core  yes (Forge extends the same database)\n")
	b.WriteString("\nProviders\n")
	b.WriteString("  configured        none\n")
	b.WriteString("  note              provider profiles land with #123\n")
	b.WriteString("\nCapabilities\n")
	for _, c := range Capabilities() {
		status := "pending"
		if c.Ready {
			status = "ready"
		}
		fmt.Fprintf(&b, "  %-8s %-36s %s\n", status, c.Name, c.Tracked)
	}

	_, err := io.WriteString(w, b.String())
	return err
}
