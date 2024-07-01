package main

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"
)

const seassionTokenRandomPartLen int = 32

func generateNewSessionToken() string {
	var randomPart string = generateRandomStr(seassionTokenRandomPartLen)

	var nowMicro int64 = time.Now().UnixMicro()
	var nowMicroHexStr string = strconv.FormatInt(nowMicro, 16)

	return randomPart + nowMicroHexStr
}

func calculateTokenHash(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("Empty token")
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token))), nil
}

type sessionStoreItf interface {
	newSession(user *user) (string, error)
	getUser(token string) (*user, error)
}
