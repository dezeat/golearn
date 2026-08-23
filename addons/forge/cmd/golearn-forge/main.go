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

// Command golearn-forge is the opt-in authoring binary (D-015).
//
// It is golearn plus question generation. The plain golearn binary has no
// network path at all and is unaffected by anything in this module; the offline
// law is binary-scoped, not key-scoped.
//
// Routing is manual os.Args handling, matching D-003 — no CLI framework. The
// full command surface, including the core's practice commands, is #129's
// story; this binary currently answers help, version and config so the module
// boundary can be proven end to end without a provider call.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dezeat/golearn/addons/forge/internal/config"
)

// version is overridden at release time via -ldflags. It is not read from the
// core module: the two binaries version independently.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run holds the routing so it is testable without spawning a process or
// touching os.Exit. Returning a code rather than exiting is what lets the
// boundary tests assert on real output, and taking io.Writer rather than
// *os.File is what lets them capture it.
func run(args []string, stdout, stderr io.Writer) int {
	var subcommand string
	for _, arg := range args {
		switch {
		case arg == "--help", arg == "-h", arg == "help":
			write(stdout, usage())
			return 0
		case arg == "--version", arg == "-v":
			write(stdout, fmt.Sprintf("golearn-forge %s\n", version))
			return 0
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			subcommand = arg
		}
		if subcommand != "" {
			break
		}
	}

	switch subcommand {
	case "":
		write(stdout, usage())
		return 0
	case "config":
		if err := config.Report(stdout, version); err != nil {
			write(stderr, fmt.Sprintf("error: %v\n", err))
			return 1
		}
		return 0
	case "generate":
		// Fail loudly and name the tracking issue rather than pretending the
		// surface exists. A silent no-op would be indistinguishable from a
		// generation that legitimately produced nothing.
		write(stderr, "error: generation is not available yet\n\n"+
			"The bounded generation pipeline lands with #122.\n"+
			"Run 'golearn-forge config' to see which surfaces are ready.\n")
		return 1
	default:
		write(stderr, fmt.Sprintf("error: unknown command %q\n\n", subcommand)+usage())
		return 1
	}
}

// write reports a failed write to stderr and otherwise swallows it. There is
// no useful recovery when the output stream itself is broken, but silently
// discarding the error would hide a closed pipe entirely.
func write(w io.Writer, s string) {
	if _, err := io.WriteString(w, s); err != nil && w != os.Stderr {
		fmt.Fprintf(os.Stderr, "warning: write failed: %v\n", err)
	}
}

func usage() string {
	return `golearn-forge — golearn plus assisted question authoring

Usage:
  golearn-forge <command> [args]

Commands:
  config      Show non-secret configuration and capability status
  generate    Generate a question pack (not available yet — #122)
  help        Show this help message

Flags:
  --version   Print the version and exit

Notes:
  Generation is opt-in and reaches the network only while authoring.
  The plain golearn binary has no network path and is unchanged.
  Practice, import, export and stats live in the golearn binary;
  routing them through this binary is tracked in #129.
`
}
