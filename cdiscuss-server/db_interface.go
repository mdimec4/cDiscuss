package main

import "fmt"
import "errors"
import "crypto/sha256"

const (
	errUserAlreadyExists = errors.New("User already exists")
	errUserDoesntExist   = errors.New("User doesn't exist")
	errUserWrongPassword = errors.New("Wrong user password")
	saltLen              = 21
)

type Comment struct {
	Id          int64  `json: id`
	UrlHash     string `json: urlHash`
	IdUser      int64  `json: idUser`
	DtCreated   int64  `json dtCreated`
	CommentBody string `json: commentBody`
}

type PageComments struct {
	Offset         uint64                  `json: offset`
	RequestedCount uint64                  `json: requestedCount`
	Count          uint64                  `json: count`
	Total          uint64                  `json: total`
	Comments       []CommentJoinedWithUser `json: comments`
}

type CommentJoinedWithUser struct {
	Id          int64  `json: id`
	Username    string `json: username`
	DtCreated   int64  `json dtCreated`
	CommentBody string `json: commentBody`
}

type User struct {
	Id        int64
	Username  string
	AdminRole bool
}

type DatabseServiceItf interface {
	ListPageComments(urlHash string, offset uint64, count uint64) (*PageComments, error)
	GetComment(id int64) (*Comment, error)
	DeleteComment(id int64) error

	CreaeUser(username string, password string, adminRole bool) (*User, error)
	ModifyUserPassword(id uint64, oldPassword string, newPassword string) error
	ModifyUserAdminRole(id uint64, adminRole bool) (*User, error)
	AuthenticateUser(username string, password string) (*User, error)
	GetUser(id string) (*User, error)
	GetUserByUsername(username string) (*User, error)
	DeleteUser(id int64) error
}

func generateSalt() string {
	return generateRandomStr(saltLen)
}

func getPasswordAndSaltSHA256Hash(password string, salt string) (string, error) {
	if password == "" {
		return "", errors.New("Empty password")
	}
	if salt == "" {
		return "", errors.New("Empty salt")
	}
	passwordAndSalt := password + salt
	sum := sha256.Sum256([]byte(passwordAndSalt))
	return fmt.Sprintf("%x", sum), nil
}
