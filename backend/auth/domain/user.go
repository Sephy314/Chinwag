package domain

import (
	"time"

	"github.com/Sephy314/chinwag/auth/structs"
)

type User struct {
	Id        string     `db:"id"`
	Name      string     `db:"name"`
	Email     string     `db:"email"`
	Password  string     `db:"password"`
	Role      Role       `db:"role"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (u User) ToProjection() structs.UserResponse {
	return structs.UserResponse{
		Id:    u.Id,
		Name:  u.Name,
		Email: u.Email,
	}
}

type Role string

const (
	USER    Role = "USER"
	MANAGER Role = "MANAGER"
	ADMIN   Role = "ADMIN"
)
