package ledger

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/user/fx-settlement-engine/internal/domain"
)

func TestCreateDoubleEntryLedger(t *testing.T) {
	s := NewService()

	tx := &domain.Transaction{
		ID:                uuid.New().String(),
		SenderAccountID:   "acc-sender-123",
		ReceiverAccountID: "acc-receiver-456",
		QuoteID:           "quote-789",
		FromAmount:        100.0,
		ToAmount:          92.0,
		FromCurrency:      "USD",
		ToCurrency:        "EUR",
		Status:            domain.TransactionCompleted,
		CreatedAt:         time.Now(),
	}

	debit, credit := s.CreateEntries(tx)

	if debit.EntryType != domain.EntryTypeDebit {
		t.Errorf("expected debit entry type, got %s", debit.EntryType)
	}
	if debit.AccountID != tx.SenderAccountID {
		t.Errorf("expected sender account ID %s, got %s", tx.SenderAccountID, debit.AccountID)
	}
	if debit.Amount != tx.FromAmount {
		t.Errorf("expected debit amount %.2f, got %.2f", tx.FromAmount, debit.Amount)
	}

	if credit.EntryType != domain.EntryTypeCredit {
		t.Errorf("expected credit entry type, got %s", credit.EntryType)
	}
	if credit.AccountID != tx.ReceiverAccountID {
		t.Errorf("expected receiver account ID %s, got %s", tx.ReceiverAccountID, credit.AccountID)
	}
	if credit.Amount != tx.ToAmount {
		t.Errorf("expected credit amount %.2f, got %.2f", tx.ToAmount, credit.Amount)
	}
}
