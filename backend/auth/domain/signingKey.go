package domain

import (
	"crypto/ecdsa"
	"time"
)

type SigningKeyEntity struct {
	Kid        string     `db:"kid"`
	PublicKey  string     `db:"public_key"`
	PrivateKey string     `db:"private_key"`
	Status     KeyStatus  `db:"status"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  *time.Time `db:"updated_at"`
	ExpiredAt  *time.Time `db:"expired_at"`
}

type SigningKey struct {
	Kid        string            `db:"kid"`
	PublicKey  *ecdsa.PublicKey  `db:"public_key"`
	PrivateKey *ecdsa.PrivateKey `db:"private_key"`
	Status     KeyStatus         `db:"status"`
	CreatedAt  time.Time         `db:"created_at"`
	UpdatedAt  *time.Time        `db:"updated_at"`
	ExpiredAt  *time.Time        `db:"expired_at"`
}

type KeyStatus string

const (
	Active   KeyStatus = "ACTIVE"
	Inactive KeyStatus = "INACTIVE"
	Expired  KeyStatus = "EXPIRED"
)

func (s KeyStatus) IncludeInJWKS() bool {
	switch s {
	case Active, Inactive:
		return true
	default:
		return false // Retired
	}
}
