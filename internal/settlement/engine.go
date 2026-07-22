package settlement

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/user/fx-settlement-engine/internal/account"
	"github.com/user/fx-settlement-engine/internal/domain"
	"github.com/user/fx-settlement-engine/internal/fx"
	"github.com/user/fx-settlement-engine/internal/kafka"
	"github.com/user/fx-settlement-engine/internal/ledger"
	"github.com/user/fx-settlement-engine/internal/repository"
)

type Engine struct {
	repo          *repository.PostgresRepository
	accService    *account.Service
	fxService     *fx.Service
	ledgerService *ledger.Service
	producer      *kafka.EventProducer
}

func NewEngine(
	repo *repository.PostgresRepository,
	accService *account.Service,
	fxService *fx.Service,
	ledgerService *ledger.Service,
	producer *kafka.EventProducer,
) *Engine {
	return &Engine{
		repo:          repo,
		accService:    accService,
		fxService:     fxService,
		ledgerService: ledgerService,
		producer:      producer,
	}
}

type SettlementRequest struct {
	SenderAccountID   string
	ReceiverAccountID string
	QuoteID           string
	ReferenceID       string
}

func (e *Engine) ProcessSettlement(ctx context.Context, req *SettlementRequest) (*domain.Transaction, error) {
	if req.SenderAccountID == req.ReceiverAccountID {
		return nil, domain.ErrSameAccountTransfer
	}

	// 1. Fetch Sender Account
	sender, err := e.accService.GetAccount(ctx, req.SenderAccountID)
	if err != nil {
		return nil, fmt.Errorf("sender account error: %w", err)
	}

	// 2. Fetch Receiver Account
	receiver, err := e.accService.GetAccount(ctx, req.ReceiverAccountID)
	if err != nil {
		return nil, fmt.Errorf("receiver account error: %w", err)
	}

	var quote *domain.Quote
	if req.QuoteID != "" {
		// Validate locked quote
		q, err := e.fxService.ValidateQuote(ctx, req.QuoteID)
		if err != nil {
			return nil, fmt.Errorf("quote validation error: %w", err)
		}
		if q.FromCurrency != sender.Currency || q.ToCurrency != receiver.Currency {
			return nil, fmt.Errorf("currency mismatch between quote and accounts")
		}
		quote = q
	} else {
		// Instant quote if no quote_id passed
		return nil, fmt.Errorf("quote_id is required for settlement execution")
	}

	// 3. Check Sender Balance Pre-Validation
	if sender.Balance < quote.FromAmount {
		return nil, domain.ErrInsufficientBalance
	}

	// 4. Construct Transaction Record
	txRecord := &domain.Transaction{
		ID:                uuid.New().String(),
		SenderAccountID:   sender.ID,
		ReceiverAccountID: receiver.ID,
		QuoteID:           quote.ID,
		FromAmount:        quote.FromAmount,
		ToAmount:          quote.ToAmount,
		FromCurrency:      sender.Currency,
		ToCurrency:        receiver.Currency,
		Status:            domain.TransactionCompleted,
		CreatedAt:         time.Now().UTC(),
	}

	// 5. Generate Double-Entry Ledger Entries
	debitEntry, creditEntry := e.ledgerService.CreateEntries(txRecord)

	// 6. Execute Atomic PostgreSQL Transaction (Debits, Credits, Ledger, Quote Status)
	if err := e.repo.ExecuteSettlementTx(ctx, txRecord, debitEntry, creditEntry); err != nil {
		return nil, fmt.Errorf("settlement transaction failed: %w", err)
	}

	// 7. Publish Async Audit Event to Kafka
	if e.producer != nil {
		auditEvent := &domain.AuditEvent{
			EventID:       uuid.New().String(),
			EventType:     "SETTLEMENT_COMPLETED",
			TransactionID: txRecord.ID,
			SenderID:      sender.ID,
			ReceiverID:    receiver.ID,
			FromAmount:    txRecord.FromAmount,
			ToAmount:      txRecord.ToAmount,
			FromCurrency:  txRecord.FromCurrency,
			ToCurrency:    txRecord.ToCurrency,
			Status:        string(txRecord.Status),
			Timestamp:     txRecord.CreatedAt,
		}
		_ = e.producer.PublishAuditEvent(ctx, auditEvent)
	}

	return txRecord, nil
}

func (e *Engine) GetTransaction(ctx context.Context, id string) (*domain.Transaction, error) {
	return e.repo.GetTransactionByID(ctx, id)
}
