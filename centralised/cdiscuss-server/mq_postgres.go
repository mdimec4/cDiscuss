package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"log/slog"
	"time"
)

type mqPostgres struct {
	callbacks  []mqPostgresCbContainer
	listener   *pq.Listener
	db         *sql.DB
	instanceID string
}

type mqPostgresCbContainer struct {
	cb          mqMessageCB
	selfTrigger bool
}

func newMqPostgres(connectionString string, instanceID string) (*mqPostgres, error) {
	if connectionString == "" || instanceID == "" {
		return nil, fmt.Errorf("MQ postgres: Bad input parameters")
	}
	mqPostgres := &mqPostgres{instanceID: instanceID, callbacks: make([]mqPostgresCbContainer, 0, 1)}

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

func (mqPostgres *mqPostgres) registerMessageCB(cb mqMessageCB, selfTrigger bool) error {
	if cb == nil {
		return errors.New("Postgres MQ nil CB")
	}
	mqPostgres.callbacks = append(mqPostgres.callbacks, mqPostgresCbContainer{cb: cb, selfTrigger: selfTrigger})
	return nil
}

func (mqPostgres *mqPostgres) sendMessage(operation string, argument string) error {
	msg := mqMessage{InstanceID: mqPostgres.instanceID, Operation: operation, Argument: argument}
	msgJsonBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("MQ postgres marshaling fail: %w", err)
	}
	_, err = mqPostgres.db.Exec("SELECT pg_notify('cDiscuss', $1)", string(msgJsonBytes))
	if err != nil {
		return fmt.Errorf("MQ postgres send fail: %w", err)
	}
	return nil
}

func (mqPostgres *mqPostgres) closeMq() error {
	err1 := mqPostgres.db.Close()
	err2 := mqPostgres.listener.Close()
	if err2 != nil {
		if err1 != nil {
			slog.Error("Posgres MQ db closing error: ", slog.Any("error", err1))
		}
		return err2
	}
	return err1
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
		if err != nil {
			slog.Error("MqPostgres JSON unmarshal problem", slog.Any("error", err))
			continue
		}

		for _, cbContainer := range mqPostgres.callbacks {
			if cbContainer.selfTrigger || mqPostgres.instanceID != msg.InstanceID {
				go cbContainer.cb(msg)
			}
		}
	}
}
