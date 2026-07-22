package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/user/fx-settlement-engine/internal/account"
	"github.com/user/fx-settlement-engine/internal/cache"
	"github.com/user/fx-settlement-engine/internal/config"
	"github.com/user/fx-settlement-engine/internal/fx"
	grpchandler "github.com/user/fx-settlement-engine/internal/grpc"
	"github.com/user/fx-settlement-engine/internal/kafka"
	"github.com/user/fx-settlement-engine/internal/ledger"
	"github.com/user/fx-settlement-engine/internal/repository"
	"github.com/user/fx-settlement-engine/internal/settlement"
	"github.com/user/fx-settlement-engine/internal/worker"
	pb "github.com/user/fx-settlement-engine/proto/gen"
)

func main() {
	log.Println("=================================================================")
	log.Println(" Starting Real-Time Cross-Border FX & Settlement Engine Service")
	log.Println("=================================================================")

	// 1. Load Configuration
	cfg := config.Load()

	// 2. Connect to PostgreSQL
	log.Printf("[Database] Connecting to PostgreSQL at %s:%s...", cfg.DBHost, cfg.DBPort)
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		log.Fatalf("Failed to open DB connection: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Printf("[Database] Warning: DB ping failed (will retry if running in Docker): %v", err)
	} else {
		log.Println("[Database] Connected successfully to PostgreSQL!")
	}

	// 3. Connect to Redis Cache
	log.Printf("[Redis] Connecting to Redis at %s...", cfg.RedisAddr)
	redisCache := cache.NewRedisCache(cfg.RedisAddr, cfg.RedisPass)
	if err := redisCache.Ping(context.Background()); err != nil {
		log.Printf("[Redis] Warning: Redis ping failed (will retry if running in Docker): %v", err)
	} else {
		log.Println("[Redis] Connected successfully to Redis!")
	}

	// 4. Initialize Kafka Producer & Consumer
	log.Printf("[Kafka] Initializing Kafka producer for topic '%s'...", cfg.KafkaTopic)
	kafkaProducer := kafka.NewEventProducer(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer kafkaProducer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kafkaConsumer := kafka.NewAuditConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, "audit-group")
	defer kafkaConsumer.Close()
	go kafkaConsumer.StartListening(ctx)

	// 5. Initialize Concurrency Worker Pool
	workerPool := worker.NewWorkerPool(10, 100)
	workerPool.Start(ctx)
	defer workerPool.Stop()

	// 6. Initialize Layered Architecture Components
	postgresRepo := repository.NewPostgresRepository(db)
	accService := account.NewService(postgresRepo, redisCache)
	fxService := fx.NewService(postgresRepo, redisCache)
	ledgerService := ledger.NewService()
	engine := settlement.NewEngine(postgresRepo, accService, fxService, ledgerService, kafkaProducer)

	// 7. Setup gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", cfg.GRPCPort, err)
	}

	grpcServer := grpc.NewServer()

	// Register Services
	accHandler := grpchandler.NewAccountGRPCHandler(accService)
	fxHandler := grpchandler.NewFXGRPCHandler(fxService)
	settlementHandler := grpchandler.NewSettlementGRPCHandler(engine)

	pb.RegisterAccountServiceServer(grpcServer, accHandler)
	pb.RegisterFXServiceServer(grpcServer, fxHandler)
	pb.RegisterSettlementServiceServer(grpcServer, settlementHandler)

	reflection.Register(grpcServer)

	// 8. Graceful Shutdown Signal Interceptor
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[gRPC Server] Listening on 0.0.0.0:%s...", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	<-stopChan
	log.Println("\n[Shutdown] Shutting down FX Settlement Engine gracefully...")
	grpcServer.GracefulStop()
	log.Println("[Shutdown] Engine stopped cleanly. Goodbye!")
}
