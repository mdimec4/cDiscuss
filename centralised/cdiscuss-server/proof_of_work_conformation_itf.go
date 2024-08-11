package main

import (
	"crypto/sha256"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type proofOfWorkConformationItf interface {
	isTokenAceptableStore(token string, requiredHardnes uint, username string) error
}

func isExpired(now time.Time, expiresTime time.Time) bool {
	return !now.Before(expiresTime)
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

	parsedToken := &parsedPowToken{hardnes: uint(hardnes), username: username,
		dtCreatedReported: dtCreatedReported, randNum: randNum}
	return parsedToken, nil
}

func countSha256LeadingZeroBits(sumSha256 [sha256.Size]byte) uint {
	var zeroBitCount uint = 0

	for i := 0; i < sha256.Size; i++ {
		for j := 0; j < 8; j++ {
			if ((sumSha256[i] >> (7 - j)) & 0x01) == 0 {
				zeroBitCount++
			} else {
				return zeroBitCount
			}
		}
	}
	return zeroBitCount
}
