package main

const (
	mqSessionEnd = "session end"
)

type mqMessage struct {
	InstanceID string `json: instance_id`
	Operation  string `json: operation`
	Argument   string `json: argument`
}

type mqMessageCbItf interface {
	onMessage(msg mqMessage)
}

type mqServiceItf interface {
	registerMessageCB(operation string, cbObj mqMessageCbItf, selfTrigger bool) error
	unregisterMessageCB(operartion string, cbObj mqMessageCbItf) error
	sendMessage(operation string, argument string) error
	closeMq() error
}
