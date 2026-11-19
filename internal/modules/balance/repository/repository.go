package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/balance/model"
	"loyalty-engine-service/internal/pkg/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type BalanceRepository interface {
	GetBalanceByMemberID(ctx context.Context, memberID string) (*model.MemberBalance, error)
	GetLedgerByMemberID(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]model.PointLedgerRow, int64, error)
	GetExpiringBucketsByMemberID(ctx context.Context, memberID string, limit int) ([]model.ExpiringBucketRow, error)
	GetMemberProfile(ctx context.Context, memberID string) (*model.MemberProfile, error)
	RegisterMember(ctx context.Context, accountNumber string, tierCode string) (*model.RegisterMemberResponse, error)
}

type balanceRepository struct {
	db *database.PostgresDB
}

func NewBalanceRepository(db *database.PostgresDB) BalanceRepository {
	return &balanceRepository{db: db}
}

func (r *balanceRepository) GetBalanceByMemberID(ctx context.Context, memberID string) (*model.MemberBalance, error) {
	query := `
		SELECT member_id, total_points, locked_points, expiring_points, version, COALESCE(updated_at, NOW())
		FROM schema_ledger.member_balances
		WHERE member_id = $1`

	var b model.MemberBalance
	err := r.db.Replica.QueryRow(ctx, query, memberID).Scan(
		&b.MemberID,
		&b.TotalPoints,
		&b.LockedPoints,
		&b.ExpiringPoints,
		&b.Version,
		&b.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query member balance: %w", err)
	}

	b.AvailablePoints = b.TotalPoints - b.LockedPoints
	if b.AvailablePoints < 0 {
		b.AvailablePoints = 0
	}

	return &b, nil
}

