package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"log/slog"
	"time"
)

const instanceIDLen int = 21

var instanceID string
var db databaseServiceItf
var mq mqServiceItf

var doRequireProofOfWorkInRequests *bool = flag.Bool("pow", true, "Enable Proof Of Work for some of requsts (create user, login, create comment)")

type cbObj struct { // TODO remove
}

// implement mqMessageCbItf
func (obj *cbObj) onMessage(msg mqMessage) {
	fmt.Printf("%v", msg)
}

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
	if err != nil {
		slog.Error("create", slog.Any("error", err))
		return
	}

	err = mq.registerMessageCB("Operacija", &cbObj{}, true)
	if err != nil {
		slog.Error("register", slog.Any("error", err))
		return
	}
	err = mq.sendMessage("Operacija", "niko arg")
	if err != nil {
		slog.Error("send", slog.Any("error", err))
		return
	}
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

	commentId, err := db.createComment(urlHash, user.Id, time.Now(), "besedilo")
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
