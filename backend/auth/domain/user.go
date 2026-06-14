package domain

import (
	"time"

	"github.com/Sephy314/chinwag/auth/structs"
)

type User struct {
	Id        string
	Name      string
	Email     string
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (u User) ToProjection() structs.UserResponse {
	return structs.UserResponse{
		Id:    u.Id,
		Name:  u.Name,
		Email: u.Email,
	}
}
