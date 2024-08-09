package main

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type sessionDataContainer struct {
	user        *user
	expiresTime time.Time
}

type sessionStore struct {
	sessionTokensMap *sync.Map

	tokenExpiresAge            time.Duration
	deleteOutdatedTokensPeriod time.Duration

	stopWorkerChan             chan bool
	deleteOutdatedTokensTicker *time.Ticker

	databaseServiceSession databaseServiceSessionItf
	mqService              mqServiceItf
	mqServiceSessionEndCB  mqMessageCB
}

func newSessionStore(databaseServiceSession databaseServiceSessionItf, mqService mqServiceItf, tokenExpiresAge time.Duration, deleteOutdatedTokensPeriod time.Duration) (*sessionStore, error) {
	var session sessionStore

	if tokenExpiresAge < 1 {
		return nil, errors.New("bad tokenExpiresAge value")
	}
	if deleteOutdatedTokensPeriod < 1 {
		return nil, errors.New("bad deleteOutdatedTokensPeriod value")
	}

	session.sessionTokensMap = &sync.Map{}
	session.databaseServiceSession = databaseServiceSession
	session.mqService = mqService
	session.tokenExpiresAge = tokenExpiresAge
	session.deleteOutdatedTokensPeriod = deleteOutdatedTokensPeriod

	session.stopWorkerChan = make(chan bool)
	session.deleteOutdatedTokensTicker = time.NewTicker(deleteOutdatedTokensPeriod)

	go session.deleteOudatedTokensLoopWorker()

	if session.mqService != nil {
		cb := func(msg mqMessage) {
			tokenHash := msg.Argument
			session.sessionTokensMap.Delete(tokenHash)
		}
		_ = &cb == &cb

		session.mqServiceSessionEndCB = cb
		session.mqService.registerMessageCB(mqSessionEnd, cb, false)
	}

	return &session, nil
}

func (session *sessionStore) deleteOudatedTokensLoopWorker() {
	for {
		select {
		case <-session.stopWorkerChan:
			return
		case <-session.deleteOutdatedTokensTicker.C:
			session.deleteOutdatedTokens()
		}
	}
}

func (session *sessionStore) deleteOutdatedTokens() {
	now := time.Now()

	if session.databaseServiceSession != nil {
		err := session.databaseServiceSession.deleteSessionsThatExpired(now)
		if err != nil {
			slog.Error("Deleting outdated seassion from DB:", slog.Any("error", err))
		}
	}

	session.sessionTokensMap.Range(func(key, value any) bool {
		sessionData, ok := value.(sessionDataContainer)
		if !ok {
			slog.Error("Map token cleanup is not working, value is not sessionDataContainer")
			return false
		}
		if isExpired(now, sessionData.expiresTime) {
			session.sessionTokensMap.Delete(key)
		}
		return true
	})
}

func (session *sessionStore) stop() {
	session.deleteOutdatedTokensTicker.Stop()

	select {
	case session.stopWorkerChan <- true:
	default:
		slog.Error("can't stop sessionStore instance")
	}

	session.sessionTokensMap.Range(func(key, value any) bool {
		session.sessionTokensMap.Delete(key)
		return true
	})
	if session.mqService != nil && session.mqServiceSessionEndCB != nil {
		if err := session.mqService.unregisterMessageCB(mqSessionEnd, session.mqServiceSessionEndCB); err != nil {
			slog.Error("sessionStore unregisterinf MQ CB error:", slog.Any("error", err))
		}
	}
}

func (session *sessionStore) storeSession(tokenHash string, user *user, now time.Time) error {
	if tokenHash == "" {
		return fmt.Errorf("tokenHash should not be empty string")
	}
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	expiresTime := now.Add(session.tokenExpiresAge)

	sessionData := sessionDataContainer{user: user, expiresTime: expiresTime}

	session.sessionTokensMap.Store(tokenHash, sessionData)

	if session.databaseServiceSession != nil {
		err := session.databaseServiceSession.createSession(tokenHash, user.id, expiresTime)
		return err
	}
	return nil
}

func (session *sessionStore) getUserOrForgetIfExpired(tokenHash string) (*user, error) {
	var (
		sessionData   sessionDataContainer
		foundInMemory bool = false
		tokenFound    bool = false
	)
	if tokenHash == "" {
		return nil, fmt.Errorf("Empty tokenHash")
	}

	sessionDataAny, ok := session.sessionTokensMap.Load(tokenHash)
	if ok {
		sessionData, ok = sessionDataAny.(sessionDataContainer)
		if !ok {
			return nil, fmt.Errorf("Internal error: session map value is not sessionDataContainer")
		}
		tokenFound = true
		foundInMemory = true
	}

	if !tokenFound && !foundInMemory && session.databaseServiceSession != nil {
		expiresTimePtr, userPtr, err := session.databaseServiceSession.getSession(tokenHash)
		if err != nil {
			return nil, err
		}
		sessionData = sessionDataContainer{user: userPtr, expiresTime: *expiresTimePtr}
		tokenFound = true
		foundInMemory = false
	}

	if !tokenFound {
		return nil, nil
	}

	isTokenExpired := isExpired(time.Now(), sessionData.expiresTime)

	if tokenFound && !foundInMemory && !isTokenExpired {
		// cache non expired dbToken
		session.sessionTokensMap.Store(tokenHash, sessionData)
	}

	if tokenFound && isTokenExpired {
		tokenFound = false

		if foundInMemory {
			session.sessionTokensMap.Delete(tokenHash)
		}
		if session.databaseServiceSession != nil {
			err := session.databaseServiceSession.deleteSession(tokenHash)
			if err != nil {
				slog.Error("Deleting outdated seassion from DB:", slog.Any("error", err))
			}
		}

	}

	return sessionData.user, nil
}

func (session *sessionStore) newSession(user *user) (string, error) {
	if user == nil {
		return "", fmt.Errorf("user is nil")
	}
	token := generateNewSessionToken()
	now := time.Now()

	tokenHash, err := calculateTokenHash(token)
	if err != nil {
		return "", err
	}

	session.storeSession(tokenHash, user, now)
	return token, nil
}

func (session *sessionStore) getUser(token string) (*user, error) {
	if token == "" {
		return nil, fmt.Errorf("Empty token")
	}

	tokenHash, err := calculateTokenHash(token)
	if err != nil {
		return nil, err
	}

	user, err := session.getUserOrForgetIfExpired(tokenHash)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (session *sessionStore) logout(token string) error {
	tokenHash, err := calculateTokenHash(token)
	if err != nil {
		return err
	}

	if session.databaseServiceSession != nil {
		err := session.databaseServiceSession.deleteSession(tokenHash)
		if err != nil {
			return fmt.Errorf("Logout faild because of database: %w", err)
		}
	}
	session.sessionTokensMap.Delete(tokenHash)

	if session.mqService != nil {
		err = session.mqService.sendMessage(mqSessionEnd, tokenHash)
		if err != nil {
			slog.Error("sessio store: informing logout to other instancs failed", slog.Any("error", err))
		}
	}
	return nil
}
