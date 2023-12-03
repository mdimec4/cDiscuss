package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
)

type PostgresAdapter struct {
	db *sql.DB
}

func NewPostgresAdapter(connStr string) (*PostgresAdapter, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to postgres: %w", err)
	}
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("Failed to ping postgres: %w", err)
	}
	postgresAdapter := &PostgresAdapter{db: db}

	return postgresAdapter, nil
}

func (postgresAdapter PostgresAdapter) Close() error {
	err := postgresAdapter.Close()
	if err != nil {
		return fmt.Errorf("Failed to close postgres: %w", err)
	}
	return nil
}

// implement DatabseServiceItf interface
func (postgresAdapter PostgresAdapter) ListPageComments(urlHash string, offset uint64, count uint64) (*PageComments, error) {
	tx, err := postgresAdapter.db.BeginTx(context.Background(), &TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("Failed to read comments (create transaction): %w", err)
	}

	totalCount, err := getCommentsTotalCount(tx, urlHash)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback getting: %w", err2)
		}
	}

	return nil, fmt.Errorf("Not implemented")
}

func getCommentsTotalCount(tx *sql.Tx, urlHash string) (uint64, error) {
	query := "SELECT COUNT(*) FROM comments WHERE page_hash = ?"
	var row *sql.Row = tx.QueryRow(query, urlHash)

	var totalCount uint64
	err := row.Scan(&totalCount)
	if err != nil {
		return 0, err
	}
	return totalCount, err
}

func getComments(tx *sql.Tx, urlHash string, offset uint64, count uint64) ([]CommentJoinedWithUser, error) {
	const query = ```
	SELECT id, username, dt_created, comment_body FROM comments
	INNER JOIN users ON id_user = id
	WHERE url_hash = ?
	ORDER BY id DESC OFFSET ? LIMIT ?
	```
}

func (postgresAdapter PostgresAdapter) GetComment(id int64) (*Comment, error) {
	const query = "SELECT id, url_hash, id_user, dt_created, comment_body FROM comments WHERE id = ? LIMIT 1"
	var row *sql.Row = postgresAdapter.db.QueryRow(query, id)

	comment := &Comment{}
	err := row.Scan(&comment.Id, &comment.UrlHash, &comment.DtCreated, &comment.CommentBody)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("Failed to query a comment id=%d: %w", id, err)
	}
	return comment, nil
}

func (postgresAdapter PostgresAdapter) DeleteComment(id int64) error {
	const query = "DELETE FROM comments WHERE id = ?"
	_, err := postgresAdapter.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("Failed to delete a comment id=%d: %w", id, err)
	}
	return nil
}

func (postgresAdapter PostgresAdapter) CreaeUser(username string, password string, adminRole bool) (*User, error) {
	tx, err := postgresAdapter.db.BeginTx(context.Background(), &TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: false})
	if err != nil {
		return nil, fmt.Errorf("Error creating user (create transaction): %w", err)
	}

	getQury := "SELECT id FROM users WHERE username = ? LIMIT 1"
	var row *sql.Row = tx.QueryRow(query, username)
	var userId int64
	err = row.Scan(&userId)
	if err != sql.ErrNoRows {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback user creation: %w", err2)
		}
		if err != null {
			return nil, fmt.Errorf("Error creating user (checking if user already exists): %w", err)
		}
		return nil, errUserAlreadyExists
	}

	var salt String = generateSalt()
	pwHash, err := getPasswordAndSaltSHA256Hash(salt, password)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback user creation: %w", err2)
		}
		return nil, fmt.Errorf("Error creating user (password and salt hash): %w", err)
	}

	queryInsert := "INSERT INTO users (username, salt, pw_hash, admin_role) VALUES(?, ?, ?, FALSE) where id = ?"
	result, err := tx.Exec(queryInsert, username, salt, pwHash, userId)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback user creation: %w", err2)
		}
		return nil, fmt.Errorf("Error creating user (insert): %w", err)
	}
	userId, err = result.LastInsertId()
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback user creation: %w", err2)
		}
		return nil, fmt.Errorf("Error creating user (insert - GetLastInsertId): %w", err)
	}
	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("Failed to commit user creation: %w", err)
	}
	return &User{Id: userId, Username: username, AdminRole: false}, null
}

