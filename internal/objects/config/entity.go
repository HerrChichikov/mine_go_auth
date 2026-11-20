package config

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type AppConfig struct {
	AppPort  string
	AppEnv   string
	AppHost  string
	AppLevel string
}

type JWTConfig struct {
	JWTSecret string
}
