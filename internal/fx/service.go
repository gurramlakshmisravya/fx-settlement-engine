package fx

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/user/fx-settlement-engine/internal/cache"
	"github.com/user/fx-settlement-engine/internal/domain"
	"github.com/user/fx-settlement-engine/internal/repository"
)

type Service struct {
	repo  *repository.PostgresRepository
	cache *cache.RedisCache
}

func NewService(repo *repository.PostgresRepository, cache *cache.RedisCache) *Service {
	return &Service{
		repo:  repo,
		cache: cache,
	}
}

func (s *Service) GetRate(ctx context.Context, fromCurrency, toCurrency string) (*domain.ExchangeRate, error) {
	return s.repo.GetExchangeRate(ctx, fromCurrency, toCurrency)
}

func (s *Service) LockQuote(ctx context.Context, fromCurrency, toCurrency string, amount float64, ttlSeconds int) (*domain.Quote, error) {
	if amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	rate, err := s.GetRate(ctx, fromCurrency, toCurrency)
	if err != nil {
		return nil, err
	}

	if ttlSeconds <= 0 {
		ttlSeconds = 60 // Default 60 seconds TTL
	}

	convertedAmount := math.Round(amount*rate.Rate*10000) / 10000
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttlSeconds) * time.Second)

	quote := &domain.Quote{
		ID:           uuid.New().String(),
		FromCurrency: fromCurrency,
		ToCurrency:   toCurrency,
		Rate:         rate.Rate,
		FromAmount:   amount,
		ToAmount:     convertedAmount,
		ExpiresAt:    expiresAt,
		Status:       domain.QuoteStatusLocked,
		CreatedAt:    now,
	}

	// Persist in DB
	if err := s.repo.SaveQuote(ctx, quote); err != nil {
		return nil, err
	}

	// Cache in Redis with matching TTL
	if s.cache != nil {
		_ = s.cache.SetQuote(ctx, quote, time.Duration(ttlSeconds)*time.Second)
	}

	return quote, nil
}

func (s *Service) ValidateQuote(ctx context.Context, quoteID string) (*domain.Quote, error) {
	// Try Redis cache first
	if s.cache != nil {
		cachedQuote, err := s.cache.GetQuote(ctx, quoteID)
		if err == nil && cachedQuote != nil {
			if time.Now().UTC().After(cachedQuote.ExpiresAt) {
				return nil, domain.ErrQuoteExpired
			}
			if cachedQuote.Status != domain.QuoteStatusLocked {
				return nil, domain.ErrQuoteAlreadyUsed
			}
			return cachedQuote, nil
		}
	}

	quote, err := s.repo.GetQuoteByID(ctx, quoteID)
	if err != nil {
		return nil, err
	}

	if time.Now().UTC().After(quote.ExpiresAt) {
		return nil, domain.ErrQuoteExpired
	}

	if quote.Status != domain.QuoteStatusLocked {
		return nil, domain.ErrQuoteAlreadyUsed
	}

	return quote, nil
}
