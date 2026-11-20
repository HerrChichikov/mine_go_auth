package models

import (
	"time"

	"github.com/google/uuid"
)

type Invitation struct {
	ID                 uuid.UUID  `db:"id" json:"id"`
	Code               string     `db:"code" json:"code"`
	CreatedBy          uuid.UUID  `db:"created_by" json:"created_by"`
	UsedBy             *uuid.UUID `db:"used_by" json:"used_by"`
	UsedAt             *time.Time `db:"used_at" json:"used_at"`
	ExpiresAt          *time.Time `db:"expires_at" json:"expires_at"`
	CreatedAt          time.Time  `db:"created_at" json:"created_at"`
	ReservedAt         *time.Time `db:"reserved_at" json:"-"`
}

func (i *Invitation) IsUsed() bool {
	return i.UsedBy != nil
}

func (i *Invitation) IsExpired() bool {
	if i.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*i.ExpiresAt)
}

func (i *Invitation) IsReserved() bool {
	return i.ReservedAt != nil
}

func (i *Invitation) IsValid() bool {
	return !i.IsUsed() && !i.IsExpired() && !i.IsReserved()
}

func (i *Invitation) MarkAsUsed(userID uuid.UUID) {
	now := time.Now()
	i.UsedBy = &userID
	i.UsedAt = &now
	i.ReservedAt = nil
}

func (i *Invitation) Reserve() {
	now := time.Now()
	i.ReservedAt = &now
}

func (i *Invitation) ClearReservation() {
	i.ReservedAt = nil
}