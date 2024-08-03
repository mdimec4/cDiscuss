package main

import "fmt"
import "time"
import "log/slog"
import "crypto/sha256"

const instanceIDLen int = 21

var instanceID string
var db databaseServiceItf
var mq mqServiceItf

func main() {
	var err error

	instanceID = generateRandomStr(instanceIDLen)

	dbConnString := "postgresql://postgres:postgres@localhost:5432/cDiscuss?sslmode=disable"
	db, err = newPostgresAdapter(dbConnString)
	if err != nil {
		slog.Error("connect", slog.Any("error", err))
		return
	}
	mq, err = newMqPostgres(dbConnString, instanceID)
	mq.registerMessageCB(func(msg mqMessage) {
		fmt.Printf("%v", msg)
	})
	time.Sleep(3 * time.Second) // TODO REMOVE
	mq.sendMessage("Operacija", "niko arg")
	time.Sleep(3000 * time.Second) // TODO REMOVE
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

	commentId, err := db.createComment(urlHash, user.id, time.Now(), "besedilo")
	if err != nil {
		slog.Error("create comment", slog.Any("error", err))
		return
	}
	fmt.Println(commentId)
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
