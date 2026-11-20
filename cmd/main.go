package main

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"alpha/config"
	"alpha/constants"
	"alpha/internal/handlers"
	"alpha/internal/modules"
	"alpha/internal/modules/logger"
	"alpha/internal/modules/routes"
	"alpha/internal/repository"
	"alpha/internal/usecase"
	"alpha/yggdrasil"

	"github.com/gofiber/fiber/v2"
)

func main() {
	config.LoadConfig()
	logger.LoggerInit(config.AppConfig.AppEnv)
	defer logger.LoggerSync()
	if err := run(); err != nil {
		logger.Log.Sugar().Error(err)
		os.Exit(1)
	}
}

func run() error {
	path := os.Getenv("AUTH_SIGNING_KEY_FILE")
	if path == "" {
		return errors.New("AUTH_SIGNING_KEY_FILE is required")
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return errors.New("cannot read AUTH_SIGNING_KEY_FILE")
	}
	var domains []string
	for item := range strings.SplitSeq(os.Getenv("AUTH_SKIN_DOMAINS"), ",") {
		if item = strings.TrimSpace(item); item != "" {
			domains = append(domains, item)
		}
	}
	signer, err := yggdrasil.NewSigner(key, domains)
	clear(key)
	if err != nil {
		return err
	}

	provisionToken := os.Getenv("AUTH_PROVISION_TOKEN")
	if provisionToken != "" && (len(provisionToken) < 32 || len(provisionToken) > 512 || strings.ContainsAny(provisionToken, " \t\r\n")) {
		return errors.New("AUTH_PROVISION_TOKEN must contain 32–512 characters without whitespace")
	}

	db, err := modules.NewPostgresDB(config.DBConfig)
	if err != nil {
		return errors.New("cannot connect to Postgres")
	}
	defer db.Close()
	migrate, cancel := context.WithTimeout(context.Background(), constants.MigrationTimeout)
	err = modules.ApplyMigrations(migrate, db)
	cancel()
	if err != nil {
		return err
	}
	attempts := repository.NewAttemptRepository(db)
	users, err := usecase.NewUserUseCase(repository.NewUserRepository(db), repository.NewSessionRepository(db), attempts)
	if err != nil {
		return err
	}
	gameRepo := repository.NewGameRepository(db)
	game := usecase.NewGameUseCase(users, gameRepo, attempts, signer)
	var proxies []string
	for item := range strings.SplitSeq(os.Getenv("AUTH_TRUSTED_PROXIES"), ",") {
		if item = strings.TrimSpace(item); item == "" {
			continue
		}
		_, addressErr := netip.ParseAddr(item)
		_, prefixErr := netip.ParsePrefix(item)
		if addressErr != nil && prefixErr != nil {
			return errors.New("AUTH_TRUSTED_PROXIES must contain IP addresses or CIDRs")
		}
		proxies = append(proxies, item)
	}
	appConfig := fiber.Config{
		AppName: "Alpha Auth App", BodyLimit: 16 << 10,
		ReadTimeout: constants.RequestTimeout, WriteTimeout: constants.RequestTimeout, IdleTimeout: 30 * time.Second,
	}
	if len(proxies) > 0 {
		appConfig.ProxyHeader = "X-Real-IP"
		appConfig.EnableTrustedProxyCheck = true
		appConfig.EnableIPValidation = true
		appConfig.TrustedProxies = proxies
	}
	app := fiber.New(appConfig)
	routes.SetupRoutes(app, handlers.NewAuthHandler(users, game, attempts), users, game, signer, provisionToken)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	maintenanceDone := make(chan struct{})
	go func() {
		defer close(maintenanceDone)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				prune, cancel := context.WithTimeout(ctx, constants.RequestTimeout)
				err := gameRepo.Prune(prune)
				cancel()
				if err != nil {
					logger.Log.Warn("Could not prune expired authentication state")
				}
			}
		}
	}()
	done := make(chan error, 1)
	go func() { done <- app.Listen(net.JoinHostPort(config.AppConfig.AppHost, config.AppConfig.AppPort)) }()
	select {
	case <-ctx.Done():
		err = app.ShutdownWithTimeout(constants.RequestTimeout)
	case err = <-done:
	}
	stop()
	<-maintenanceDone
	return err
}
