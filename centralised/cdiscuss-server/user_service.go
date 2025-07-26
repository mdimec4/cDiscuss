package main

import (
	"net/http"
	"time"
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

func (userService *userService) logout(sessionCookie *http.Cookie) (*http.Cookie, error) {
	sessionToken, err := validateSessionCookie(sessionCookie)
	if err != nil {
		return nil, err
	}
	err = userService.sessionStore.logout(sessionToken)
	if err != nil {
		return nil, err
	}

	sessionCookie.Expires = time.Time{}
	sessionCookie.MaxAge = -1

	return sessionCookie, nil
}

func (userService *userService) getLoginProofOfWorkRequiredHardnes() uint {
	return proofOfWorkLoginRequiredHardnes
}

func (userService *userService) getCreateUserProofOfWorkRequiredHardnes() uint {
	return proofOfWorkCreateUserRequiredHardnes

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

func (userService *userService) modifyPassword(sessionCookie *http.Cookie, oldPassword string, newPassword string) error {
	err := validatePassword(newPassword)
	if err != nil {
		return err
	}

	user, err := userService.getSessionUser(sessionCookie)
	if err != nil {
		return err
	}

	return userService.databaseServiceUser.modifyUserPassword(user.Id, oldPassword, newPassword)
}

func (userService *userService) deleteAccount(sessionCookie *http.Cookie) (*http.Cookie, error) {
	user, err := userService.getSessionUser(sessionCookie)
	if err != nil {
		return nil, err
	}

	err = userService.databaseServiceUser.deleteUser(user.Id)
	if err != nil {
		return nil, err
	}

	return userService.logout(sessionCookie)
}

func (userService *userService) createUserAsAdmin(sessionCookie *http.Cookie, username string, password string, adminRole bool) (*user, error) {
	user, err := userService.getSessionUser(sessionCookie)
	if err != nil {
		return nil, err
	}

	if !user.AdminRole {
		return nil, errUserNotAdmin
	}

	err = validateUsername(username)
	if err != nil {
		return nil, err
	}
	err = validatePassword(password)
	if err != nil {
		return nil, err
	}

	return userService.databaseServiceUser.createUser(username, password, adminRole)
}
