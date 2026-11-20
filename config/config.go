package config

import (
	"alpha/constants"
	"alpha/internal/modules"
	"alpha/internal/objects/config"
)

var AppConfig *config.AppConfig
var DBConfig *config.DatabaseConfig
var JWTConfig *config.JWTConfig

func LoadConfig() {
	AppConfig = &config.AppConfig{

		AppPort: modules.GetEnv(
			constants.ENV_APP_PORT,
			constants.APP_DEFAULT_PORT,
		),

		AppEnv: modules.GetEnv(
			constants.ENV_APP_ENV,
			constants.APP_DEFAULT_ENV,
		),

		AppHost: modules.GetEnv(
			constants.ENV_APP_HOST,
			constants.APP_DEFAULT_HOST,
		),

		AppLevel: modules.GetEnv(
			constants.ENV_APP_APP_LEVEL,
			constants.APP_DEFAULT_APP_LEVEL,
		),
	}

	DBConfig = &config.DatabaseConfig{
		Host: modules.GetEnv(
			constants.ENV_DB_HOST,
			constants.DB_DEFAULT_HOST,
		),

		Port: modules.GetEnv(
			constants.ENV_DB_PORT,
			constants.DB_DEFAULT_PORT,
		),

		User: modules.GetEnv(
			constants.ENV_DB_USER,
			constants.DB_DEFAULT_USER,
		),

		Password: modules.GetEnv(
			constants.ENV_DB_PASSWORD,
			constants.DB_DEFAULT_PASSWORD,
		),

		DBName: modules.GetEnv(
			constants.ENV_DB_NAME,
			constants.DB_DEFAULT_DB_NAME,
		),

		SSLMode: modules.GetEnv(
			constants.ENV_DB_SSL_MODE,
			constants.DB_DEFAULT_SSL_MODE,
		),
	}

	JWTConfig = &config.JWTConfig{
		JWTSecret: modules.GetEnv(
			constants.ENV_JWT_SECRET,
			constants.DEFAULT_JWT_SECRET,
		),
	}
}
