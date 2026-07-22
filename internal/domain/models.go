package domain

import (
	"errors"
	"time"
)

var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrInsufficientBalance = errors.New("insufficient account balance")
	ErrInvalidAmount       = errors.New("amount must be greater than zero")
	ErrSameAccountTransfer = errors.New("sender and receiver accounts must be different")
	ErrRateNotFound        = errors.New("exchange rate not found for currency pair")
	ErrQuoteNotFound       = errors.New("quote not found")
	ErrQuoteExpired        = errors.New("quote has expired")
	ErrQuoteAlreadyUsed    = errors.New("quote has already been used")
	ErrTransactionNotFound = errors.New("transaction not found")
)

type Account struct {
	ID        string    `json:"id"`
	OwnerName string    `json:"owner_name"`
	Currency  string    `json:"currency"`
	Balance   float64   `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ExchangeRate struct {
	ID           string    `json:"id"`
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	Rate         float64   `json:"rate"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type QuoteStatus string

const (
	QuoteStatusLocked  QuoteStatus = "LOCKED"
	QuoteStatusExpired QuoteStatus = "EXPIRED"
	QuoteStatusUsed    QuoteStatus = "USED"
)

type Quote struct {
	ID           string      `json:"id"`
	FromCurrency string      `json:"from_currency"`
	ToCurrency   string      `json:"to_currency"`
	Rate         float64     `json:"rate"`
	FromAmount   float64     `json:"from_amount"`
	ToAmount     float64     `json:"to_amount"`
	ExpiresAt    time.Time   `json:"expires_at"`
	Status       QuoteStatus `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
}

type TransactionStatus string

const (
	TransactionPending   TransactionStatus = "PENDING"
	TransactionCompleted TransactionStatus = "COMPLETED"
	TransactionFailed    TransactionStatus = "FAILED"
)

type Transaction struct {
	ID                string            `json:"id"`
	SenderAccountID   string            `json:"sender_account_id"`
	ReceiverAccountID string            `json:"receiver_account_id"`
	QuoteID           string            `json:"quote_id"`
	FromAmount        float64           `json:"from_amount"`
	ToAmount          float64           `json:"to_amount"`
	FromCurrency      string            `json:"from_currency"`
	ToCurrency        string            `json:"to_currency"`
	Status            TransactionStatus `json:"status"`
	CreatedAt         time.Time         `json:"created_at"`
}

type EntryType string

const (
	EntryTypeDebit  EntryType = "DEBIT"
	EntryTypeCredit EntryType = "CREDIT"
)

type LedgerEntry struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	EntryType     EntryType `json:"entry_type"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
}

type AuditEvent struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	TransactionID string    `json:"transaction_id"`
	SenderID      string    `json:"sender_id"`
	ReceiverID    string    `json:"receiver_id"`
	FromAmount    float64   `json:"from_amount"`
	ToAmount      float64   `json:"to_amount"`
	FromCurrency  string    `json:"from_currency"`
	ToCurrency    string    `json:"to_currency"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
}
