package main

import (
	"net/http"
)

var (
	errInternalServer = newInternalServerError("Internal server error!", http.StatusInternalServerError)

	errUserAlreadyExists      = newValidationError("User already exists.", http.StatusConflict)
	errUserDoesntExist        = newValidationError("User doesn't exist.", http.StatusUnauthorized)
	errCommentDoesntExist     = newValidationError("Comment doesn't exist.", http.StatusNotFound)
	errUserWrongPassword      = newValidationError("Wrong user password.", http.StatusUnauthorized)
	errUrlHashLen             = newValidationError("Wrong URL hash length.", http.StatusBadRequest)
	errUsernameToShort        = newValidationError("Username is too short.", http.StatusBadRequest)
	errUsernameToLong         = newValidationError("Username is too long.", http.StatusBadRequest)
	errUsernameUnAllowedChars = newValidationError("Username contains un allowed chars.", http.StatusBadRequest)
	errPasswordToShort        = newValidationError("Password is too short.", http.StatusBadRequest)
	errPasswordToLong         = newValidationError("Password is too long.", http.StatusBadRequest)

	errInvalidPowToken = newValidationError("Invalid POW token.", http.StatusUnauthorized)

	errUserSessionIsNotValid = newValidationError("User session is not valid or doesn't exist", http.StatusUnauthorized)
)

type validationError struct {
	ErrStr     string // exported so it can ce used as json DTO for error
	HttpStatus int
}

func newValidationError(errStr string, httpStatus int) validationError {
	var err validationError
	err.ErrStr = errStr
	err.HttpStatus = httpStatus
	return err
}

func (err validationError) Error() string {
	return err.errStr
}

type internalServerError struct {
	ErrStr     string // exported so it can ce used as json DTO for error
	HttpStatus int
}

func newInternalServerError(errStr string, httpStatus int) internalServerError {
	var err internalServerError
	err.ErrStr = errStr
	err.HttpStatus = httpStatus
	return err
}
func (err internalServerError) Error() string {
	return err.errStr
}