func (postgresAdapter PostgresAdapter) ModifyUserPassword(id uint64, oldPassword string, newPassword string) error {
	tx, err := postgresAdapter.db.BeginTx(context.Background(), &TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: false})
	if err != nil {
		return fmt.Errorf("Error modifing user password (create transaction): %w", err)
	}

	getQury := "SELECT salt, pw_hash FROM users WHERE id = ? LIMIT 1"
	var row *sql.Row = tx.QueryRow(query, id)
	var oldSalt string
	var oldPwHash string
	err = row.Scan(&oldSalt, oldPwHash)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback user password change: %w", err2)
		}
		if err == sql.ErrNoRows {
			return errUserDoesntExist
		}
		return fmt.Errorf("Error modifing user password (get user): %w", err)
	}

	oldPwHashRegenerated, err := getPasswordAndSaltSHA256Hash(oldSalt, oldPassword)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback user password change: %w", err2)
		}
		return fmt.Errorf("Error modifing user password (old password and salt hash): %w", err)
	}
	if oldPwHash != oldPwHashRegenerated {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback user password change: %w", err2)
		}
		return errUserWrongPassword
	}

	newSalt := generateSalt()
	newPwHash, err := getPasswordAndSaltSHA256Hash(newSalt, newPassword)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback user creation: %w", err2)
		}
		return nil, fmt.Errorf("Error creating user (password and salt hash): %w", err)
	}

	queryInsert := "INSERT INTO users (salt, pw_hash) VALUES(?, ?) where id = ?"
	result, err := tx.Exec(queryInsert, newSalt, newPwHash, userId)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			log.Errorf("Failed to rollback user password change: %w", err2)
		}
		return nil, fmt.Errorf("Error changing password (insert): %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("Failed to commit user password change: %w", err)
	}
	return nil
}

func (postgresAdapter PostgresAdapter) ModifyUserAdminRole(id uint64, adminRole bool) (*User, error) {
	const query = "UPDATE SET admin_role = ? WHERE id = ?"
	_, err := postgresAdapter.db.Exec(query, adminRole, id)
	if err != nil {
		return fmt.Errorf("Failed to updae adminRole=%v  id=%d: %w", adminRole, id, err)
	}
	return nil
}

func (postgresAdapter PostgresAdapter) AuthenticateUser(username string, password string) (*User, error) {
	const query = "SELECT id, username, salt, pw_hash, admin_role FROM users WHERE username = ? LIMIT 1"
	var row *sql.Row = postgresAdapter.db.QueryRow(query, username)

	user := &User{}
	var salt string
	var pwHash string
	err := row.Scan(&user.Id, &user.Username, &salt, &pwHash, &user.AdminRole)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errUserDoesntExist
		}
		return nil, fmt.Errorf("Failed to authenicate a user (query a user) username='%s': %w", username, err)
	}

	pwHashRegenerated, err := getPasswordAndSaltSHA256Hash(salt, password)
	if err != nil {
		return nil, fmt.Errorf("Failed to authenicate a user (password check) username='%s': %w", username, err)
	}

	if pwHash != pwHashRegenerated {
		return nil, errUserWrongPassword
	}
	return user, nil
}

func (postgresAdapter PostgresAdapter) GetUser(id string) (*User, error) {
	const query = "SELECT id, username, admin_role FROM users WHERE id = ? LIMIT 1"
	var row *sql.Row = postgresAdapter.db.QueryRow(query, id)

	user := &User{}
	err := row.Scan(&user.Id, &user.Username, &user.AdminRole)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errUserDoesntExist
		}
		return nil, fmt.Errorf("Failed to query a user id=%d: %w", id, err)
	}
	return user, nil
}

func (postgresAdapter PostgresAdapter) GetUserByUsername(username string) (*User, error) {
	const query = "SELECT id, username, admin_role FROM users WHERE username = ? LIMIT 1"
	var row *sql.Row = postgresAdapter.db.QueryRow(query, username)

	user := &User{}
	err := row.Scan(&user.Id, &user.Username, &user.AdminRole)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errUserDoesntExist
		}
		return nil, fmt.Errorf("Failed to query a user username='%s': %w", username, err)
	}
	return user, nil
}

func (postgresAdapter PostgresAdapter) DeleteUser(id int64) error {
	const query = "DELETE FROM users WHERE id = ?"
	_, err := postgresAdapter.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("Failed to delete a user id=%d: %w", id, err)
	}
	return nil
}
