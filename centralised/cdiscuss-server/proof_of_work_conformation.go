package main

import "sync"
import "time"
import "strings"
import "strconv"
import "errors"
import "log/slog"

type proofOfWorkConformation struct {
	alreadyUsedTokensMap *sync.Map

	tokeExpiresAge             time.Duration
	deleteOutdatedTokensPeriod time.Duration

	stopWorkerChan             chan bool
	deleteOutdatedTokensTicker *time.Ticker

	databaseService databseServiceItf
}

func newProofOfWorkConformation(databaseService databseServiceItf, tokeExpiresAge time.Duration, deleteOutdatedTokensPeriod time.Duration) (*proofOfWorkConformation, error) {
	var powConform proofOfWorkConformation

	if tokeExpiresAge < 1 {
		return nil, errors.New("bad tokeExpiresAge value")
	}
	if deleteOutdatedTokensPeriod < 1 {
		return nil, errors.New("bad deleteOutdatedTokensPeriod value")
	}

	powConform.alreadyUsedTokensMap = &sync.Map{}
	powConform.databaseService = databaseService
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

	if powConform.databaseService != nil {
		powConform.databaseService.deletePowTokensThatExpired(now)
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

func isExpired(now time.Time, expiresTime time.Time) bool {
	return !now.Before(expiresTime)
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

	if powConform.databaseService != nil {
		err := powConform.databaseService.createPowToken(token, expiresTime)
		return err
	}
	return nipowConform.databaseServicel
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

	if !tokenFound && !foundInMemory && powConform.databaseService != nil {
		expiresTimePtr, err := powConform.databaseService.getPowToken(token)
		if err != nil {
			return true, err
		}
		expiresTime = *expiresTimePtr
		tokenFound = true
		foundInMemory = false
	}

	isTokenExpired := isExpired(time.Now(), expiresTime)
	if tokenFound && isTokenExpired {
		tokenFound = false

		if foundInMemory {
			powConform.alreadyUsedTokensMap.Delete(token)
		}
		if powConform.databaseService != nil {
			powConform.databaseService.deletePowToken(token)
		}

	}

	if tokenFound && !foundInMemory && !isTokenExpired {
		// cache non expired dbToken
		powConform.alreadyUsedTokensMap.Store(token, expiresTime)
	}

	return tokenFound, nil
}

type parsedPowToken struct {
	hardnes           uint
	username          string
	dtCreatedReported time.Time
	randNum           int64
}

// POW tokens look like 19:adam:1717855224906:3764117886647529
func parsePowToken(token string) (*parsedPowToken, error) {
	const tokenPartCount int = 4

	var tokenParts []string = strings.SplitN(token, ":", tokenPartCount)
	if len(tokenParts) != tokenPartCount {
		slog.Error("POW token wrong parts count")
		return nil, errInvalidPowToken
	}

	hardnesStr := tokenParts[0]
	username := tokenParts[1]
	timestampStr := tokenParts[2]
	randNumStr := tokenParts[3]

	hardnes, err := strconv.ParseUint(hardnesStr, 10, 32)
	if err != nil {
		slog.Error("parse POW hardnes", slog.Any("error", err))
		return nil, errInvalidPowToken
	}
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		slog.Error("parse POW timestamp", slog.Any("error", err))
		return nil, errInvalidPowToken
	}
	dtCreatedReported := time.UnixMilli(timestamp)

	randNum, err := strconv.ParseInt(randNumStr, 10, 64)
	if err != nil {
		slog.Error("parse POW randnum", slog.Any("error", err))
		return nil, errInvalidPowToken
	}

	return &parsedPowToken{hardnes: uint(hardnes), username: username,
			dtCreatedReported: dtCreatedReported, randNum: randNum},
		nil
}

func (powConform *proofOfWorkConformation) isTokenAceptableStore(token string, requiredHardnes uint, username string) (bool, error) {
	var timestampTimeDiff time.Duration = 0

	isTokenUsed, err := powConform.isTokenUsedAndForgetIfExpired(token)
	if err != nil {
		return false, err
	}
	if isTokenUsed {
		return false, nil
	}

	parsedPowToken, err := parsePowToken(token)
	if err != nil {
		return false, err
	}

	if parsedPowToken.hardnes < requiredHardnes {
		return false, errInvalidPowToken
	}

	if parsedPowToken.username != username {
		return false, errInvalidPowToken
	}

	now := time.Now()
	if parsedPowToken.dtCreatedReported.After(now) {
		timestampTimeDiff = parsedPowToken.dtCreatedReported.Since(now)
	} else {
		timestampTimeDiff = parsedPowToken.dtCreatedReported.Until(now)
	}

	if timestampTimeDiff >= powConform.tokeExpiresAge {
		return false, errInvalidPowToken
	}

	// TODO check token, Count sha256 zero bits and compare them to required hardnes

	powConform.storeToken(token, now)
	return true, nil
}
