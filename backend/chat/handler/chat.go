package handler

import (
	"context"
	"net/http"

	"github.com/Sephy314/chinwag/chat/service"
	"github.com/Sephy314/chinwag/chat/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type ChatHandlerInterface interface {
	Health(c *echo.Context) error
	CreateMessage(c *echo.Context) error
	GetMessage(c *echo.Context) error
	ListMessages(c *echo.Context) error
	UpdateMessage(c *echo.Context) error
	DeleteMessage(c *echo.Context) error
}

type ChatHandler struct {
	svc service.ChatServiceInterface
}

func NewChatHandler(svc service.ChatServiceInterface) *ChatHandler {
	return &ChatHandler{svc: svc}
}

// Health godoc
// @Summary      Health check
// @Description  Check the health status of the chat service
// @Tags         chat
// @Produce      json
// @Success      200 {object} response.Response[any]
// @Router       /chat/health [get]
func (h *ChatHandler) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

// CreateMessage godoc
// @Summary      Create a chat message
// @Description  Send a new message to a room. The authenticated user must be a member of the room. Message type: 0=TEXT, 1=SYSTEM, 2=IMAGE, 3=FILE, 4=NOTICE. On success, the message is broadcast via WebSocket to all connected clients in the room.
// @Tags         chat-message
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        roomId  path  string                          true "Room UUID" 
// @Param        request body  structs.CreateMessageRequest     true "Message content" 
// @Success      201     {object} response.Response[structs.MessageResponse] "Message created"
// @Failure      400     {object} response.Response[any]        "Invalid request body or UUID format"
// @Failure      401     {object} response.Response[any]        "Unauthorized"
// @Failure      403     {object} response.Response[any]        "Not a member of this room"
// @Router       /chat/rooms/{roomId}/messages [post]
func (h *ChatHandler) CreateMessage(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}
	uid, _ := uuid.Parse(*userId)

	var req structs.CreateMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	ctx := c.Request().Context()
	ctx = context.WithValue(ctx, "authorId", uid)

	msg, err := h.svc.CreateMessage(ctx, roomId, req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusCreated, response.Created(msg))
}

// GetMessage godoc
// @Summary      Get a chat message
// @Description  Retrieve a single chat message by its UUID.
// @Tags         chat-message
// @Produce      json
// @Security     BearerAuth
// @Param        roomId    path string true "Room UUID" 
// @Param        messageId path string true "Message UUID" 
// @Success      200 {object} response.Response[structs.MessageResponse] "Message found"
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Failure      404 {object} response.Response[any] "Message not found"
// @Router       /chat/rooms/{roomId}/messages/{messageId} [get]
func (h *ChatHandler) GetMessage(c *echo.Context) error {
	messageId, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid message id"))
	}

	msg, err := h.svc.GetMessage(c.Request().Context(), messageId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(msg))
}

// ListMessages godoc
// @Summary      List chat messages
// @Description  Retrieve messages in a room with cursor-based pagination. Sorted by created_at DESC. Provide the cursor from the previous response to fetch the next page. Default limit is 50, max 200.
// @Tags         chat-message
// @Produce      json
// @Security     BearerAuth
// @Param        roomId  path    string true  "Room UUID" 
// @Param        cursor query   string false "Cursor from previous response (base64 encoded JSON)" 
// @Param        limit  query   int    false "Number of messages per page (default 50, max 200)" 
// @Success      200    {object} response.Response[[]structs.MessageResponse] "Paginated messages with cursor meta"
// @Failure      400    {object} response.Response[any] "Invalid UUID format"
// @Router       /chat/rooms/{roomId}/messages [get]
func (h *ChatHandler) ListMessages(c *echo.Context) error {
	roomId := c.Param("roomId")
	if roomId == "" {
		return c.JSON(http.StatusBadRequest, response.Error("room id is required"))
	}

	req := structs.ListMessagesRequest{
		RoomID: roomId,
		Cursor: c.QueryParam("cursor"),
		Limit:  50,
	}

	msgs, meta, err := h.svc.ListMessages(c.Request().Context(), req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	var metaResp *structs.CursorMeta
	if meta != nil {
		metaResp = &structs.CursorMeta{
			NextCursor: meta.NextCursor,
			HasMore:    meta.HasMore,
		}
	}

	resp := response.OK(msgs)
	resp.Meta = metaResp
	return c.JSON(http.StatusOK, resp)
}

// UpdateMessage godoc
// @Summary      Update a chat message
// @Description  Update the content of a chat message. Only the original author can update their message. On success, the update is broadcast via WebSocket.
// @Tags         chat-message
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        roomId    path  string                        true "Room UUID" 
// @Param        messageId path  string                        true "Message UUID" 
// @Param        request   body  structs.UpdateMessageRequest  true "Fields to update" 
// @Success      200       {object} response.Response[structs.MessageResponse] "Message updated"
// @Failure      400       {object} response.Response[any] "Invalid request body or UUID format"
// @Failure      401       {object} response.Response[any] "Unauthorized"
// @Failure      403       {object} response.Response[any] "Not the author of this message"
// @Failure      404       {object} response.Response[any] "Message not found"
// @Router       /chat/rooms/{roomId}/messages/{messageId} [put]
func (h *ChatHandler) UpdateMessage(c *echo.Context) error {
	messageId, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid message id"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}
	uid, _ := uuid.Parse(*userId)

	var req structs.UpdateMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	msg, err := h.svc.UpdateMessage(c.Request().Context(), messageId, uid, req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(msg))
}

// DeleteMessage godoc
// @Summary      Delete a chat message
// @Description  Soft-delete a chat message by UUID. Only the original author can delete their message. On success, the deletion is broadcast via WebSocket.
// @Tags         chat-message
// @Produce      json
// @Security     BearerAuth
// @Param        roomId    path string true "Room UUID" 
// @Param        messageId path string true "Message UUID" 
// @Success      200 {object} response.Response[any] "Message deleted"
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Failure      401 {object} response.Response[any] "Unauthorized"
// @Failure      403 {object} response.Response[any] "Not the author of this message"
// @Failure      404 {object} response.Response[any] "Message not found"
// @Router       /chat/rooms/{roomId}/messages/{messageId} [delete]
func (h *ChatHandler) DeleteMessage(c *echo.Context) error {
	messageId, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid message id"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}
	uid, _ := uuid.Parse(*userId)

	if err := h.svc.DeleteMessage(c.Request().Context(), messageId, uid); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK[any](nil))
}

var _ ChatHandlerInterface = (*ChatHandler)(nil)