package ledger

import (
	"time"

	"github.com/google/uuid"
	"github.com/user/fx-settlement-engine/internal/domain"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) CreateEntries(tx *domain.Transaction) (*domain.LedgerEntry, *domain.LedgerEntry) {
	now := time.Now().UTC()

	debit := &domain.LedgerEntry{
		ID:            uuid.New().String(),
		TransactionID: tx.ID,
		AccountID:     tx.SenderAccountID,
		EntryType:     domain.EntryTypeDebit,
		Amount:        tx.FromAmount,
		Currency:      tx.FromCurrency,
		CreatedAt:     now,
	}

	credit := &domain.LedgerEntry{
		ID:            uuid.New().String(),
		TransactionID: tx.ID,
		AccountID:     tx.ReceiverAccountID,
		EntryType:     domain.EntryTypeCredit,
		Amount:        tx.ToAmount,
		Currency:      tx.ToCurrency,
		CreatedAt:     now,
	}

	return debit, credit
}
