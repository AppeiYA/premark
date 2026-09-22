package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"premark/internal/adapter/in/httpapi"
	"premark/internal/adapter/in/scheduler"
	"premark/internal/adapter/out/jupiter"
	"premark/internal/adapter/out/memstore"
	"premark/internal/adapter/out/notify/logger"
	"premark/internal/adapter/out/notify/multi"
	"premark/internal/adapter/out/notify/telegram"
	"premark/internal/adapter/out/prestocksapi"
	"premark/internal/adapter/out/solanarpc"
	"premark/internal/adapter/out/sqlitestore"
	"premark/internal/adapter/out/system"
	"premark/internal/config"
	"premark/internal/ports"
	"premark/internal/usecase"
)

type mintInfoAdapter struct {
	rpc *solanarpc.Client
}

func (a *mintInfoAdapter) MintInfo(ctx context.Context, mint string) (jupiter.MintInfo, error) {
	info, err := a.rpc.MintInfo(ctx, mint)
	if err != nil {
		return jupiter.MintInfo{}, err
	}
	return jupiter.MintInfo{
		Decimals:   info.Decimals,
		Multiplier: info.Multiplier,
	}, nil
}

func main() {
	loggerHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	log := slog.New(loggerHandler)

	cfg, err := config.Load()
	if err != nil {
		log.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	clock := system.NewClock()
	ids := system.NewIDGenerator()

	marketSource := prestocksapi.New(cfg.PreStocksAPIURL, http.DefaultClient, clock, 10*time.Second)

	rpcClient := solanarpc.New(cfg.SolanaRPCURL, http.DefaultClient, clock, 60*time.Second)
	mints := &mintInfoAdapter{rpc: rpcClient}
	quoteProvider := jupiter.New(jupiter.Config{
		BaseURL:          cfg.JupiterBaseURL,
		APIKey:           cfg.JupiterAPIKey,
		USDCMint:         cfg.USDCMint,
		SlippageBps:      50,
		ImpactIsFraction: cfg.JupiterImpactIsFraction,
	}, http.DefaultClient, clock, mints)

	var ruleRepo ports.RuleRepository
	var snapRepo ports.SnapshotRepository
	var signalRepo ports.SignalRepository

	if cfg.Storage == "sqlite" {
		db, err := sqlitestore.Open(cfg.DBPath)
		if err != nil {
			log.Error("failed to open sqlite database", "path", cfg.DBPath, "error", err)
			os.Exit(1)
		}
		defer db.Close()
		ruleRepo = sqlitestore.NewRuleRepository(db)
		snapRepo = sqlitestore.NewSnapshotRepository(db)
		signalRepo = sqlitestore.NewSignalRepository(db)
	} else {
		ruleRepo = memstore.NewRuleRepository()
		snapRepo = memstore.NewSnapshotRepository()
		signalRepo = memstore.NewSignalRepository()
	}

	notifiers := []ports.Notifier{logger.New(log)}
	if cfg.TelegramBotToken != "" {
		notifiers = append(notifiers, telegram.New(cfg.TelegramBotToken, http.DefaultClient))
	}
	notifier := multi.New(notifiers...)

	ruleService := usecase.NewRuleService(ruleRepo, marketSource, clock, ids)
	marketQuery := usecase.NewMarketQuery(marketSource, snapRepo, clock)
	signalQuery := usecase.NewSignalQuery(signalRepo)
	ingestor := usecase.NewIngestor(marketSource, snapRepo, log)
	evaluator := usecase.NewEvaluator(ruleRepo, quoteProvider, signalRepo, notifier, clock, ids, log)
	scanner := usecase.NewScanner(ingestor, evaluator, log)

	sched := scheduler.New(scanner, cfg.ScanInterval, log)

	router := httpapi.NewRouter(httpapi.Deps{
		Rules:      ruleService,
		Market:     marketQuery,
		Signals:    signalQuery,
		Scanner:    scanner,
		AdminToken: cfg.AdminToken,
		Logger:     log,
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("scheduler starting", "interval", cfg.ScanInterval)
		if err := sched.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("scheduler error", "error", err)
		}
	}()

	go func() {
		log.Info("server starting", "addr", server.Addr, "storage", cfg.Storage)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	log.Info("stopped gracefully")
}
