package account

import (
	"context"
	"testing"

	"github.com/user/fx-settlement-engine/internal/domain"
)

func TestAccountValidation(t *testing.T) {
	s := NewService(nil, nil)
	ctx := context.Background()

	t.Run("Empty Owner Name", func(t *testing.T) {
		_, err := s.CreateAccount(ctx, "", "USD", 100)
		if err == nil {
			t.Errorf("expected error for empty owner name, got nil")
		}
	})

	t.Run("Empty Currency", func(t *testing.T) {
		_, err := s.CreateAccount(ctx, "Alice", "", 100)
		if err == nil {
			t.Errorf("expected error for empty currency, got nil")
		}
	})

	t.Run("Negative Initial Balance", func(t *testing.T) {
		_, err := s.CreateAccount(ctx, "Alice", "USD", -50)
		if err != domain.ErrInvalidAmount {
			t.Errorf("expected ErrInvalidAmount, got %v", err)
		}
	})
}
