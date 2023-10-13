package sample

import (
	"context"
	"database/sql"
	"fmt"
)

type App struct {
	db *sql.DB
}

func New(db *sql.DB) *App {
	return &App{db: db}
}

func (a *App) CreateUser(ctx context.Context, username, email string) error {
	_, err := a.db.ExecContext(ctx, "INSERT INTO users (username, email) VALUES (?, ?)", username, email)
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}
	return nil
}
