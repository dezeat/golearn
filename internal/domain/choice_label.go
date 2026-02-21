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

package domain

// DisplayLabelForIndex converts a zero-based index to a spreadsheet-style
// column label (0→A, 1→B, …, 25→Z, 26→AA). Used by both the CLI and TUI
// to assign display labels to answer choices.
func DisplayLabelForIndex(index int) string {
	if index < 0 {
		return ""
	}

	value := index + 1
	label := ""
	for value > 0 {
		value--
		label = string(rune('A'+(value%26))) + label
		value /= 26
	}
	return label
}
