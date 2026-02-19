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
