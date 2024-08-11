package main

import (
	"net/http"
	"regexp"
	"time"
)

const (
	saltLen                              int           = 21
	usernameMinLen                       int           = 4
	usernameMaxLen                       int           = 50
	passwordMinLen                       int           = 21
	passwordMaxLen                       int           = 100
	proofOfWorkLoginRequiredHardnes      uint          = 10
	proofOfWorkCreateUserRequiredHardnes uint          = 19
	seassionExpiresAge                   time.Duration = time.Hour * 24 * 30 * 6 // rughly six months
	seassionCleanUpPeriod                time.Duration = time.Hour
	sessionCookieName                    string        = "CDSESSION"
)

type userServiceItf interface {
	login(powString string, username string, passwoed string) (*http.Cookie, *user, error)
	getSessionUser(sessionCookie *http.Cookie) (*user, error)

	getLoginProofOfWorkRequredHardnes() uint
	getCreateUserProofOfWorkRequredHardnes() uint

	// creates new user in db and creates session
	// TODO validate username ^[A-Za-z0-9]{4,50}$ because ':' char is not allowed (POW token)
	createUser(powString string, username string, password string) (*http.Cookie, error)

	modifyPassword(oldPassword string, newPassword string) error

	// deletes user from db and destroys session
	deleteAccount() error
}

type adminUserServiceItf interface {
	createUser(username string, password string, adminRole bool) (*user, error)
	deleteUser(idUser int64) error // also destroys existing sessions
	modifyUserPassword(id int64, oldPassword string, newPassword string) error
	modifyUserAdminRole(id int64, adminRole bool) error // also modifies existing sessions
}

var usernameRegex = regexp.MustCompile(`(?m)^[a-zA-Z0-9]*$`) // because of Proof Of Work token format username must not contain ':' char.

func validateUsername(username string) error {
	if len(username) < usernameMinLen {
		return errUsernameTooShort
	}
	if len(username) > usernameMaxLen {
		return errUsernameTooLong
	}

	match := usernameRegex.Match([]byte(username))
	if !match {
		return errUsernameUnallowedChars
	}

	return nil
}

func validatePassword(password string) error {
	if len(password) < passwordMinLen {
		return errPasswordTooShort
	}
	if len(password) > passwordMaxLen {
		return errPasswordTooLong
	}

	return nil
}
