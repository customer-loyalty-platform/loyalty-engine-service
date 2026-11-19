package service

import (
	"context"
	"strings"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/balance/model"
	"loyalty-engine-service/internal/modules/balance/repository"

	"go.uber.org/zap"
)

type BalanceService interface {
	GetBalance(ctx context.Context, memberID string) (*model.MemberBalance, error)
	GetLedgerHistory(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]model.PointLedgerRow, int64, error)
	GetExpiringPoints(ctx context.Context, memberID string, limit int) ([]model.ExpiringBucketRow, error)
	GetProfile(ctx context.Context, memberID string) (*model.MemberProfile, error)
	RegisterMember(ctx context.Context, req model.RegisterMemberRequest) (*model.RegisterMemberResponse, error)
}

type balanceService struct {
	repo repository.BalanceRepository
	log  *zap.Logger
}

func NewBalanceService(repo repository.BalanceRepository, log *zap.Logger) BalanceService {
	return &balanceService{
		repo: repo,
		log:  log,
	}
}

func (s *balanceService) GetBalance(ctx context.Context, memberID string) (*model.MemberBalance, error) {
	if strings.TrimSpace(memberID) == "" {
		return nil, domain.ErrBadRequest
	}

	bal, err := s.repo.GetBalanceByMemberID(ctx, memberID)
	if err != nil {
		s.log.Error("failed to get member balance", zap.String("member_id", memberID), zap.Error(err))
		return nil, err
	}

	return bal, nil
}

func (s *balanceService) GetLedgerHistory(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]model.PointLedgerRow, int64, error) {
	if strings.TrimSpace(memberID) == "" {
		return nil, 0, domain.ErrBadRequest
	}

	if pagination.Limit <= 0 {
		pagination.Limit = 10
	}
	if pagination.Page <= 0 {
		pagination.Page = 1
	}

	rows, total, err := s.repo.GetLedgerByMemberID(ctx, memberID, pagination)
	if err != nil {
		s.log.Error("failed to get point ledgers", zap.String("member_id", memberID), zap.Error(err))
		return nil, 0, err
	}

	return rows, total, nil
}

func (s *balanceService) GetExpiringPoints(ctx context.Context, memberID string, limit int) ([]model.ExpiringBucketRow, error) {
	if strings.TrimSpace(memberID) == "" {
		return nil, domain.ErrBadRequest
	}

	buckets, err := s.repo.GetExpiringBucketsByMemberID(ctx, memberID, limit)
	if err != nil {
		s.log.Error("failed to get expiring points buckets", zap.String("member_id", memberID), zap.Error(err))
		return nil, err
	}

	return buckets, nil
}

func (s *balanceService) GetProfile(ctx context.Context, memberID string) (*model.MemberProfile, error) {
	if strings.TrimSpace(memberID) == "" {
		return nil, domain.ErrBadRequest
	}

	profile, err := s.repo.GetMemberProfile(ctx, memberID)
	if err != nil {
		s.log.Error("failed to get member profile", zap.String("member_id", memberID), zap.Error(err))
		return nil, err
	}

	return profile, nil
}

func (s *balanceService) RegisterMember(ctx context.Context, req model.RegisterMemberRequest) (*model.RegisterMemberResponse, error) {
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)
	if req.AccountNumber == "" {
		return nil, domain.ErrBadRequest
	}

	resp, err := s.repo.RegisterMember(ctx, req.AccountNumber, req.TierCode)
	if err != nil {
		s.log.Error("failed to register member", zap.String("account_number", req.AccountNumber), zap.Error(err))
		return nil, err
	}

	s.log.Info("member registered successfully",
		zap.String("member_id", resp.MemberID),
		zap.String("account_number", resp.AccountNumber),
		zap.String("tier_code", resp.TierCode),
	)

	return resp, nil
}

