package main

import (
	"testing"
	"time"
)

func TestParsePowToken(t *testing.T) {
	const token = "19:adam:1717855224906:3764117886647529"

	parsedPowToken, err := parsePowToken(token)
	if err != nil {
		t.Fatalf("POW token parsing error: %v", err)
	}

	if parsedPowToken.hardnes != 19 {
		t.Errorf("Wrong hardnes: %d", parsedPowToken.hardnes)
	}
	if parsedPowToken.username != "adam" {
		t.Errorf("Wrong username: %s", parsedPowToken.username)
	}
	if parsedPowToken.dtCreatedReported != time.UnixMilli(1717855224906) {
		t.Errorf("Wrong timestamp: %v", parsedPowToken.dtCreatedReported)
	}
}
