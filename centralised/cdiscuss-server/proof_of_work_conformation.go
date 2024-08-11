package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type proofOfWorkConformation struct {
	alreadyUsedTokensMap *sync.Map

	tokeExpiresAge             time.Duration
	deleteOutdatedTokensPeriod time.Duration

	stopWorkerChan             chan bool
	deleteOutdatedTokensTicker *time.Ticker

	databaseServicePow databaseServiceProofOfWorkItf
}

func newProofOfWorkConformation(databaseServicePow databaseServiceProofOfWorkItf, tokeExpiresAge time.Duration, deleteOutdatedTokensPeriod time.Duration) (*proofOfWorkConformation, error) {
	var powConform proofOfWorkConformation

	if tokeExpiresAge < 1 {
		return nil, errors.New("bad tokeExpiresAge value")
	}
	if deleteOutdatedTokensPeriod < 1 {
		return nil, errors.New("bad deleteOutdatedTokensPeriod value")
	}

	powConform.alreadyUsedTokensMap = &sync.Map{}
	powConform.databaseServicePow = databaseServicePow
	powConform.tokeExpiresAge = tokeExpiresAge
	powConform.deleteOutdatedTokensPeriod = deleteOutdatedTokensPeriod

	powConform.stopWorkerChan = make(chan bool)
	powConform.deleteOutdatedTokensTicker = time.NewTicker(deleteOutdatedTokensPeriod)

	go powConform.deleteOudatedTokensLoopWorker()

	return &powConform, nil
}

func (powConform *proofOfWorkConformation) deleteOudatedTokensLoopWorker() {
	for {
		select {
		case <-powConform.stopWorkerChan:
			return
		case <-powConform.deleteOutdatedTokensTicker.C:
			powConform.deleteOutdatedTokens()
		}
	}
}

func (powConform *proofOfWorkConformation) deleteOutdatedTokens() {
	now := time.Now()

	if powConform.databaseServicePow != nil {
		powConform.databaseServicePow.deletePowTokensThatExpired(now)
	}

	powConform.alreadyUsedTokensMap.Range(func(key, value any) bool {
		expiresTime, ok := value.(time.Time)
		if !ok {
			slog.Error("Map token cleanup is not working, value is not time.Time")
			return false
		}
		if isExpired(now, expiresTime) {
			powConform.alreadyUsedTokensMap.Delete(key)
		}
		return true
	})
}

func (powConform *proofOfWorkConformation) stop() {
	powConform.deleteOutdatedTokensTicker.Stop()

	select {
	case powConform.stopWorkerChan <- true:
	default:
		slog.Error("can't stop proofOfWorkConformation instance")
	}

	powConform.alreadyUsedTokensMap.Range(func(key, value any) bool {
		powConform.alreadyUsedTokensMap.Delete(key)
		return true
	})
}

func (powConform *proofOfWorkConformation) storeToken(token string, now time.Time) error {
	expiresTime := now.Add(powConform.tokeExpiresAge)

	powConform.alreadyUsedTokensMap.Store(token, expiresTime)

	if powConform.databaseServicePow != nil {
		err := powConform.databaseServicePow.createPowToken(token, expiresTime)
		return err
	}
	return nil
}

func (powConform *proofOfWorkConformation) isTokenUsedAndForgetIfExpired(token string) (bool, error) {
	var (
		expiresTime   time.Time
		foundInMemory bool = false
		tokenFound    bool = false
	)

	expiresTimeAny, ok := powConform.alreadyUsedTokensMap.Load(token)
	if ok {
		expiresTime, ok = expiresTimeAny.(time.Time)
		if !ok {
			return true, fmt.Errorf("Internal error: POW map value is not time.Time")
		}
		tokenFound = true
		foundInMemory = true
	}

	if !tokenFound && !foundInMemory && powConform.databaseServicePow != nil {
		expiresTimePtr, err := powConform.databaseServicePow.getPowToken(token)
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
		powConform.alreadyUsedTokensMap.Store(token, expiresTime)
	}

	if tokenFound && isTokenExpired {
		tokenFound = false

		if foundInMemory {
			powConform.alreadyUsedTokensMap.Delete(token)
		}
		if powConform.databaseServicePow != nil {
			powConform.databaseServicePow.deletePowToken(token)
		}

	}

	return tokenFound, nil
}

func (powConform *proofOfWorkConformation) isTokenAceptableStore(token string, requiredHardnes uint, username string) error {
	isTokenUsed, err := powConform.isTokenUsedAndForgetIfExpired(token)
	if err != nil {
		return err
	}
	if isTokenUsed {
		return errUsedPowToken
	}

	parsedPowToken, err := parsePowToken(token)
	if err != nil {
		return err
	}

	if parsedPowToken.hardnes < requiredHardnes {
		return errInvalidPowToken
	}

	if parsedPowToken.username != username {
		return errInvalidPowToken
	}

	now := time.Now()
	timestampTimeDiff := parsedPowToken.dtCreatedReported.Sub(now).Abs()
	if timestampTimeDiff >= powConform.tokeExpiresAge {
		return errInvalidPowToken
	}

	var sumSha256 [sha256.Size]byte = sha256.Sum256([]byte(token))
	hashLeadingZeroBitCount := countSha256LeadingZeroBits(sumSha256)
	if hashLeadingZeroBitCount < requiredHardnes {
		return errInvalidPowToken
	}

	powConform.storeToken(token, now)
	return nil
}
