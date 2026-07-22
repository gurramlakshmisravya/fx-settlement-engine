package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/user/fx-settlement-engine/internal/domain"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Account Operations
func (r *PostgresRepository) CreateAccount(ctx context.Context, ownerName, currency string, initialBalance float64) (*domain.Account, error) {
	acc := &domain.Account{
		ID:        uuid.New().String(),
		OwnerName: ownerName,
		Currency:  currency,
		Balance:   initialBalance,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	query := `
		INSERT INTO accounts (id, owner_name, currency, balance, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query, acc.ID, acc.OwnerName, acc.Currency, acc.Balance, acc.CreatedAt, acc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}
	return acc, nil
}

func (r *PostgresRepository) GetAccountByID(ctx context.Context, id string) (*domain.Account, error) {
	query := `SELECT id, owner_name, currency, balance, created_at, updated_at FROM accounts WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	acc := &domain.Account{}
	err := row.Scan(&acc.ID, &acc.OwnerName, &acc.Currency, &acc.Balance, &acc.CreatedAt, &acc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	return acc, nil
}

func (r *PostgresRepository) UpdateBalance(ctx context.Context, id string, amountDelta float64) (*domain.Account, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	acc, err := r.getAccountForUpdateTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	newBalance := acc.Balance + amountDelta
	if newBalance < 0 {
		return nil, domain.ErrInsufficientBalance
	}

	acc.Balance = newBalance
	acc.UpdatedAt = time.Now().UTC()

	_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance = $1, updated_at = $2 WHERE id = $3`, acc.Balance, acc.UpdatedAt, acc.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update account balance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return acc, nil
}

func (r *PostgresRepository) getAccountForUpdateTx(ctx context.Context, tx *sql.Tx, id string) (*domain.Account, error) {
	query := `SELECT id, owner_name, currency, balance, created_at, updated_at FROM accounts WHERE id = $1 FOR UPDATE`
	row := tx.QueryRowContext(ctx, query, id)

	acc := &domain.Account{}
	err := row.Scan(&acc.ID, &acc.OwnerName, &acc.Currency, &acc.Balance, &acc.CreatedAt, &acc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account for update: %w", err)
	}
	return acc, nil
}

// Exchange Rate Operations
func (r *PostgresRepository) GetExchangeRate(ctx context.Context, fromCurrency, toCurrency string) (*domain.ExchangeRate, error) {
	if fromCurrency == toCurrency {
		return &domain.ExchangeRate{
			ID:           uuid.New().String(),
			FromCurrency: fromCurrency,
			ToCurrency:   toCurrency,
			Rate:         1.0,
			UpdatedAt:    time.Now().UTC(),
		}, nil
	}

	query := `SELECT id, from_currency, to_currency, rate, updated_at FROM exchange_rates WHERE from_currency = $1 AND to_currency = $2`
	row := r.db.QueryRowContext(ctx, query, fromCurrency, toCurrency)

	rate := &domain.ExchangeRate{}
	err := row.Scan(&rate.ID, &rate.FromCurrency, &rate.ToCurrency, &rate.Rate, &rate.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrRateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange rate: %w", err)
	}
	return rate, nil
}

// Quote Operations
func (r *PostgresRepository) SaveQuote(ctx context.Context, quote *domain.Quote) error {
	query := `
		INSERT INTO quotes (id, from_currency, to_currency, rate, from_amount, to_amount, expires_at, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		quote.ID, quote.FromCurrency, quote.ToCurrency, quote.Rate,
		quote.FromAmount, quote.ToAmount, quote.ExpiresAt, quote.Status, quote.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save quote: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetQuoteByID(ctx context.Context, id string) (*domain.Quote, error) {
	query := `SELECT id, from_currency, to_currency, rate, from_amount, to_amount, expires_at, status, created_at FROM quotes WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	q := &domain.Quote{}
	err := row.Scan(&q.ID, &q.FromCurrency, &q.ToCurrency, &q.Rate, &q.FromAmount, &q.ToAmount, &q.ExpiresAt, &q.Status, &q.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrQuoteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}
	return q, nil
}

func (r *PostgresRepository) MarkQuoteUsed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE quotes SET status = $1 WHERE id = $2`, domain.QuoteStatusUsed, id)
	return err
}

// Atomic Settlement Execution
func (r *PostgresRepository) ExecuteSettlementTx(
	ctx context.Context,
	txRecord *domain.Transaction,
	debitEntry *domain.LedgerEntry,
	creditEntry *domain.LedgerEntry,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Lock Sender Account & Check Balance
	sender, err := r.getAccountForUpdateTx(ctx, tx, txRecord.SenderAccountID)
	if err != nil {
		return err
	}
	if sender.Balance < txRecord.FromAmount {
		return domain.ErrInsufficientBalance
	}

	// 2. Lock Receiver Account
	receiver, err := r.getAccountForUpdateTx(ctx, tx, txRecord.ReceiverAccountID)
	if err != nil {
		return err
	}

	// 3. Update Sender & Receiver Balances
	newSenderBalance := sender.Balance - txRecord.FromAmount
	newReceiverBalance := receiver.Balance + txRecord.ToAmount
	now := time.Now().UTC()

	_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance = $1, updated_at = $2 WHERE id = $3`, newSenderBalance, now, sender.ID)
	if err != nil {
		return fmt.Errorf("failed to debit sender: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance = $1, updated_at = $2 WHERE id = $3`, newReceiverBalance, now, receiver.ID)
	if err != nil {
		return fmt.Errorf("failed to credit receiver: %w", err)
	}

	// 4. Save Transaction Record
	queryTx := `
		INSERT INTO transactions (id, sender_account_id, receiver_account_id, quote_id, from_amount, to_amount, from_currency, to_currency, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = tx.ExecContext(ctx, queryTx,
		txRecord.ID, txRecord.SenderAccountID, txRecord.ReceiverAccountID, txRecord.QuoteID,
		txRecord.FromAmount, txRecord.ToAmount, txRecord.FromCurrency, txRecord.ToCurrency,
		txRecord.Status, txRecord.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save transaction: %w", err)
	}

	// 5. Save Immutable Ledger Entries (Double-Entry)
	queryLedger := `
		INSERT INTO ledger (id, transaction_id, account_id, entry_type, amount, currency, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.ExecContext(ctx, queryLedger,
		debitEntry.ID, debitEntry.TransactionID, debitEntry.AccountID,
		debitEntry.EntryType, debitEntry.Amount, debitEntry.Currency, debitEntry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save debit ledger entry: %w", err)
	}

	_, err = tx.ExecContext(ctx, queryLedger,
		creditEntry.ID, creditEntry.TransactionID, creditEntry.AccountID,
		creditEntry.EntryType, creditEntry.Amount, creditEntry.Currency, creditEntry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save credit ledger entry: %w", err)
	}

	// 6. Update Quote status if quote_id present
	if txRecord.QuoteID != "" {
		_, err = tx.ExecContext(ctx, `UPDATE quotes SET status = $1 WHERE id = $2`, domain.QuoteStatusUsed, txRecord.QuoteID)
		if err != nil {
			return fmt.Errorf("failed to mark quote as used: %w", err)
		}
	}

	return tx.Commit()
}

func (r *PostgresRepository) GetTransactionByID(ctx context.Context, id string) (*domain.Transaction, error) {
	query := `
		SELECT id, sender_account_id, receiver_account_id, quote_id, from_amount, to_amount, from_currency, to_currency, status, created_at
		FROM transactions WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	t := &domain.Transaction{}
	err := row.Scan(&t.ID, &t.SenderAccountID, &t.ReceiverAccountID, &t.QuoteID, &t.FromAmount, &t.ToAmount, &t.FromCurrency, &t.ToCurrency, &t.Status, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTransactionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	return t, nil
}

func (r *PostgresRepository) ListAccounts(ctx context.Context) ([]*domain.Account, error) {
	query := `SELECT id, owner_name, currency, balance, created_at, updated_at FROM accounts ORDER BY created_at DESC LIMIT 50`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*domain.Account
	for rows.Next() {
		acc := &domain.Account{}
		if err := rows.Scan(&acc.ID, &acc.OwnerName, &acc.Currency, &acc.Balance, &acc.CreatedAt, &acc.UpdatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

func (r *PostgresRepository) ListTransactions(ctx context.Context) ([]*domain.Transaction, error) {
	query := `
		SELECT id, sender_account_id, receiver_account_id, quote_id, from_amount, to_amount, from_currency, to_currency, status, created_at
		FROM transactions ORDER BY created_at DESC LIMIT 50
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*domain.Transaction
	for rows.Next() {
		t := &domain.Transaction{}
		if err := rows.Scan(&t.ID, &t.SenderAccountID, &t.ReceiverAccountID, &t.QuoteID, &t.FromAmount, &t.ToAmount, &t.FromCurrency, &t.ToCurrency, &t.Status, &t.CreatedAt); err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}

