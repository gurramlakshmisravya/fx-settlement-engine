package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/user/fx-settlement-engine/proto/gen"
)

func main() {
	serverAddr := flag.String("addr", "fx-settlement-engine.onrender.com:443", "Engine server address (host:port)")
	useTLS := flag.Bool("tls", true, "Use TLS connection")
	flag.Parse()

	log.Printf("Connecting to FX Settlement Engine at %s (TLS=%v)...", *serverAddr, *useTLS)

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
	if *useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	accClient := pb.NewAccountServiceClient(conn)
	fxClient := pb.NewFXServiceClient(conn)
	settlementClient := pb.NewSettlementServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	log.Println("\n=======================================================")
	log.Println(" 1. Creating USD Sender Account...")

	var sender pb.Account
	err = callAPI(ctx, *serverAddr, *useTLS, "/api/accounts", &pb.CreateAccountRequest{
		OwnerName:      "Alice USD Corporate",
		Currency:       "USD",
		InitialBalance: 10000.0,
	}, &sender, func() error {
		res, err := accClient.CreateAccount(ctx, &pb.CreateAccountRequest{
			OwnerName:      "Alice USD Corporate",
			Currency:       "USD",
			InitialBalance: 10000.0,
		})
		if err == nil && res != nil {
			sender = *res
		}
		return err
	})
	if err != nil {
		log.Fatalf("Failed to create sender account: %v", err)
	}
	fmt.Printf("   ✅ Sender Created: ID=%s | Owner=%s | Balance=%.2f %s\n", sender.Id, sender.OwnerName, sender.Balance, sender.Currency)

	log.Println("\n 2. Creating EUR Receiver Account...")
	var receiver pb.Account
	err = callAPI(ctx, *serverAddr, *useTLS, "/api/accounts", &pb.CreateAccountRequest{
		OwnerName:      "Bob EUR Enterprise",
		Currency:       "EUR",
		InitialBalance: 500.0,
	}, &receiver, func() error {
		res, err := accClient.CreateAccount(ctx, &pb.CreateAccountRequest{
			OwnerName:      "Bob EUR Enterprise",
			Currency:       "EUR",
			InitialBalance: 500.0,
		})
		if err == nil && res != nil {
			receiver = *res
		}
		return err
	})
	if err != nil {
		log.Fatalf("Failed to create receiver account: %v", err)
	}
	fmt.Printf("   ✅ Receiver Created: ID=%s | Owner=%s | Balance=%.2f %s\n", receiver.Id, receiver.OwnerName, receiver.Balance, receiver.Currency)

	log.Println("\n 3. Locking FX Exchange Quote (USD -> EUR for $1,000)...")
	var quote pb.Quote
	err = callAPI(ctx, *serverAddr, *useTLS, "/api/quotes/lock", &pb.LockQuoteRequest{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		Amount:       1000.0,
		TtlSeconds:   60,
	}, &quote, func() error {
		res, err := fxClient.LockQuote(ctx, &pb.LockQuoteRequest{
			FromCurrency: "USD",
			ToCurrency:   "EUR",
			Amount:       1000.0,
			TtlSeconds:   60,
		})
		if err == nil && res != nil {
			quote = *res
		}
		return err
	})
	if err != nil {
		log.Fatalf("Failed to lock quote: %v", err)
	}
	fmt.Printf("   ✅ FX Quote Locked: ID=%s | Rate=%.4f | $%.2f USD -> €%.2f EUR | Expires=%s\n",
		quote.QuoteId, quote.Rate, quote.FromAmount, quote.ToAmount, quote.ExpiresAt)

	log.Println("\n 4. Executing Atomic Cross-Border Settlement Transaction...")
	var settlement pb.SettlementResponse
	err = callAPI(ctx, *serverAddr, *useTLS, "/api/settlements", &pb.CreateSettlementRequest{
		SenderAccountId:   sender.Id,
		ReceiverAccountId: receiver.Id,
		QuoteId:           quote.QuoteId,
		ReferenceId:       "REF-SETTLE-001",
	}, &settlement, func() error {
		res, err := settlementClient.CreateSettlement(ctx, &pb.CreateSettlementRequest{
			SenderAccountId:   sender.Id,
			ReceiverAccountId: receiver.Id,
			QuoteId:           quote.QuoteId,
			ReferenceId:       "REF-SETTLE-001",
		})
		if err == nil && res != nil {
			settlement = *res
		}
		return err
	})
	if err != nil {
		log.Fatalf("Settlement failed: %v", err)
	}
	fmt.Printf("   🎉 SETTLEMENT SUCCESSFUL!\n")
	fmt.Printf("      Tx ID: %s\n", settlement.TransactionId)
	fmt.Printf("      Status: %s\n", settlement.Status)
	fmt.Printf("      Debited Sender: -%.2f %s\n", settlement.SenderDebited, settlement.FromCurrency)
	fmt.Printf("      Credited Receiver: +%.2f %s\n", settlement.ReceiverCredited, settlement.ToCurrency)
	fmt.Printf("      FX Execution Rate: %.4f\n", settlement.FxRate)

	log.Println("\n 5. Settlement Execution Complete!")
	log.Println("=======================================================")
}

func callAPI(ctx context.Context, addr string, useTLS bool, endpoint string, reqPayload interface{}, respObj interface{}, grpcFunc func() error) error {
	// Try gRPC call first
	err := grpcFunc()
	if err == nil {
		return nil
	}

	// If gRPC returns proxy error (e.g. Render HTTP 502/Unavailable), fallback to HTTP REST API
	scheme := "https"
	if !useTLS {
		scheme = "http"
	}

	// Format host address
	host := addr
	if strings.HasSuffix(host, ":443") {
		host = strings.TrimSuffix(host, ":443")
	}

	url := fmt.Sprintf("%s://%s%s", scheme, host, endpoint)
	body, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	return json.Unmarshal(respBytes, respObj)
}
