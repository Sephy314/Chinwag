package structs

// CursorMeta carries cursor-based pagination metadata, matching the envelope
// used by the chat and auth services.
type CursorMeta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}
