package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"log/slog"
	"time"
)

// implements interfaces: databaseServiceCommentItf, databaseServiceUserItf,
// databaseServiceProofOfWorkItf, databaseServiceSessionItf and finaly databaseServiceItf
type postgresAdapter struct {
	connString string
	db         *sql.DB
}

func newPostgresAdapter(connStr string) (*postgresAdapter, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to postgres: %w", err)
	}
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("Failed to ping postgres: %w", err)
	}
	postgresAdapter := &postgresAdapter{connString: connStr, db: db}

	return postgresAdapter, nil
}

func (postgresAdapter postgresAdapter) closeDb() error {
	err := postgresAdapter.db.Close()
	if err != nil {
		return fmt.Errorf("Failed to close postgres: %w", err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) listPageComments(urlHash string, offset uint64, count uint64) (*pageComments, error) {
	if len(urlHash) != urlHashLen {
		return nil, errUrlHashLen
	}

	tx, err := postgresAdapter.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("Failed to read comments (create transaction): %w", err)
	}

	totalCount, err := getCommentsTotalCount(tx, urlHash)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback (getting comments)!", slog.Any("error", err2))
		}
		return nil, fmt.Errorf("Failed to read comments (total comments count): %w", err)
	}

	commentsSlice, err := getComments(tx, urlHash, offset, count)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback (getting comments)!", slog.Any("error", err2))
		}
		return nil, fmt.Errorf("Failed to read comments (getting comments): %w", err)
	}
	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("Failed to commit reading comments: %w", err)
	}

	actualCount := len(commentsSlice)
	pageComments := &pageComments{Offset: offset, RequestedCount: count, Count: uint64(actualCount), Total: totalCount, Comments: commentsSlice}
	return pageComments, nil
}

func getCommentsTotalCount(tx *sql.Tx, urlHash string) (uint64, error) {
	if len(urlHash) != urlHashLen {
		return 0, errUrlHashLen
	}

	const query = "SELECT COUNT(*) FROM comments WHERE url_hash=$1"
	var row *sql.Row = tx.QueryRow(query, urlHash)

	var totalCount uint64
	err := row.Scan(&totalCount)
	if err != nil {
		return 0, err
	}
	return totalCount, err
}

func getComments(tx *sql.Tx, urlHash string, offset uint64, count uint64) ([]commentJoinedWithUser, error) {
	if len(urlHash) != urlHashLen {
		return nil, errUrlHashLen
	}

	const query = `SELECT cm.id, us.username, cm.dt_created, cm.comment_body FROM comments cm
	INNER JOIN users us ON cm.id_user=us.id
	WHERE cm.url_hash=$1 
	ORDER BY cm.id DESC OFFSET $2 LIMIT $3`

	rows, err := tx.Query(query, urlHash, offset, count)
	if err != nil {
		return nil, err
	}

	initialSliceCap := count
	if initialSliceCap > 1024 {
		initialSliceCap = 1024 // limit requested slice size to prevent out of memory DOS attack
	}
	commentsSlice := make([]commentJoinedWithUser, 0, initialSliceCap)

	for rows.Next() {
		comment := commentJoinedWithUser{}
		err = rows.Scan(&comment.Id, &comment.Username, &comment.DtCreated, &comment.CommentBody)
		if err != nil {
			return nil, err
		}
		commentsSlice = append(commentsSlice, comment)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return commentsSlice, nil
}

func (postgresAdapter postgresAdapter) getComment(id int64) (*comment, error) {
	const query = "SELECT id, url_hash, id_user, dt_created, comment_body FROM comments WHERE id=$1 LIMIT 1"
	var row *sql.Row = postgresAdapter.db.QueryRow(query, id)

	comment := &comment{}
	err := row.Scan(&comment.Id, &comment.UrlHash, &comment.DtCreated, &comment.CommentBody)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return comment, errCommentDoesntExist
		}
		return nil, fmt.Errorf("Failed to query a comment id=%d: %w", id, err)
	}
	return comment, nil
}

func (postgresAdapter postgresAdapter) createComment(urlHash string, idUser int64, dtCreated time.Time, commentBody string) (int64, error) {
	if len(urlHash) != urlHashLen {
		return -1, errUrlHashLen
	}

	if commentBody == "" {
		return -1, fmt.Errorf("Failed to crate a comment urlHash=%s, idUser=%d: empty comment body", urlHash, idUser)
	}

	var commentId int64
	const query = "INSERT INTO comments (url_hash, id_user, dt_created, comment_body) VALUES($1, $2, $3, $4) RETURNING id"
	row := postgresAdapter.db.QueryRow(query, urlHash, idUser, dtCreated, commentBody)
	err := row.Scan(&commentId)
	if err != nil {
		return -1, fmt.Errorf("Failed to crate a comment urlHash=%s, idUser=%d: %w", urlHash, idUser, err)
	}
	return commentId, nil
}

