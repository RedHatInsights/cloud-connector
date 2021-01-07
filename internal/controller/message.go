package controller

import (
	"github.com/google/uuid"
)

type Message struct {
	MessageID uuid.UUID
	Recipient string
	RouteList []string
	Payload   interface{}
	Directive string
}

type ResponseMessage struct {
	AccountNumber string      `json:"account"`
	Sender        string      `json:"sender"`
	MessageType   string      `json:"message_type"`
	MessageID     string      `json:"message_id"`
	Payload       interface{} `json:"payload"`
	Code          int         `json:"code"`
	InResponseTo  string      `json:"in_response_to"`
	Serial        int         `json:"serial"`
}
