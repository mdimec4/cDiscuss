package main

import "fmt"
import "time"
import "errors"
import "crypto/sha256"

const (
	saltLen        = 21
	urlHashLen     = 32 // sha256
	usernameMaxLen = 50
)

type comment struct {
	Id          int64     `json: id`
	UrlHash     string    `json: urlHash`
	IdUser      int64     `json: idUser`
	DtCreated   time.Time `json dtCreated`
	CommentBody string    `json: commentBody`
}

type pageComments struct {
	Offset         uint64                  `json: offset`
	RequestedCount uint64                  `json: requestedCount`
	Count          uint64                  `json: count`
	Total          uint64                  `json: total`
	Comments       []commentJoinedWithUser `json: comments`
}

type commentJoinedWithUser struct {
	Id          int64     `json: id`
	Username    string    `json: username`
	DtCreated   time.Time `json dtCreated`
	CommentBody string    `json: commentBody`
}

type user struct {
	id        int64
	username  string
	adminRole bool
}

type databseServiceItf interface {
	listPageComments(urlHash string, offset uint64, count uint64) (*pageComments, error)
	getComment(id int64) (*comment, error)
	createComment(urlHash string, idUser int64, dtCreated time.Time, commentBody string) (int64, error)
	deleteComment(id int64) error

	createUser(username string, password string, adminRole bool) (*user, error)
	modifyUserPassword(id int64, oldPassword string, newPassword string) error
	modifyUserAdminRole(id int64, adminRole bool) error
	authenticateUser(username string, password string) (*user, error)
	getUser(id int64) (*user, error)
	getUserByUsername(username string) (*user, error)
	deleteUser(id int64) error
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
