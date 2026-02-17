package tui

const (
	keyQuit    = "q"
	keyBack    = "esc"
	keyEnter   = "enter"
	keyUp      = "up"
	keyDown    = "down"
	keyLeft    = "left"
	keyRight   = "right"
	keyViUp    = "k"
	keyViDown  = "j"
	keyToggle  = " "
	keySkip    = "s"
	keyReview  = "r"
	keyExplain = "e"
)

func isUpNav(key string) bool {
	return key == keyUp || key == keyViUp
}

func isDownNav(key string) bool {
	return key == keyDown || key == keyViDown
}

func isAdjustUp(key string) bool {
	return key == keyRight || key == keyViUp
}

func isAdjustDown(key string) bool {
	return key == keyLeft || key == keyViDown
}

func isBackKey(key string) bool {
	return key == keyBack
}

func isQuitKey(key string) bool {
	return key == keyQuit
}

func isEnterKey(key string) bool {
	return key == keyEnter
}

func isToggleKey(key string) bool {
	return key == keyToggle
}

func isSkipKey(key string) bool {
	return key == keySkip
}

func isExplainKey(key string) bool {
	return key == keyExplain
}

func isReviewKey(key string) bool {
	return key == keyReview
}

const (
	footerMenuTop       = "[↑/↓] Navigate   [Enter] Select   [Esc] Back   [Q] Quit"
	footerMenuTopReview = "[↑/↓] Navigate   [Enter] Select   [R] Review   [Esc] Back   [Q] Quit"
	footerMenuSub       = "[↑/↓] Navigate   [Enter] Select   [Esc] Back"
	footerMenuSubReview = "[↑/↓] Navigate   [Enter] Select   [R] Review   [Esc] Back"
	footerQuiz          = "[↑/↓] Navigate   [Space] Toggle   [Enter] Submit   [Esc] Cancel   [S] Skip"
	footerReview        = "[Enter] Next   [E] Toggle Explanation   [Esc] Back"
	footerSessionConfig = "[↑/↓] Navigate   [←/→] Adjust   [Enter] Start   [Esc] Back"
)
