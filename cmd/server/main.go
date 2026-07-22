package main

import (
	"context"
	"database/sql"

	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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

//go:embed web/index.html
var indexHTML []byte

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

	// 7. Setup gRPC Handlers
	accHandler := grpchandler.NewAccountGRPCHandler(accService)
	fxHandler := grpchandler.NewFXGRPCHandler(fxService)
	settlementHandler := grpchandler.NewSettlementGRPCHandler(engine)

	grpcServer := grpc.NewServer()
	pb.RegisterAccountServiceServer(grpcServer, accHandler)
	pb.RegisterFXServiceServer(grpcServer, fxHandler)
	pb.RegisterSettlementServiceServer(grpcServer, settlementHandler)
	reflection.Register(grpcServer)

	// 8. Create REST HTTP Mux for Web Dashboard & API endpoints
	httpMux := http.NewServeMux()

	httpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"UP","database":"connected","redis":"connected"}`))
	})

	httpMux.HandleFunc("/api/accounts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			accounts, err := postgresRepo.ListAccounts(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(accounts)
			return
		}
		if r.Method == http.MethodPost {
			var req pb.CreateAccountRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			res, err := accHandler.CreateAccount(r.Context(), &req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(res)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	httpMux.HandleFunc("/api/transactions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			txs, err := postgresRepo.ListTransactions(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(txs)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	httpMux.HandleFunc("/api/quotes/lock", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var req pb.LockQuoteRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			res, err := fxHandler.LockQuote(r.Context(), &req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(res)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	httpMux.HandleFunc("/api/settlements", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var req pb.CreateSettlementRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			res, err := settlementHandler.CreateSettlement(r.Context(), &req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(res)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// Multiplex gRPC and REST HTTP traffic on single PORT
	mixedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			httpMux.ServeHTTP(w, r)
		}
	})

	h2s := &http2.Server{}
	h2cHandler := h2c.NewHandler(mixedHandler, h2s)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.GRPCPort),
		Handler: h2cHandler,
	}

	// 9. Graceful Shutdown Signal Interceptor
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[Server] Server listening on 0.0.0.0:%s (Web UI & gRPC & REST Enabled)...", cfg.GRPCPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stopChan
	log.Println("\n[Shutdown] Shutting down FX Settlement Engine gracefully...")
	ctxShut, cancelShut := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShut()
	grpcServer.GracefulStop()
	httpServer.Shutdown(ctxShut)
	log.Println("[Shutdown] Engine stopped cleanly. Goodbye!")
}