func (r *balanceRepository) GetLedgerByMemberID(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]model.PointLedgerRow, int64, error) {
	countQuery := `SELECT COUNT(*) FROM schema_ledger.point_ledgers WHERE member_id = $1`

	var total int64
	err := r.db.Replica.QueryRow(ctx, countQuery, memberID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count point ledgers: %w", err)
	}

	if total == 0 {
		return []model.PointLedgerRow{}, 0, nil
	}

	dataQuery := `
		SELECT id, member_id, type, amount, balance_after, reference_id, idempotency_key, created_at
		FROM schema_ledger.point_ledgers
		WHERE member_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Replica.Query(ctx, dataQuery, memberID, pagination.Limit, pagination.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query point ledgers: %w", err)
	}
	defer rows.Close()

	var ledgers []model.PointLedgerRow
	for rows.Next() {
		var row model.PointLedgerRow
		if err := rows.Scan(
			&row.ID,
			&row.MemberID,
			&row.Type,
			&row.Amount,
			&row.BalanceAfter,
			&row.ReferenceID,
			&row.IdempotencyKey,
			&row.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan ledger row: %w", err)
		}
		ledgers = append(ledgers, row)
	}

	return ledgers, total, nil
}

func (r *balanceRepository) GetExpiringBucketsByMemberID(ctx context.Context, memberID string, limit int) ([]model.ExpiringBucketRow, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, member_id, original_points, remaining_points, earned_at, expires_at, expiry_policy, status
		FROM schema_earning.point_expiry_buckets
		WHERE member_id = $1
		  AND status IN ('ACTIVE', 'PARTIALLY_CONSUMED')
		  AND expires_at > NOW()
		ORDER BY expires_at ASC
		LIMIT $2`

	rows, err := r.db.Replica.Query(ctx, query, memberID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query expiring buckets: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	var buckets []model.ExpiringBucketRow
	for rows.Next() {
		var b model.ExpiringBucketRow
		if err := rows.Scan(
			&b.ID,
			&b.MemberID,
			&b.OriginalPoints,
			&b.RemainingPoints,
			&b.EarnedAt,
			&b.ExpiresAt,
			&b.ExpiryPolicy,
			&b.Status,
		); err != nil {
			return nil, fmt.Errorf("failed to scan expiring bucket: %w", err)
		}

		diff := b.ExpiresAt.Sub(now)
		days := int(math.Ceil(diff.Hours() / 24))
		if days < 0 {
			days = 0
		}
		b.DaysRemaining = days

		buckets = append(buckets, b)
	}

	if buckets == nil {
		buckets = []model.ExpiringBucketRow{}
	}

	return buckets, nil
}

func (r *balanceRepository) GetMemberProfile(ctx context.Context, memberID string) (*model.MemberProfile, error) {
	query := `
		SELECT m.id, m.account_number, COALESCE(m.status, 'ACTIVE'), COALESCE(m.created_at, NOW()),
		       t.id, t.tier_code, t.tier_name, t.multiplier, 
		       COALESCE(t.color_hex, ''), COALESCE(t.icon_url, ''), COALESCE(t.perks, '[]'::jsonb),
		       COALESCE(b.total_points, 0.00), COALESCE(b.locked_points, 0.00), 
		       COALESCE(b.expiring_points, 0.00), COALESCE(b.version, 1), COALESCE(b.updated_at, NOW())
		FROM schema_ledger.members m
		LEFT JOIN schema_backoffice.tier_definitions t ON m.current_tier_id = t.id
		LEFT JOIN schema_ledger.member_balances b ON m.id = b.member_id
		WHERE m.id = $1`

	var (
		p               model.MemberProfile
		tierID          *string
		tierCode        *string
		tierName        *string
		multiplier      *float64
		colorHex        *string
		iconURL         *string
		perksBytes      []byte
		totalPoints     float64
		lockedPoints    float64
		expiringPoints  float64
		balanceVersion  int
		balanceUpdated  time.Time
	)

	err := r.db.Replica.QueryRow(ctx, query, memberID).Scan(
		&p.MemberID,
		&p.AccountNumber,
		&p.Status,
		&p.CreatedAt,
		&tierID,
		&tierCode,
		&tierName,
		&multiplier,
		&colorHex,
		&iconURL,
		&perksBytes,
		&totalPoints,
		&lockedPoints,
		&expiringPoints,
		&balanceVersion,
		&balanceUpdated,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query member profile: %w", err)
	}

	// Populate Tier if present
	if tierID != nil {
		p.Tier = &model.TierInfo{
			TierID:     *tierID,
			TierCode:   *tierCode,
			TierName:   *tierName,
			Multiplier: *multiplier,
			ColorHex:   *colorHex,
			IconURL:    *iconURL,
			Perks:      perksBytes,
		}
	}

	// Populate Balance
	avail := totalPoints - lockedPoints
	if avail < 0 {
		avail = 0
	}
	p.Balance = &model.MemberBalance{
		MemberID:        p.MemberID,
		TotalPoints:     totalPoints,
		LockedPoints:    lockedPoints,
		ExpiringPoints:  expiringPoints,
		AvailablePoints: avail,
		Version:         balanceVersion,
		UpdatedAt:       balanceUpdated,
	}

	return &p, nil
}

func (r *balanceRepository) RegisterMember(ctx context.Context, accountNumber string, tierCode string) (*model.RegisterMemberResponse, error) {
	tx, err := r.db.Master.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start registration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Resolve Tier ID
	var resolvedTierID string
	var resolvedTierCode string

	if tierCode != "" {
		err := tx.QueryRow(ctx, `
			SELECT id, tier_code 
			FROM schema_backoffice.tier_definitions 
			WHERE tier_code = $1 LIMIT 1`, tierCode).Scan(&resolvedTierID, &resolvedTierCode)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("specified tier code %s not found: %w", tierCode, domain.ErrNotFound)
			}
			return nil, err
		}
	} else {
		// Default to base tier with lowest sort order or 0 min_points
		err := tx.QueryRow(ctx, `
			SELECT id, tier_code 
			FROM schema_backoffice.tier_definitions 
			ORDER BY sort_order ASC, min_points ASC 
			LIMIT 1`).Scan(&resolvedTierID, &resolvedTierCode)
		if err != nil {
			return nil, fmt.Errorf("no default tier definition found in system: %w", err)
		}
	}

	// 2. Insert into schema_ledger.members
	var memberID string
	var createdAt time.Time
	var status string

	insertMemberQuery := `
		INSERT INTO schema_ledger.members (account_number, current_tier_id, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', NOW(), NOW())
		RETURNING id, status, created_at`

	err = tx.QueryRow(ctx, insertMemberQuery, accountNumber, resolvedTierID).Scan(&memberID, &status, &createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("account number %s already exists: %w", accountNumber, domain.ErrConflict)
		}
		return nil, fmt.Errorf("failed to insert member: %w", err)
	}

	// 3. Insert into schema_ledger.member_balances
	insertBalanceQuery := `
		INSERT INTO schema_ledger.member_balances (member_id, total_points, locked_points, expiring_points, version, updated_at)
		VALUES ($1, 0.00, 0.00, 0.00, 1, NOW())`

	_, err = tx.Exec(ctx, insertBalanceQuery, memberID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize member balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit registration transaction: %w", err)
	}

	return &model.RegisterMemberResponse{
		MemberID:      memberID,
		AccountNumber: accountNumber,
		Status:        status,
		TierCode:      resolvedTierCode,
		CreatedAt:     createdAt,
	}, nil
}

