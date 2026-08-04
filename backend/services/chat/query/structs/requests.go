package structs

type ListMessagesRequest struct {
	RoomID string
	Cursor string `query:"cursor"`
	After  string `query:"after"`
	Limit  int    `query:"limit"`
}
