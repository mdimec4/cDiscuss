package main

var (
	errUserAlreadyExists  = newValidationError("User already exists")
	errUserDoesntExist    = newValidationError("User doesn't exist")
	errCommentDoesntExist = newValidationError("Comment doesn't exist")
	errUserWrongPassword  = newValidationError("Wrong user password")
	errUrlHashLen         = newValidationError("wrong URL hash length")
	errUsernameToLong     = newValidationError("username is too long")
)

type validationError struct {
	errStr string
}

func newValidationError(errStr string) validationError {
	var err validationError
	err.errStr = errStr
	return err
}

func (err validationError) Error() string {
	return err.errStr
}
