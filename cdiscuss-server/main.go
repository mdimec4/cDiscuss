package main

import "fmt"
import "time"
import "log/slog"
import "crypto/sha512"

func main() {
	var db DatabseServiceItf
	db, err := NewPostgresAdapter("postgresql://localhost:5432/cDiscuss?sslmode=disable")
	if err != nil {
		slog.Error("connect", slog.Any("error", err))
		return
	}
	/*
		user, err := db.CreateUser("miha", "ahim", false)
		if err != nil {
			slog.Error("create user", slog.Any("error", err))
			return
		}
		fmt.Println(user)
	*/
	user, err := db.AuthenticateUser("miha", "ahim")
	if err != nil {
		slog.Error("authenticate user", slog.Any("error", err))
		return
	}

	urlHash := fmt.Sprintf("%x", sha512.Sum512([]byte("https://www.example.com")))

	err = db.CreateComment(urlHash, user.Id, time.Now(), "besedilo")
	if err != nil {
		slog.Error("create comment", slog.Any("error", err))
		return
	}

	pageComments, err := db.ListPageComments(urlHash, 0, 100)
	if err != nil {
		slog.Error("list comments", slog.Any("error", err))
		return
	}
	fmt.Println(pageComments)
}
