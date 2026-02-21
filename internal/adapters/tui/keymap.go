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
	footerMenuRoot      = "[↑/↓] Navigate   [Enter] Select   [Q] Quit"
	footerRegister      = "[Tab] Next Field   [Enter] Confirm   [Esc] Back"
	footerBackOnly      = "[Esc] Back"
	footerQuiz          = "[↑/↓] Navigate   [Space] Toggle   [Enter] Submit   [Esc] Cancel   [S] Skip"
	footerReview        = "[Enter] Next   [E] Toggle Explanation   [Esc] Back"
	footerSessionConfig = "[↑/↓] Navigate   [←/→] Adjust   [Enter] Start   [Esc] Back"
)
