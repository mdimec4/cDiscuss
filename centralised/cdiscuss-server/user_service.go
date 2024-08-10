package main

import (
	"net/http"
)

type userService struct {
	sessionStore        sessionStoreItf
	databaseServiceUser databaseServiceUserItf
}

func newUserService(sessionStore sessionStoreItf, databaseServiceUser databaseServiceUserItf) *userService {
	return &userService{sessionStore: sessionStore, databaseServiceUser: databaseServiceUser}
}

func (userService *userService) login(username string, password string) (*http.Cookie, *user, error) {
	user, err := userService.databaseServiceUser.authenticateUser(username, password)
	if err != nil {
		return nil, nil, err
	}
	sessionToken, expiresTime, err := userService.sessionStore.newSession(user)
	if err != nil {
		return nil, nil, err
	}

	cookie := &http.Cookie{Name: sessionCookieName, Value: sessionToken, Expires: expiresTime}

	return cookie, user, nil
}
