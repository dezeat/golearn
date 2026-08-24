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

package boundary_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// coreRuntimeDeps is the four-dependency ceiling D-015 fixes for the root
// module. Forge owns every provider SDK, HTTP client and retrieval dependency
// in its own module; none of them may appear here.
var coreRuntimeDeps = []string{
	"github.com/charmbracelet/bubbletea",
	"github.com/charmbracelet/lipgloss",
	"gopkg.in/yaml.v3",
	"modernc.org/sqlite",
}

// forbiddenCoreImports are the packages whose presence in the core binary's
// transitive graph would mean a network path had been linked into the offline
// product. net and net/url are deliberately absent from this list: both are
// already pulled in transitively by github.com/google/uuid and are unreachable
// from first-party code, so listing them would assert something untrue.
var forbiddenCoreImports = []string{
	"net/http",
	"net/http/httputil",
	"net/rpc",
	"net/smtp",
	"crypto/tls",
}

// forbiddenFirstPartyImports additionally bans the low-level primitives in
// first-party code. A transitive dependency may carry them; golearn's own
// packages may not reach for them.
var forbiddenFirstPartyImports = []string{
	"net",
	"net/url",
	"net/http",
	"crypto/tls",
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

// goList runs `go list` inside the root module with the network switched off,
// so the guard itself cannot make the check non-deterministic or online.
func goList(t *testing.T, args ...string) []string {
	t.Helper()
	root := moduleRoot(t)
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GOPROXY=off",
		"GOFLAGS=-mod=readonly",
		"GOWORK=off",
	)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		// The core module has no requirement on the addon, so a core package
		// importing Forge fails to resolve before any assertion runs. Name the
		// rule rather than leaving a bare toolchain error: the module boundary
		// is enforcing D-015 structurally, and the reader should know that.
		if strings.Contains(stderr, "golearn/addons/") {
			t.Fatalf("a core package imports the Forge addon: the dependency "+
				"direction is Forge -> core only (D-015). The core module "+
				"cannot resolve it, so this fails at load time:\n%s", stderr)
		}
		t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, stderr)
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// TestCoreBinaryHasNoHTTPPath is the load-bearing guard for D-015. The plain
// golearn binary must not link an HTTP client, whatever a nested addon module
// does. GOWORK=off is set so a developer's local workspace file cannot mask a
// leak that CI would then catch.
func TestCoreBinaryHasNoHTTPPath(t *testing.T) {
	deps := goList(t, "list", "-deps", "./cmd/golearn")

	present := make(map[string]bool, len(deps))
	for _, d := range deps {
		present[d] = true
	}

	for _, forbidden := range forbiddenCoreImports {
		if present[forbidden] {
			t.Errorf("core binary links %q: the offline binary must have no network path (D-015)", forbidden)
		}
	}
}

// TestCoreModuleStaysAtFourRuntimeDeps fails if the root module's direct
// requirements drift from the ceiling D-015 fixes. go.mod is parsed directly
// rather than via `go list -m all`: the module-graph form needs to fetch every
// dependency's go.mod, which would make this guard require the network to
// prove that the core does not use it.
//
// Indirect requirements are ignored. They are the transitive consequence of
// the four, not a widening of the boundary.
func TestCoreModuleStaysAtFourRuntimeDeps(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	want := make(map[string]bool, len(coreRuntimeDeps))
	for _, d := range coreRuntimeDeps {
		want[d] = true
	}

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		// Anything carrying the indirect marker is transitive, not a direct
		// requirement, and is out of scope for the ceiling.
		if strings.Contains(line, "// indirect") {
			continue
		}
		line = strings.TrimPrefix(line, "require ")
		fields := strings.Fields(line)
		// A direct requirement inside the block is "<path> <version>"; every
		// other line (module, go, toolchain, require, parens) is not.
		if len(fields) != 2 || !strings.HasPrefix(fields[1], "v") || !strings.Contains(fields[0], ".") {
			continue
		}
		path := fields[0]
		if !want[path] {
			t.Errorf("unexpected direct runtime dependency %q: the core module is fixed at four (D-015); Forge dependencies belong in addons/forge", path)
			continue
		}
		delete(want, path)
	}

	for missing := range want {
		t.Errorf("expected direct runtime dependency %q is gone: update coreRuntimeDeps deliberately, with a decision entry", missing)
	}
}

// TestFirstPartyCodeImportsNoNetwork walks the core's own packages and fails if
// any of them imports a network primitive directly. This catches a leak that
// the graph-level check cannot: first-party code reaching for net when a
// transitive dependency has already put it in the graph.
func TestFirstPartyCodeImportsNoNetwork(t *testing.T) {
	// All three import sets, not just .Imports.
	//
	// This guard previously extracted .TestImports into a field it never read,
	// and omitted .XTestImports from the template entirely — and core test code
	// is almost all XTest. An independent review probe added net/http, net/url
	// and crypto/tls to two core test files and the guard stayed green. It was
	// scanning a third of the code it claimed to cover.
	//
	// Test code is in scope on purpose: a test that dials is still a core
	// package reaching for the network, and it is the likeliest place for one
	// to appear, since "it is only a test" is exactly the reasoning that
	// precedes it.
	pkgs := goList(t, "list", "-f",
		"{{.ImportPath}}|{{join .Imports \",\"}}|{{join .TestImports \",\"}}|{{join .XTestImports \",\"}}", "./...")

	banned := make(map[string]bool, len(forbiddenFirstPartyImports))
	for _, b := range forbiddenFirstPartyImports {
		banned[b] = true
	}

	const importSets = 3
	for _, line := range pkgs {
		parts := strings.SplitN(line, "|", importSets+1)
		if len(parts) < 2 {
			continue
		}
		pkg := parts[0]
		// Iterate every set the template produced rather than an enumerated
		// pair, so adding a set to the template cannot leave it unscanned.
		for _, set := range parts[1:] {
			for _, imp := range strings.Split(set, ",") {
				if banned[strings.TrimSpace(imp)] {
					t.Errorf("first-party package %s imports %q directly: the core has no network path (D-015)", pkg, imp)
				}
			}
		}
	}
}

// TestCoreNeverImportsForge enforces the other half of D-015's one-way rule.
// The dependency direction is Forge -> core and never the reverse: a core
// package importing the addon would drag Forge's provider and HTTP
// dependencies back into the offline binary, defeating the module split.
//
// This is checked from the core side on purpose. A guard living in the addon
// could be deleted along with the addon; this one fails the core's own gate.
func TestCoreNeverImportsForge(t *testing.T) {
	const forgePrefix = "github.com/dezeat/golearn/addons/"

	pkgs := goList(t, "list", "-f", "{{.ImportPath}}|{{join .Imports \",\"}}|{{join .TestImports \",\"}}", "./...")

	for _, line := range pkgs {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 2 {
			continue
		}
		pkg := parts[0]
		for _, group := range parts[1:] {
			for _, imp := range strings.Split(group, ",") {
				if strings.HasPrefix(strings.TrimSpace(imp), forgePrefix) {
					t.Errorf("core package %s imports %q: the dependency direction is Forge -> core only (D-015)", pkg, imp)
				}
			}
		}
	}
}
