package domain

import "time"

type User struct {
	Id        string
	Name      string
	Email     string
	Password  string
	CreatedAt time.Time
	DeletedAt *time.Time
}
