package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/lib/pq"
	"log/slog"
	"time"
)

type mqPostgres struct {
	callbacks  []mqMessageCB
	listener   *pq.Listener
	db         *sql.DB
	instanceID string
}

func newMqPostgres(connectionString string, instanceID string) (*mqPostgres, error) {
	if connectionString == "" || instanceID == "" {
		return nil, fmt.Errorf("MQ postgres: Bad input parameters")
	}
	mqPostgres := &mqPostgres{instanceID: instanceID, callbacks: make([]mqMessageCB, 0, 1)}

	// Listen for notifications
	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	minReconn := 10 * time.Second
	maxReconn := time.Minute
	mqPostgres.listener = pq.NewListener(connectionString, minReconn, maxReconn, reportProblem)
	err := mqPostgres.listener.Listen("cDiscuss")
	if err != nil {
		return nil, fmt.Errorf("Failed to create MQ postgres listener: %w", err)
	}

	go mqPostgres.listen()

	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to MQ postgres: %w", err)
	}
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("Failed to ping MQ postgres: %w", err)
	}
	mqPostgres.db = db

	return mqPostgres, nil
}

func (mqPostgres *mqPostgres) registerMessageCB(cb mqMessageCB) {
	mqPostgres.callbacks = append(mqPostgres.callbacks, cb)
}

func (mqPostgres *mqPostgres) sendMessage(operation string, argument string) error {
	msg := mqMessage{InstanceID: mqPostgres.instanceID, Operation: operation, Argument: argument}
	msgJsonBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("MQ postgres marshaling fail: %w", err)
	}
	_, err = mqPostgres.db.Exec("NOTIFY cDiscuss $1", string(msgJsonBytes))
	if err != nil {
		return fmt.Errorf("MQ postgres send fail: %w", err)
	}
	return nil
}

func (mqPostgres *mqPostgres) closeMq() error {
	return mqPostgres.listener.Close()
}

func (mqPostgres *mqPostgres) listen() {
	for {
		// process all available work before waiting for notifications
		n, ok := <-mqPostgres.listener.Notify
		if !ok {
			return
		}
		if n == nil {
			continue
		}
		var msg mqMessage
		err := json.Unmarshal([]byte(n.Extra), &msg)
		if err == nil {
			slog.Error("MqPostgres JSON unmarshal problem", slog.Any("error", err))
			continue
		}

		for _, cb := range mqPostgres.callbacks {
			go cb(msg)
		}
	}
}
