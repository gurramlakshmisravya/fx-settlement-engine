package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/user/fx-settlement-engine/internal/account"
	"github.com/user/fx-settlement-engine/internal/fx"
	"github.com/user/fx-settlement-engine/internal/settlement"
	pb "github.com/user/fx-settlement-engine/proto/gen"
)

type AccountGRPCHandler struct {
	pb.UnimplementedAccountServiceServer
	accService *account.Service
}

func NewAccountGRPCHandler(accService *account.Service) *AccountGRPCHandler {
	return &AccountGRPCHandler{accService: accService}
}

func (h *AccountGRPCHandler) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.Account, error) {
	acc, err := h.accService.CreateAccount(ctx, req.GetOwnerName(), req.GetCurrency(), req.GetInitialBalance())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to create account: %v", err)
	}

	return &pb.Account{
		Id:        acc.ID,
		OwnerName: acc.OwnerName,
		Currency:  acc.Currency,
		Balance:   acc.Balance,
		CreatedAt: acc.CreatedAt.Format(time.RFC3339),
		UpdatedAt: acc.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (h *AccountGRPCHandler) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.Account, error) {
	acc, err := h.accService.GetAccount(ctx, req.GetAccountId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "account not found: %v", err)
	}

	return &pb.Account{
		Id:        acc.ID,
		OwnerName: acc.OwnerName,
		Currency:  acc.Currency,
		Balance:   acc.Balance,
		CreatedAt: acc.CreatedAt.Format(time.RFC3339),
		UpdatedAt: acc.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (h *AccountGRPCHandler) UpdateBalance(ctx context.Context, req *pb.UpdateBalanceRequest) (*pb.Account, error) {
	acc, err := h.accService.UpdateBalance(ctx, req.GetAccountId(), req.GetAmountDelta())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update balance: %v", err)
	}

	return &pb.Account{
		Id:        acc.ID,
		OwnerName: acc.OwnerName,
		Currency:  acc.Currency,
		Balance:   acc.Balance,
		CreatedAt: acc.CreatedAt.Format(time.RFC3339),
		UpdatedAt: acc.UpdatedAt.Format(time.RFC3339),
	}, nil
}

type FXGRPCHandler struct {
	pb.UnimplementedFXServiceServer
	fxService *fx.Service
}

func NewFXGRPCHandler(fxService *fx.Service) *FXGRPCHandler {
	return &FXGRPCHandler{fxService: fxService}
}

func (h *FXGRPCHandler) LockQuote(ctx context.Context, req *pb.LockQuoteRequest) (*pb.Quote, error) {
	q, err := h.fxService.LockQuote(ctx, req.GetFromCurrency(), req.GetToCurrency(), req.GetAmount(), int(req.GetTtlSeconds()))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to lock quote: %v", err)
	}

	return &pb.Quote{
		QuoteId:      q.ID,
		FromCurrency: q.FromCurrency,
		ToCurrency:   q.ToCurrency,
		Rate:         q.Rate,
		FromAmount:   q.FromAmount,
		ToAmount:     q.ToAmount,
		ExpiresAt:    q.ExpiresAt.Format(time.RFC3339),
		IsLocked:     true,
	}, nil
}

type SettlementGRPCHandler struct {
	pb.UnimplementedSettlementServiceServer
	engine *settlement.Engine
}

func NewSettlementGRPCHandler(engine *settlement.Engine) *SettlementGRPCHandler {
	return &SettlementGRPCHandler{engine: engine}
}

func (h *SettlementGRPCHandler) CreateSettlement(ctx context.Context, req *pb.CreateSettlementRequest) (*pb.SettlementResponse, error) {
	sReq := &settlement.SettlementRequest{
		SenderAccountID:   req.GetSenderAccountId(),
		ReceiverAccountID: req.GetReceiverAccountId(),
		QuoteID:           req.GetQuoteId(),
		ReferenceID:       req.GetReferenceId(),
	}

	tx, err := h.engine.ProcessSettlement(ctx, sReq)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "settlement processing failed: %v", err)
	}

	fxRate := 1.0
	if tx.FromAmount > 0 {
		fxRate = tx.ToAmount / tx.FromAmount
	}

	return &pb.SettlementResponse{
		TransactionId:     tx.ID,
		Status:            string(tx.Status),
		SenderAccountId:   tx.SenderAccountID,
		ReceiverAccountId: tx.ReceiverAccountID,
		SenderDebited:     tx.FromAmount,
		ReceiverCredited:  tx.ToAmount,
		FromCurrency:      tx.FromCurrency,
		ToCurrency:        tx.ToCurrency,
		FxRate:            fxRate,
		Timestamp:         tx.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (h *SettlementGRPCHandler) GetTransaction(ctx context.Context, req *pb.GetTransactionRequest) (*pb.Transaction, error) {
	tx, err := h.engine.GetTransaction(ctx, req.GetTransactionId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "transaction not found: %v", err)
	}

	return &pb.Transaction{
		Id:                tx.ID,
		SenderAccountId:   tx.SenderAccountID,
		ReceiverAccountId: tx.ReceiverAccountID,
		QuoteId:           tx.QuoteID,
		FromAmount:        tx.FromAmount,
		ToAmount:          tx.ToAmount,
		FromCurrency:      tx.FromCurrency,
		ToCurrency:        tx.ToCurrency,
		Status:            string(tx.Status),
		CreatedAt:         tx.CreatedAt.Format(time.RFC3339),
	}, nil
}
