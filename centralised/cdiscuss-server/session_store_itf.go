package main

import (
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
