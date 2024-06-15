package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type sessionStore struct {
	alreadyUsedTokensMap *sync.Map

	tokeExpiresAge             time.Duration
	deleteOutdatedTokensPeriod time.Duration

	stopWorkerChan             chan bool
	deleteOutdatedTokensTicker *time.Ticker

	databaseService databseServiceItf
}

func newSessionStore(databaseService databseServiceItf, tokeExpiresAge time.Duration, deleteOutdatedTokensPeriod time.Duration) (*sessionStore, error) {
	var session sessionStore

	if tokeExpiresAge < 1 {
		return nil, errors.New("bad tokeExpiresAge value")
	}
	if deleteOutdatedTokensPeriod < 1 {
		return nil, errors.New("bad deleteOutdatedTokensPeriod value")
	}

	session.alreadyUsedTokensMap = &sync.Map{}
	session.databaseService = databaseService
	session.tokeExpiresAge = tokeExpiresAge
	session.deleteOutdatedTokensPeriod = deleteOutdatedTokensPeriod

	session.stopWorkerChan = make(chan bool)
	session.deleteOutdatedTokensTicker = time.NewTicker(deleteOutdatedTokensPeriod)

	go session.deleteOudatedTokensLoopWorker()

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

	if session.databaseService != nil {
		session.databaseService.deleteSeassionTokensThatExpired(now)
	}

	session.alreadyUsedTokensMap.Range(func(key, value any) bool {
		expiresTime, ok := value.(time.Time)
		if !ok {
			slog.Error("Map token cleanup is not working, value is not time.Time")
			return false
		}
		if isExpired(now, expiresTime) {
			session.alreadyUsedTokensMap.Delete(key)
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

	session.alreadyUsedTokensMap.Range(func(key, value any) bool {
		session.alreadyUsedTokensMap.Delete(key)
		return true
	})
}

func (session *sessionStore) storeToken(token string, now time.Time) error {
	expiresTime := now.Add(session.tokeExpiresAge)

	session.alreadyUsedTokensMap.Store(token, expiresTime)

	if session.databaseService != nil {
		err := session.databaseService.createSeassionToken(token, expiresTime)
		return err
	}
	return nil
}

func (session *sessionStore) isTokenUsedAndForgetIfExpired(token string) (bool, error) {
	var (
		expiresTime   time.Time
		foundInMemory bool = false
		tokenFound    bool = false
	)

	expiresTimeAny, ok := session.alreadyUsedTokensMap.Load(token)
	if ok {
		expiresTime, ok = expiresTimeAny.(time.Time)
		if !ok {
			return true, fmt.Errorf("Internal error: POW map value is not time.Time")
		}
		tokenFound = true
		foundInMemory = true
	}

	if !tokenFound && !foundInMemory && session.databaseService != nil {
		expiresTimePtr, err := session.databaseService.getSeassionToken(token)
		if err != nil {
			return true, err
		}
		expiresTime = *expiresTimePtr
		tokenFound = true
		foundInMemory = false
	}

	if !tokenFound {
		return false, nil
	}

	isTokenExpired := isExpired(time.Now(), expiresTime)

	if tokenFound && !foundInMemory && !isTokenExpired {
		// cache non expired dbToken
		session.alreadyUsedTokensMap.Store(token, expiresTime)
	}

	if tokenFound && isTokenExpired {
		tokenFound = false

		if foundInMemory {
			session.alreadyUsedTokensMap.Delete(token)
		}
		if session.databaseService != nil {
			session.databaseService.deleteSeassionToken(token)
		}

	}

	return tokenFound, nil
}

func (session *sessionStore) isTokenAceptableStore(token string, requiredHardnes uint, username string) (bool, error) {
	isTokenUsed, err := session.isTokenUsedAndForgetIfExpired(token)
	if err != nil {
		return false, err
	}
	if isTokenUsed {
		return false, nil
	}

	parsedSeassionToken, err := parseSeassionToken(token)
	if err != nil {
		return false, err
	}

	if parsedSeassionToken.hardnes < requiredHardnes {
		return false, errUserSessionIsNotValid
	}

	if parsedSeassionToken.username != username {
		return false, errUserSessionIsNotValid
	}

	now := time.Now()
	timestampTimeDiff := parsedSeassionToken.dtCreatedReported.Sub(now).Abs()
	if timestampTimeDiff >= session.tokeExpiresAge {
		return false, errUserSessionIsNotValid
	}

	var sumSha256 [sha256.Size]byte = sha256.Sum256([]byte(token))
	hashLeadingZeroBitCount := countSha256LeadingZeroBits(sumSha256)
	if hashLeadingZeroBitCount < requiredHardnes {
		return false, errUserSessionIsNotValid
	}

	session.storeToken(token, now)
	return true, nil
}
