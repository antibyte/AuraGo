package chibisensei

// Issue is one validation finding the API reports with a 422. B1 only needs this type for the
// server error writer; B2 extends this file with the remaining API types.
type Issue struct {
	Field      string `json:"field,omitempty"`
	Line       int    `json:"line,omitempty"`
	MessageKey string `json:"message_key"`
	Message    string `json:"message"`
}
