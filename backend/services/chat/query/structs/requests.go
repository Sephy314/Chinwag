package structs

type ListMessagesRequest struct {
	RoomID string
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit"`
}
