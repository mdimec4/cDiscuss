package main

const (
	mqSessionEnd = "session end"
)

type mqMessage struct {
	InstanceID string `json: instance_id`
	Operation  string `json: operation`
	Argument   string `json: argument`
}

type mqMessageCB func(msg mqMessage)

type mqServiceItf interface {
	registerMessageCB(operation string, cb *mqMessageCB, selfTrigger bool) error
	unregisterMessageCB(operartion string, cb *mqMessageCB) error
	sendMessage(operation string, argument string) error
	closeMq() error
}
