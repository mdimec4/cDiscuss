package main

const (
	mqSsessionEnd = "session end"
)

type mqMessage struct {
	InstanceID string `json: instance_id`
	Operation  string `json: operation`
	Argument   string `json: argument`
}

type mqMessageCB func(msg mqMessage)

type mqServiceItf interface {
	registerMessageCB(cb mqMessageCB, selfTrigger bool) error
	sendMessage(operation string, argument string) error
	closeMq() error
}
