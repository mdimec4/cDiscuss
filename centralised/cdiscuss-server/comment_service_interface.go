package main

import (
	"net/http"
)

const (
	urlHashLen int = 64 // sha256
)

type commentiServiceItf interface {
	listPageComments(urlHash string, offset uint64, count uint64) (*pageComments, error)
	createComment(sessionCookie *http.Cookie, idParent *int64, urlHash string, commentBody string) (int64, error)
	deleteComment(sessionCookie *http.Cookie, id int64) error
}
