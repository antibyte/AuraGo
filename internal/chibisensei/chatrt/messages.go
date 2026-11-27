package chatrt

// PrepareMessages returns a copy of the conversation in the form the trainer used. With the system role
// "merge_into_first_user" a leading system message is removed and its non-empty content is put in front of the
// first user message as system + "\n\n" + user. The input is never changed.
func PrepareMessages(systemRole string, in []Message) []Message {
	out := make([]Message, len(in))
	copy(out, in)
	if systemRole != "merge_into_first_user" || len(out) == 0 || out[0].Role != "system" {
		return out
	}
	system := out[0].Content
	out = out[1:]
	if system == "" {
		return out
	}
	for i := range out {
		if out[i].Role == "user" {
			out[i].Content = system + "\n\n" + out[i].Content
			return out
		}
	}
	return out
}
