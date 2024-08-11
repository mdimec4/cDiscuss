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