func (postgresAdapter postgresAdapter) deleteComment(id int64) error {
	const query = "DELETE FROM comments WHERE id=$1"
	_, err := postgresAdapter.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("Failed to delete a comment id=%d: %w", id, err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) createUser(username string, password string, adminRole bool) (*user, error) {
	if username == "" {
		return nil, fmt.Errorf("Error creating user: empty username")
	}
	if len(username) > usernameMaxLen {
		return nil, errUsernameTooLong
	}
	if password == "" {
		return nil, fmt.Errorf("Error creating user: empty password")
	}

	tx, err := postgresAdapter.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: false})
	if err != nil {
		return nil, fmt.Errorf("Error creating user (create transaction): %w", err)
	}

	userId, err := getUserId(tx, username)
	if !errors.Is(err, sql.ErrNoRows) {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback user creation!", slog.Any("error", err2))
		}
		if err != nil {
			return nil, fmt.Errorf("Error creating user (checking if user already exists): %w", err)
		}
		return nil, errUserAlreadyExists
	}

	var salt string = generateSalt()
	pwHash, err := getPasswordAndSaltSHA256Hash(salt, password)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback user creation!", slog.Any("error", err2))
		}
		return nil, fmt.Errorf("Error creating user (password and salt hash): %w", err)
	}

	const queryInsert = "INSERT INTO users (username, salt, pw_hash, admin_role) VALUES($1, $2, $3, FALSE) RETURNING id"
	row := tx.QueryRow(queryInsert, username, salt, pwHash)
	err = row.Scan(&userId)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback user creation!", slog.Any("error", err2))
		}
		return nil, fmt.Errorf("Error creating user (insert): %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("Failed to commit user creation: %w", err)
	}
	return &user{Id: userId, Username: username, AdminRole: adminRole}, nil
}

func getUserId(tx *sql.Tx, username string) (int64, error) {
	if len(username) > usernameMaxLen {
		return -1, errUsernameTooLong
	}

	const getQuery = "SELECT id FROM users WHERE username=$1 LIMIT 1"
	var row *sql.Row = tx.QueryRow(getQuery, username)
	var userId int64
	err := row.Scan(&userId)
	if err != nil {
		return -1, err
	}
	return userId, nil
}

func (postgresAdapter postgresAdapter) modifyUserPassword(id int64, oldPassword string, newPassword string) error {
	if oldPassword == "" {
		return fmt.Errorf("Error modifing user password: empty old password")
	}
	if newPassword == "" {
		return fmt.Errorf("Error modifing user password: empty new password")
	}

	tx, err := postgresAdapter.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: false})
	if err != nil {
		return fmt.Errorf("Error modifing user password (create transaction): %w", err)
	}

	const getQuery = "SELECT salt, pw_hash FROM users WHERE id=$1 LIMIT 1"
	var row *sql.Row = tx.QueryRow(getQuery, id)
	var oldSalt string
	var oldPwHash string
	err = row.Scan(&oldSalt, oldPwHash)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback user password change!", slog.Any("error", err2))
		}
		if errors.Is(err, sql.ErrNoRows) {
			return errUserDoesntExist
		}
		return fmt.Errorf("Error modifing user password (get user): %w", err)
	}

	oldPwHashRegenerated, err := getPasswordAndSaltSHA256Hash(oldSalt, oldPassword)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback user password change!", slog.Any("error", err2))
		}
		return fmt.Errorf("Error modifing user password (old password and salt hash): %w", err)
	}
	if oldPwHash != oldPwHashRegenerated {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback user password change!", slog.Any("error", err2))
		}
		return errUserWrongPassword
	}

	newSalt := generateSalt()
	newPwHash, err := getPasswordAndSaltSHA256Hash(newSalt, newPassword)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback user password change!", slog.Any("error", err2))
		}
		return fmt.Errorf("Error changing password (password and salt hash): %w", err)
	}

	const queryInsert = "UPDATE users SET salt=$1, pw_hash=$2 WHERE id=$3"
	_, err = tx.Exec(queryInsert, newSalt, newPwHash, id)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			slog.Error("Failed to rollback user password change!", slog.Any("error", err2))
		}
		return fmt.Errorf("Error changing password (update): %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("Failed to commit user password change: %w", err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) modifyUserAdminRole(id int64, adminRole bool) error {
	const query = "UPDATE SET admin_role=$1 WHERE id=$2"
	_, err := postgresAdapter.db.Exec(query, adminRole, id)
	if err != nil {
		return fmt.Errorf("Failed to updae adminRole=%v  id=%d: %w", adminRole, id, err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) authenticateUser(username string, password string) (*user, error) {
	if len(username) > usernameMaxLen {
		return nil, errUsernameTooLong
	}

	const query = "SELECT id, username, salt, pw_hash, admin_role FROM users WHERE username=$1 LIMIT 1"
	var row *sql.Row = postgresAdapter.db.QueryRow(query, username)

	user := &user{}
	var salt string
	var pwHash string
	err := row.Scan(&user.Id, &user.Username, &salt, &pwHash, &user.AdminRole)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

func (postgresAdapter postgresAdapter) getUser(id int64) (*user, error) {
	const query = "SELECT id, username, admin_role FROM users WHERE id=$1 LIMIT 1"
	var row *sql.Row = postgresAdapter.db.QueryRow(query, id)

	user := &user{}
	err := row.Scan(&user.Id, &user.Username, &user.AdminRole)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errUserDoesntExist
		}
		return nil, fmt.Errorf("Failed to query a user id=%d: %w", id, err)
	}
	return user, nil

}

func (postgresAdapter postgresAdapter) getUserByUsername(username string) (*user, error) {
	if len(username) > usernameMaxLen {
		return nil, errUsernameTooLong
	}

	const query = "SELECT id, username, admin_role FROM users WHERE username=$1 LIMIT 1"
	var row *sql.Row = postgresAdapter.db.QueryRow(query, username)

	user := &user{}
	err := row.Scan(&user.Id, &user.Username, &user.AdminRole)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errUserDoesntExist
		}
		return nil, fmt.Errorf("Failed to query a user username='%s': %w", username, err)
	}
	return user, nil
}

