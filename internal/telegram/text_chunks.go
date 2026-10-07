package telegram

// Telegram limits plain messages in UTF-16 code units. Keep Unicode intact.
func telegramTextChunks(text string) []string {
	var chunks []string
	start, units := 0, 0
	for offset, r := range text {
		width := 1
		if r > 0xffff {
			width = 2
		}
		if units+width > 4096 {
			chunks = append(chunks, text[start:offset])
			start = offset
			units = 0
		}
		units += width
	}
	if start < len(text) {
		chunks = append(chunks, text[start:])
	}
	return chunks
}
