package structs

type CreateUserReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginReq represents the payload for login requests
type LoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
