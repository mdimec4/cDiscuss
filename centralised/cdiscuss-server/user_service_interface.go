package main

import "time"
import "net/http"

const (
	saltLen                              = 21
	usernameMinLen                       = 4
	usernameMaxLen                       = 50
	passwordMinLen                       = 21
	passwordMaxLen                       = 100
	proofOfWorkCreateUserRequiredHardnes = uint(19)
	proofOfWorkCreteUserMaxAge           = time.Duration(5 * time.Minute)
)

type userServiceItf interface {
	login(username string, password string) (*http.Cookie, error)
	logout() error
	getSessionUser(sessionCookie *http.Cookie) *user

	getCreateUserProofOfWorkRequredHardnes() uint

	// creates new user in db and creates session
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
