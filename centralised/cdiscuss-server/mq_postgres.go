package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"log/slog"
	"sync"
	"time"
)

type mqPostgres struct {
	callbackMapMutex *sync.RWMutex
	callbacksMap     map[string]map[mqMessageCbItf]bool

	listener   *pq.Listener
	db         *sql.DB
	instanceID string
}

func newMqPostgres(connectionString string, instanceID string) (*mqPostgres, error) {
	if connectionString == "" || instanceID == "" {
		return nil, fmt.Errorf("MQ postgres: Bad input parameters")
	}
	mqPostgres := &mqPostgres{callbackMapMutex: &sync.RWMutex{}, callbacksMap: make(map[string]map[mqMessageCbItf]bool), instanceID: instanceID}

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

func (mqPostgres *mqPostgres) registerMessageCB(operation string, cbObj mqMessageCbItf, selfTrigger bool) error {
	if cbObj == nil {
		return errors.New("Postgres MQ nil cbObj")
	}
	if operation == "" {
		return errors.New("Postgres MQ: can't register empty operation string")
	}

	mqPostgres.callbackMapMutex.Lock()

	callbacksSet, ok := mqPostgres.callbacksMap[operation]
	if !ok {
		callbacksSet = make(map[mqMessageCbItf]bool)
		mqPostgres.callbacksMap[operation] = callbacksSet
	}
	callbacksSet[cbObj] = selfTrigger

	mqPostgres.callbackMapMutex.Unlock()

	return nil
}

func (mqPostgres *mqPostgres) unregisterMessageCB(operation string, cbObj mqMessageCbItf) error {
	if cbObj == nil {
		return errors.New("Postgres MQ nil cbObj")
	}
	if operation == "" {
		return errors.New("Postgres MQ: can't unregister empty operation string")
	}

	mqPostgres.callbackMapMutex.Lock()
	defer mqPostgres.callbackMapMutex.Unlock()

	callbacksSet, ok := mqPostgres.callbacksMap[operation]
	if !ok {
		return nil
	}
	delete(callbacksSet, cbObj)
	if len(callbacksSet) == 0 {
		delete(mqPostgres.callbacksMap, operation)
	}

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

		mqPostgres.callbackMapMutex.RLock()

		callbacksSet, ok := mqPostgres.callbacksMap[msg.Operation]
		if !ok {
			mqPostgres.callbackMapMutex.RUnlock()
			continue
		}

		for cbObj, selfTrigger := range callbacksSet {
			if selfTrigger || mqPostgres.instanceID != msg.InstanceID {
				go cbObj.onMessage(msg)
			}
		}

		mqPostgres.callbackMapMutex.RUnlock()

	}
}
