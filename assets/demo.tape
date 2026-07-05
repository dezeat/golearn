# golearn animated hero — charmbracelet/vhs script.
#
# Renders assets/hero.gif: the ASCII-logo start screen, a short practice
# session with quiz-show review + explanation, and the stats view.
# Deterministic and re-runnable — it seeds a throwaway demo DB at
# /tmp/golearn-demo.db from a bundled pack, so the recording never depends on
# your real ~/.golearn data or on wall-clock state.
#
# Prerequisites: vhs (github.com/charmbracelet/vhs), plus ttyd and ffmpeg on
# PATH. Run from the repo root so ./bin/golearn resolves:
#   make build && rm -f /tmp/golearn-demo.db* && vhs assets/demo.tape

Output assets/hero.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 700
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set PlaybackSpeed 1.0

# Seed a deterministic, throwaway demo database from a bundled pack.
Hide
Type "rm -f /tmp/golearn-demo.db*" Enter
Type "./bin/golearn --db /tmp/golearn-demo.db import packs/go-basics.yaml" Enter
Sleep 1s
Type "clear" Enter
Show

# Launch the TUI against the demo DB.
Type "./bin/golearn --db /tmp/golearn-demo.db tui" Enter
# Hold on the ASCII-logo start screen — this is the branded opener.
Sleep 4s

# Enter the app → Home menu.
Enter
Sleep 1800ms

# Home → Practice.
Enter
Sleep 1500ms

# Topic Select → pick Go Basics.
Enter
Sleep 1500ms

# Session Config → shorten to a quick 3-question run, then start.
Up
Sleep 500ms
Left Left Left Left Left Left Left
Sleep 1200ms
Enter
Sleep 1800ms

# Question 1 → choose an answer, submit, read the quiz-show review + explanation.
Down
Sleep 700ms
Space
Sleep 600ms
Enter
Sleep 1600ms
Type "e"
Sleep 2600ms
Enter
Sleep 1400ms

# Question 2 → answer and submit.
Down Down
Sleep 700ms
Space
Sleep 600ms
Enter
Sleep 1800ms
Enter
Sleep 1200ms

# Question 3 → answer and submit → session summary.
Space
Sleep 600ms
Enter
Sleep 1800ms
Enter
Sleep 1600ms

# Summary → open the per-pack stats view.
Down
Sleep 700ms
Enter
Sleep 3200ms

# Let the final frame breathe.
Sleep 1800ms
