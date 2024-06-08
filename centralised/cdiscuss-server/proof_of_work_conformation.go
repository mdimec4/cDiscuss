package main

import "sync"
import "time"
import "strings"
import "strconv"
import "errors"
import "log/slog"

type proofOfWorkConformation struct {
	alreadyUsedTokensMap *sync.Map

	tokenAllowedAge            time.Duration
	deleteOutdatedTokensPeriod time.Duration

	stopWorkerChan             chan bool
	deleteOutdatedTokensTicker *time.Ticker

	databaseService databseServiceItf
}

func newProofOfWorkConformation(databaseService databseServiceItf, tokenAllowedAge time.Duration, deleteOutdatedTokensPeriod time.Duration) (*proofOfWorkConformation, error) {
	var powConform proofOfWorkConformation

	if tokenAllowedAge < 1 {
		return nil, errors.New("bad tokenAllowedAge value")
	}
	if deleteOutdatedTokensPeriod < 1 {
		return nil, errors.New("bad deleteOutdatedTokensPeriod value")
	}

	powConform.alreadyUsedTokensMap = &sync.Map{}
	powConform.databaseService = databaseService
	powConform.tokenAllowedAge = tokenAllowedAge
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

type parsedPowToken struct {
	hardnes           uint32
	username          string
	dtCreatedReported time.Time
	randNum           int64
}

//19:adam:1717855224906:3764117886647529
func sePowToken(token string) (*parsedPowToken, error) {
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

	return &parsedPowToken{hardnes: uint32(hardnes), username: username,
			dtCreatedReported: dtCreatedReported, randNum: randNum},
		nil
}
