# golearn animated hero — charmbracelet/vhs script.
#
# Renders assets/hero.gif: a short practice session followed by the stats
# dashboard. Deterministic and re-runnable — it seeds a throwaway demo DB at
# /tmp/golearn-demo.db from a bundled pack, so the recording never depends on
# your real ~/.golearn data or on wall-clock state.
#
# Prerequisites: `vhs` (https://github.com/charmbracelet/vhs) and a built
# binary at ./bin/golearn (run `make build` first).
#
# Re-render:
#   make build
#   rm -f /tmp/golearn-demo.db*
#   ./bin/golearn --db /tmp/golearn-demo.db import packs/go-basics.yaml
#   vhs assets/demo.tape

Output assets/hero.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 700
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms
Set PlaybackSpeed 1.0

# Seed a deterministic, throwaway demo database from a bundled pack.
Hide
Type "rm -f /tmp/golearn-demo.db*" Enter
Type "./bin/golearn --db /tmp/golearn-demo.db import packs/go-basics.yaml" Enter
Sleep 1s
Type "clear" Enter
Show

# Launch the TUI against the demo DB.
# Note: on a first-run empty DB the TUI opens on Profile Select / register —
# if so, type a handle then Enter before the Home menu appears. Verify the
# exact opening screen when you render and adjust the keystrokes below.
Type "./bin/golearn --db /tmp/golearn-demo.db tui" Enter
Sleep 2s

# Home menu → Start Practice.
Enter
Sleep 1500ms

# Topic Select → pick the first topic.
Enter
Sleep 1500ms

# Session Config → accept defaults, start the session.
Enter
Sleep 2s

# Answer a couple of questions: move selection, toggle, submit, review.
Down
Sleep 800ms
Space
Sleep 600ms
Enter
Sleep 1800ms
Type "e"
Sleep 2500ms
Enter
Sleep 1500ms

Down
Sleep 800ms
Space
Sleep 600ms
Enter
Sleep 2200ms

# Back out to the home menu, then open the stats dashboard.
Escape
Sleep 1s
Escape
Sleep 1s
Down
Down
Sleep 800ms
Enter
Sleep 3s

# Let the final frame breathe.
Sleep 2s
