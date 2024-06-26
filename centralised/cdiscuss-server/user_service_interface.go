package main

import "time"
import "net/http"

const (
	saltLen                              int           = 21
	usernameMinLen                       int           = 4
	usernameMaxLen                       int           = 50
	passwordMinLen                       int           = 21
	passwordMaxLen                       int           = 100
	proofOfWorkCreateUserRequiredHardnes int           = 19
	seassionExpiresAge                   time.Duration = time.Hour * 24 * 30 * 6 // rughly six months
	seassionCleanUpPeriod                time.Duration = time.Hour
)

type userServiceItf interface {
	getSessionUser(sessionCookie *http.Cookie) *user

	getCreateUserProofOfWorkRequredHardnes() uint

	// creates new user in db and creates session
	// TODO validate username ^[A-Za-z0-9]{1,50}$ because ':' char is not allowed (POW token)
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