func (postgresAdapter postgresAdapter) deleteUser(id int64) error {
	const query = "DELETE FROM users WHERE id=$1"
	_, err := postgresAdapter.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("Failed to delete a user id=%d: %w", id, err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) getPowToken(token string) (*time.Time, error) {
	const query = "SELECT dt_expires FROM used_pow_tokens WHERE pow_token=$1"
	var row *sql.Row = postgresAdapter.db.QueryRow(query, token)

	var dtExpires time.Time
	err := row.Scan(&dtExpires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("Failed to query a pow token='%s': %w", token, err)
	}
	return &dtExpires, nil
}

func (postgresAdapter postgresAdapter) createPowToken(token string, dtExpires time.Time) error {
	const queryInsert = "INSERT INTO used_pow_tokens (pow_token, dt_expires) VALUES($1, $2)"
	_, err := postgresAdapter.db.Exec(queryInsert, token, dtExpires)
	if err != nil {
		return fmt.Errorf("Failed to insert pow token='%s': %w", token, err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) deletePowToken(token string) error {
	const query = "DELETE FROM used_pow_tokens WHERE pow_token=$1"
	_, err := postgresAdapter.db.Exec(query, token)
	if err != nil {
		return fmt.Errorf("Failed to delete pow token='%s': %w", token, err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) deletePowTokensThatExpired(now time.Time) error {
	const query = "DELETE FROM used_pow_tokens WHERE dt_expires <= $1"
	_, err := postgresAdapter.db.Exec(query, now)
	if err != nil {
		return fmt.Errorf("Failed to delete pow tokens<='%v': %w", now, err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) getSession(tokenHash string) (*time.Time, *user, error) {
	const query = `SELECT s.date_expires, usr.id, usr.username, usr.admin_role FROM user_sessions s 
	INNER JOIN users usr ON usr.id = s.id_user
	WHERE session_token_hash=$1`
	var row *sql.Row = postgresAdapter.db.QueryRow(query, tokenHash)

	var dtExpires time.Time
	var user user

	err := row.Scan(&dtExpires, user.Id, &user.Username, &user.AdminRole)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("Failed to query a seassion tokenHash='%s': %w", tokenHash, err)
	}
	return &dtExpires, &user, nil
}

func (postgresAdapter postgresAdapter) createSession(tokenHash string, idUser int64, dtExpires time.Time) error {
	const queryInsert = "INSERT INTO user_sessions (seassion_token_hash, id_user, dt_expires) VALUES($1, $2, $3)"
	_, err := postgresAdapter.db.Exec(queryInsert, idUser, tokenHash, dtExpires)
	if err != nil {
		return fmt.Errorf("Failed to insert seassion tokenHash='%s': %w", tokenHash, err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) deleteSession(tokenHash string) error {
	const query = "DELETE FROM user_sessions WHERE seassion_token_hash=$1"
	_, err := postgresAdapter.db.Exec(query, tokenHash)
	if err != nil {
		return fmt.Errorf("Failed to delete seassion tokenHash='%s': %w", tokenHash, err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) deleteSessionsThatExpired(now time.Time) error {
	const query = "DELETE FROM user_sessions WHERE dt_expires <= $1"
	_, err := postgresAdapter.db.Exec(query, now)
	if err != nil {
		return fmt.Errorf("Failed to delete seassions <='%v': %w", now, err)
	}
	return nil
}

func (postgresAdapter postgresAdapter) deleteSessionsForUser(idUser int64) error {
	const query = "DELETE FROM user_sessions WHERE id_user <= $1"
	_, err := postgresAdapter.db.Exec(query, idUser)
	if err != nil {
		return fmt.Errorf("Failed to delete seassions for user <='%d': %w", idUser, err)
	}
	return nil
}
