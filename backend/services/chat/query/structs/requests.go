package structs

type ListMessagesRequest struct {
	RoomID string
	Cursor string `query:"cursor"`
	After  string `query:"after"`
	Limit  int    `query:"limit"`
}

type AdminListMessagesRequest struct {
	Cursor   string `query:"cursor"`
	Limit    int    `query:"limit"`
	RoomID   string `query:"room_id"`
	AuthorID string `query:"author_id"`
	Search   string `query:"q"`
}
