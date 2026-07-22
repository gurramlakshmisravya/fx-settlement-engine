package account

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/user/fx-settlement-engine/internal/cache"
	"github.com/user/fx-settlement-engine/internal/domain"
	"github.com/user/fx-settlement-engine/internal/repository"
)

type Service struct {
	repo  *repository.PostgresRepository
	cache *cache.RedisCache
	mu    sync.Mutex
}

func NewService(repo *repository.PostgresRepository, cache *cache.RedisCache) *Service {
	return &Service{
		repo:  repo,
		cache: cache,
	}
}

func (s *Service) CreateAccount(ctx context.Context, ownerName, currency string, initialBalance float64) (*domain.Account, error) {
	if ownerName == "" {
		return nil, fmt.Errorf("owner_name cannot be empty")
	}
	if currency == "" {
		return nil, fmt.Errorf("currency cannot be empty")
	}
	if initialBalance < 0 {
		return nil, domain.ErrInvalidAmount
	}

	acc, err := s.repo.CreateAccount(ctx, ownerName, currency, initialBalance)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.SetAccount(ctx, acc, 5*time.Minute)
	}

	return acc, nil
}

func (s *Service) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	if s.cache != nil {
		cachedAcc, err := s.cache.GetAccount(ctx, id)
		if err == nil && cachedAcc != nil {
			return cachedAcc, nil
		}
	}

	acc, err := s.repo.GetAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.SetAccount(ctx, acc, 5*time.Minute)
	}

	return acc, nil
}

func (s *Service) UpdateBalance(ctx context.Context, id string, amountDelta float64) (*domain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, err := s.repo.UpdateBalance(ctx, id, amountDelta)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.InvalidateAccount(ctx, id)
	}

	return acc, nil
}
