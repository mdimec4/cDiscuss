package main

import "sync"
import "time"
import "errors"
import "log/slog"

type proofOfWorkConformation struct {
	alreadyUsedTokensMap *sync.Map

	tokenAllowedAge        time.Duratiuon
	minLeadingZerosHardnes uint

	stopWorkerChan             chan bool
	deleteOutdatedTokensTicker *time.Ticker
}

func newProofOfWorkConformation(tokenAllowedAge time.Duratiuon, minLeadingZerosHardnes uint) (*proofOfWorkConformation, error) {
	var powConform proofOfWorkConformation

	if tokenAllowedAge < 1 {
		return nil, errors.New("bad tokenAllowedAge value")
	}

	if minLeadingZerosHardnes < 1 {
		return nil, errors.New("bad minLeadingZerosHardnes value")
	}

	powConform.alreadyUsedTokensMap = &sync.Map{}
	powConform.tokenAllowedAge = tokenAllowedAge
	powConform.minLeadingZerosHardnes = minLeadingZerosHardnes

	powConConform.stopWorkerChan = make(chan bool)
	powConform.deleteOutdatedTokensTicker = time.NewTicker(tokenAllowedAge)

	go powConform.deleteOudatedTokensLoopWorker()

	return &powConform
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

func (powConform *proofOfWorkConformation) deleteOudatedTokens() {
	now := time.Now()

	powConform.alreadyUsedTokensMap.Range(func(key, value any) bool {
		createdTime, ok := value.(time.Time)
		if !ok {
			slog.Error("Map token cleanup is not working, value is not time.Time")
			return false
		}
		if createdTime.Add(powConform.tokenAllowedAge).Before(now) {
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
