package main

import "fmt"
import "time"
import "log/slog"
import "crypto/sha256"

func main() {
	var db databseServiceItf
	db, err := newPostgresAdapter("postgresql://localhost:5432/cDiscuss?sslmode=disable")
	if err != nil {
		slog.Error("connect", slog.Any("error", err))
		return
	}
	user, err := db.createUser("miha", "ahim", false)
	if err != nil {
		slog.Error("create user", slog.Any("error", err))
		return
	}
	fmt.Println(user)
	user, err = db.authenticateUser("miha", "ahim")
	if err != nil {
		slog.Error("authenticate user", slog.Any("error", err))
		return
	}

	urlHash := fmt.Sprintf("%x", sha256.Sum256([]byte("https://www.example.com")))

	err = db.createComment(urlHash, user.id, time.Now(), "besedilo")
	if err != nil {
		slog.Error("create comment", slog.Any("error", err))
		return
	}
	/*
		err = db.deleteUser(user.id)
		if err != nil {
			slog.Error("delete user", slog.Any("error", err))
			return
		}
	*/
	pageComments, err := db.listPageComments(urlHash, 0, 100)
	if err != nil {
		slog.Error("list comments", slog.Any("error", err))
		return
	}
	fmt.Println(pageComments)
}
