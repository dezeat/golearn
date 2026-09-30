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
	"os"
	"runtime"
	"strings"

	forgedomain "github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/internal/adapters/sqlite"
)

// coredomainProfiles is a seam so the report can be rendered without importing
// the domain package under a name that reads like the core's.
func coredomainProfiles() []forgedomain.Profile { return forgedomain.Profiles() }

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
		{Name: "run records, schema 0.2.0, drafts", Ready: true, Tracked: "#121"},
		{Name: "provider profiles and secrets", Ready: true, Tracked: "#123"},
		// Not Ready, and the distinction matters: the adapter and the evidence
		// records exist, but the source-authority policy and the V1 adapter
		// choice are #120's, and page fetch/extraction is not built. Reporting
		// this as ready would promise grounding the pipeline does not perform.
		{Name: "web research and evidence records", Ready: false, Tracked: "#126"},
		// Likewise: the backend and the gate exist and are tested, but no
		// embedding model has a committed calibration, so the gate refuses to
		// score rather than thresholding on a number picked by feel (D-022).
		{Name: "near-duplicate similarity gate", Ready: false, Tracked: "#124"},
		{Name: "bounded generation pipeline", Ready: true, Tracked: "#122"},
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
	for _, p := range coredomainProfiles() {
		source := "no credential needed"
		if p.CredentialEnvVar != "" {
			source = "set " + p.CredentialEnvVar
			if _, ok := os.LookupEnv(p.CredentialEnvVar); ok {
				// Says a credential was found. Never says what it is, and
				// never a fragment of it.
				source = p.CredentialEnvVar + " is set"
			}
		}
		embeds := "no embeddings capability"
		if p.Embeds {
			embeds = "embeddings available"
		}
		fmt.Fprintf(&b, "  %-12s %-24s %s\n", p.ID, source, embeds)
	}
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
