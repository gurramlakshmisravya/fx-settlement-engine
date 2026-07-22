package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/user/fx-settlement-engine/proto/gen"
)

func main() {
	serverAddr := flag.String("addr", "fx-settlement-engine.onrender.com:443", "gRPC server address (host:port)")
	useTLS := flag.Bool("tls", true, "Use TLS connection")
	flag.Parse()

	log.Printf("Connecting to FX Settlement Engine at %s (TLS=%v)...", *serverAddr, *useTLS)

	var opts []grpc.DialOption
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("\n=======================================================")
	log.Println(" 1. Creating USD Sender Account...")
	sender, err := accClient.CreateAccount(ctx, &pb.CreateAccountRequest{
		OwnerName:      "Alice USD Corporate",
		Currency:       "USD",
		InitialBalance: 10000.0,
	})
	if err != nil {
		log.Fatalf("Failed to create sender account: %v", err)
	}
	fmt.Printf("   ✅ Sender Created: ID=%s | Owner=%s | Balance=%.2f %s\n", sender.Id, sender.OwnerName, sender.Balance, sender.Currency)

	log.Println("\n 2. Creating EUR Receiver Account...")
	receiver, err := accClient.CreateAccount(ctx, &pb.CreateAccountRequest{
		OwnerName:      "Bob EUR Enterprise",
		Currency:       "EUR",
		InitialBalance: 500.0,
	})
	if err != nil {
		log.Fatalf("Failed to create receiver account: %v", err)
	}
	fmt.Printf("   ✅ Receiver Created: ID=%s | Owner=%s | Balance=%.2f %s\n", receiver.Id, receiver.OwnerName, receiver.Balance, receiver.Currency)

	log.Println("\n 3. Locking FX Exchange Quote (USD -> EUR for $1,000)...")
	quote, err := fxClient.LockQuote(ctx, &pb.LockQuoteRequest{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		Amount:       1000.0,
		TtlSeconds:   60,
	})
	if err != nil {
		log.Fatalf("Failed to lock quote: %v", err)
	}
	fmt.Printf("   ✅ FX Quote Locked: ID=%s | Rate=%.4f | $%.2f USD -> €%.2f EUR | Expires=%s\n",
		quote.QuoteId, quote.Rate, quote.FromAmount, quote.ToAmount, quote.ExpiresAt)

	log.Println("\n 4. Executing Atomic Cross-Border Settlement Transaction...")
	settlement, err := settlementClient.CreateSettlement(ctx, &pb.CreateSettlementRequest{
		SenderAccountId:   sender.Id,
		ReceiverAccountId: receiver.Id,
		QuoteId:           quote.QuoteId,
		ReferenceId:       "REF-SETTLE-001",
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

	log.Println("\n 5. Verifying Updated Sender & Receiver Balances...")
	updatedSender, _ := accClient.GetAccount(ctx, &pb.GetAccountRequest{AccountId: sender.Id})
	updatedReceiver, _ := accClient.GetAccount(ctx, &pb.GetAccountRequest{AccountId: receiver.Id})

	fmt.Printf("   📊 Updated Sender Balance: %.2f USD (Debited $1,000)\n", updatedSender.Balance)
	fmt.Printf("   📊 Updated Receiver Balance: %.2f EUR (Credited €920)\n", updatedReceiver.Balance)
	log.Println("=======================================================")
}
