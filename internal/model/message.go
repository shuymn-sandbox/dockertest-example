package model

import "time"

type Message struct {
	ID        string
	Sender    *User
	Body      string
	CreatedAt time.Time
}
