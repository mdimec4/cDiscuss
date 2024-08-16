package main

import (
	"net/http"
)

type userService struct {
	sessionStore                   sessionStoreItf
	databaseServiceUser            databaseServiceUserItf
	proofOfWorkConformation        proofOfWorkConformationItf
	doRequireProofOfWorkInRequests bool
}

func newUserService(sessionStore sessionStoreItf, databaseServiceUser databaseServiceUserItf,
	proofOfWorkConformation proofOfWorkConformationItf, doRequireProofOfWorkInRequests bool) *userService {
	return &userService{sessionStore: sessionStore, databaseServiceUser: databaseServiceUser,
		proofOfWorkConformation: proofOfWorkConformation, doRequireProofOfWorkInRequests: doRequireProofOfWorkInRequests}
}

func (userService *userService) login(powString, username string, password string) (*http.Cookie, *user, error) {
	err := validateUsername(username)
	if err != nil {
		return nil, nil, err
	}
	err = validatePassword(password)
	if err != nil {
		return nil, nil, err
	}

	if userService.doRequireProofOfWorkInRequests && userService.proofOfWorkConformation != nil {
		err = userService.proofOfWorkConformation.isTokenAceptableStore(powString, proofOfWorkLoginRequiredHardnes, username)
		if err != nil {
			return nil, nil, err
		}
	}

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

func (userService *userService) getSessionUser(sessionCookie *http.Cookie) (*user, error) {
	sessionToken, err := validateSessionCookie(sessionCookie)
	if err != nil {
		return nil, err
	}
	return userService.sessionStore.getUser(sessionToken)
}

func (userService *userService) logout(sessionCookie *http.Cookie) error {
	sessionToken, err := validateSessionCookie(sessionCookie)
	if err != nil {
		return err
	}
	return userService.sessionStore.logout(sessionToken)
}

func (userService *userService) createUser(powString, username string, password string) (*http.Cookie, *user, error) {
	err := validateUsername(username)
	if err != nil {
		return nil, nil, err
	}
	err = validatePassword(password)
	if err != nil {
		return nil, nil, err
	}

	if userService.doRequireProofOfWorkInRequests && userService.proofOfWorkConformation != nil {
		err = userService.proofOfWorkConformation.isTokenAceptableStore(powString, proofOfWorkCreateUserRequiredHardnes, username)
		if err != nil {
			return nil, nil, err
		}
	}

	isAdminRole := false
	user, err := userService.databaseServiceUser.createUser(username, password, isAdminRole)
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
