package main

import (
	"os"
	"os/signal"
	"syscall"

	"alpha/config"
	"alpha/internal/handlers"
	"alpha/internal/modules"
	"alpha/internal/modules/logger"
	"alpha/internal/modules/routes"
	"alpha/internal/objects/enums"
	"alpha/internal/repository"
	"alpha/internal/services"
	"alpha/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

var ServiceStatus enums.ServiceStatus

func main() {
	// INIT STATE
	ServiceStatus = enums.Start

	config.LoadConfig()

	logger.LoggerInit(config.AppConfig.AppEnv)
	defer logger.LoggerSync()
	// END INIT STATE

	// DB INIT
	db, err := modules.NewPostgresDB(
		config.DBConfig,
	)

	if err != nil {
		ServiceStatus = enums.DB_Broken

		logger.Log.Sugar().Fatalf(
			"Не удалось подключиться к БД: %v",
			err,
		)
	}

	defer func() {
		if err := db.Close(); err != nil {
			logger.Log.Sugar().Fatalf("Ошибка закрытия коннекта к БД: %v", err)
		}
		logger.Log.Info("Коннект к БД закрыт")
	}()

	if err := modules.ApplyMigrations(db); err != nil {
		logger.Log.Sugar().Fatalf(
			"Не удалось применить миграции: %v",
			err,
		)
	}
	// END DB INIT

	// REPO INIT
	userRepo := repository.NewUserRepository(db)
	invitationRepo := repository.NewInvitationRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	// END REPO INIT

	// SERVICES INIT
	tokenService := services.NewTokenService(config.JWTConfig.JWTSecret)
	// END SERVICES INIT

	// USECASE INIT
	userUseCase := usecase.NewUserUseCase(
		userRepo,
		invitationRepo,
		sessionRepo,
		tokenService,
	)
	// END USECASE INIT

	// HANDLERS INIT
	authHandler := handlers.NewAuthHandler(userUseCase)
	// END HANDLERS INIT

	// APP INIT
	app := fiber.New(fiber.Config{
		AppName:      "Alpha Auth App",
		ServerHeader: "Alpha",
		BodyLimit:    10 * 1024 * 1024,
	},
	)
	routes.SetupRoutes(
		app,
		authHandler,
	)
	ServiceStatus = enums.Ok

	go func() {
		logger.Log.Sugar().Infof(
			"Сервер запущен на порту :%s",
			config.AppConfig.AppPort,
		)

		if err := app.Listen(":" + config.AppConfig.AppPort); err != nil {
			logger.Log.Sugar().Fatalf("Ошибка запуска сервера: %v", err)
		}

	}()
	// END APP INIT

	// APP QUIT CONTROL
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ServiceStatus = enums.Exit

	if err := app.Shutdown(); err != nil {
		logger.Log.Sugar().Fatalf("Ошибка при остановке сервера: %v", err)
	}

	logger.Log.Info("Сервер получил сигнал завершения")
	// APP QUIT CONTROL
}
