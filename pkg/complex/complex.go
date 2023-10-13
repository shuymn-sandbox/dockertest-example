package complex

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/shuymn-sandbox/dockertest-example/internal/model"
	"go.jetpack.io/typeid"
)

type App struct {
	mysql     *sql.DB
	firestore *firestore.Client
}

func NewApp(mysql *sql.DB, firestore *firestore.Client) *App {
	return &App{mysql: mysql, firestore: firestore}
}

func (a *App) CreateUser(ctx context.Context, username, email string) (*model.User, error) {
	now := time.Now()

	res, err := a.mysql.ExecContext(ctx, "INSERT INTO users (username, email, created_at, updated_at) VALUES (?, ?, ?, ?)", username, email, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}
	return &model.User{
		ID:        int(id),
		Username:  username,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (a *App) SendMessage(ctx context.Context, user *model.User, body string) (*model.Message, error) {
	tid, err := typeid.New("message")
	if err != nil {
		return nil, fmt.Errorf("failed to create typeid: %w", err)
	}

	now := time.Now()
	_, err = a.firestore.Collection("messages").Doc(tid.String()).Set(ctx, map[string]any{
		"sender": map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
		"body":       body,
		"created_at": now.Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add message: %w", err)
	}

	return &model.Message{
		ID:        tid.String(),
		Sender:    user,
		Body:      body,
		CreatedAt: now,
	}, err
}
