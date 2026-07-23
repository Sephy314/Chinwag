package domain

type MessageType int16

const (
	MessageTypeTEXT   MessageType = 0
	MessageTypeSYSTEM MessageType = 1
	MessageTypeIMAGE  MessageType = 2
	MessageTypeFILE   MessageType = 3
	MessageTypeNOTICE MessageType = 4
)

var _ = []MessageType{MessageTypeSYSTEM, MessageTypeIMAGE, MessageTypeFILE, MessageTypeNOTICE}
