package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"alpha/internal/interfaces"
	ierr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"
	rsql "alpha/internal/objects/sql"

	"github.com/google/uuid"
)

type invitationRepository struct {
	db *sql.DB
}

func NewInvitationRepository(db *sql.DB) interfaces.InvitationInterface {
	return &invitationRepository{db: db}
}

func (r *invitationRepository) Create(ctx context.Context, inv *models.Invitation) error {
	_, err := r.db.ExecContext(
		ctx, rsql.CreateInvitation,
		inv.ID,
		inv.Code,
		inv.CreatedBy,
		inv.ExpiresAt,
		inv.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

func (r *invitationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Invitation, error) {
	return r.getOne(ctx, rsql.GetInvitationByID, id)
}

func (r *invitationRepository) GetByCode(ctx context.Context, code string) (*models.Invitation, error) {
	return r.getOne(ctx, rsql.GetInvitationByCode, code)
}

func (r *invitationRepository) GetByCreator(ctx context.Context, userID uuid.UUID) ([]*models.Invitation, error) {
	rows, err := r.db.QueryContext(ctx, rsql.GetInvitationsByCreator, userID)
	if err != nil {
		return nil, fmt.Errorf("get invitations by creator: %w", err)
	}
	defer rows.Close()

	var result []*models.Invitation
	for rows.Next() {
		inv, err := r.scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, inv)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invitations: %w", err)
	}
	return result, nil
}

func (r *invitationRepository) Use(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, rsql.UseInvitation, userID, id)
	if err != nil {
		return fmt.Errorf("use invitation: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ierr.ErrInvitationUsed
	}
	return nil
}

func (r *invitationRepository) ClearExpiredReservations(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, rsql.ClearExpiredReservations)
	if err != nil {
		return 0, fmt.Errorf("clear expired reservations: %w", err)
	}
	return res.RowsAffected()
}

func (r *invitationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, rsql.DeleteInvitation, id)
	if err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ierr.ErrInvitationUsed
	}
	return nil
}

func (r *invitationRepository) getOne(ctx context.Context, query string, args ...any) (*models.Invitation, error) {
	row := r.db.QueryRowContext(ctx, query, args...)

	inv := &models.Invitation{}
	err := row.Scan(
		&inv.ID,
		&inv.Code,
		&inv.CreatedBy,
		&inv.UsedBy,
		&inv.UsedAt,
		&inv.ExpiresAt,
		&inv.CreatedAt,
		&inv.ReservedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ierr.ErrInvitationNotFound
		}
		return nil, fmt.Errorf("scan invitation: %w", err)
	}

	return inv, nil
}

func (r *invitationRepository) scanInvitation(rows *sql.Rows) (*models.Invitation, error) {
	inv := &models.Invitation{}
	err := rows.Scan(
		&inv.ID, &inv.Code, &inv.CreatedBy, &inv.UsedBy, &inv.UsedAt,
		&inv.ExpiresAt, &inv.CreatedAt, &inv.ReservedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ierr.ErrInvitationNotFound
		}
		return nil, fmt.Errorf("scan invitation: %w", err)
	}
	return inv, nil
}

func (r *invitationRepository) Update(ctx context.Context, invite *models.Invitation) error {
	res, err := r.db.ExecContext(ctx, rsql.UpdateInvitation,
		invite.UsedBy,
		invite.UsedAt,
		invite.ExpiresAt,
		invite.ReservedAt,
		invite.ID,
	)
	if err != nil {
		return fmt.Errorf("update invitation: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ierr.ErrInvitationNotFound
	}
	return nil
}

func (r *invitationRepository) MarkAsUsed(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, rsql.MarkInvitationAsUsed, userID, id)
	if err != nil {
		return fmt.Errorf("mark as used: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ierr.ErrInvitationNotFound
	}
	return nil
}
