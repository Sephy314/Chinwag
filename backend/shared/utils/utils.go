package utils

import "regexp"

func IsEmail(email string) bool {
	var emailRegex = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)

	return emailRegex.MatchString(email)
}
