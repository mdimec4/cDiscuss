package main

import (
	"net/http"
)

type errWithHttpStatus interface {
	error
	getHttpStatus() int
}

var (
	errInternalServer = newInternalServerError("Internal server error!", http.StatusInternalServerError)

	errUserAlreadyExists      = newValidationError("User already exists.", http.StatusConflict)
	errUserDoesntExist        = newValidationError("User doesn't exist.", http.StatusUnauthorized)
	errCommentDoesntExist     = newValidationError("Comment doesn't exist.", http.StatusNotFound)
	errUserWrongPassword      = newValidationError("Wrong user password.", http.StatusUnauthorized)
	errUrlHashLen             = newValidationError("Wrong URL hash length.", http.StatusBadRequest)
	errUsernameTooShort       = newValidationError("Username is too short.", http.StatusBadRequest)
	errUsernameTooLong        = newValidationError("Username is too long.", http.StatusBadRequest)
	errUsernameUnallowedChars = newValidationError("Username contains unallowed chars.", http.StatusBadRequest)
	errPasswordTooShort       = newValidationError("Password is too short.", http.StatusBadRequest)
	errPasswordTooLong        = newValidationError("Password is too long.", http.StatusBadRequest)

	errInvalidPowToken = newValidationError("Invalid POW token.", http.StatusUnauthorized)

	errUserSessionIsNotValid = newValidationError("User session is not valid or doesn't exist", http.StatusUnauthorized)
)

type validationError struct {
	errStr     string
	httpStatus int
}

func newValidationError(errStr string, httpStatus int) validationError {
	var err validationError
	err.errStr = errStr
	err.httpStatus = httpStatus
	return err
}

func (err validationError) Error() string {
	return err.errStr
}

func (err validationError) getHttpStatus() int {
	return err.httpStatus
}

type internalServerError struct {
	errStr     string
	httpStatus int
}

func newInternalServerError(errStr string, httpStatus int) internalServerError {
	var err internalServerError
	err.errStr = errStr
	err.httpStatus = httpStatus
	return err
}

func (err internalServerError) Error() string {
	return err.errStr
}

func (err internalServerError) getHttpStatus() int {
	return err.httpStatus
}

type errorDTO struct {
	ErrStr     string `json: err`
	HttpStatus int    `json: status`
}

func newErrorDTO(errHttp errWithHttpStatus) errorDTO {
	return errorDTO{ErrStr: errHttp.Error(), HttpStatus: errHttp.getHttpStatus()}
}
