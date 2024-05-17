package main

var (
	errUserAlreadyExists  = newValidationError("User already exists")
	errUserDoesntExist    = newValidationError("User doesn't exist")
	errCommentDoesntExist = newValidationError("Comment doesn't exist")
	errUserWrongPassword  = newValidationError("Wrong user password")
	errUrlHashLen         = newValidationError("Wrong URL hash length")
	errUsernameToShort    = newValidationError("Username is too short")
	errUsernameToLong     = newValidationError("Username is too long")
	errPasswordToShort    = newValidationError("Password is too short")
	errPasswordToLong     = newValidationError("Password is too long")

	errUserSessionIsNotValid = newValidationError("User session is not valid or doesn't exist")
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
