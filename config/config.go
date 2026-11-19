package config

import (
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Workers  WorkersConfig  `mapstructure:"workers"`
}

type AppConfig struct {
	Port         string        `mapstructure:"port"`
	Env          string        `mapstructure:"env"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type DatabaseConfig struct {
	MasterDSN   string        `mapstructure:"master_dsn"`
	ReplicaDSN  string        `mapstructure:"replica_dsn"`
	MaxConns    int32         `mapstructure:"max_conns"`
	MinConns    int32         `mapstructure:"min_conns"`
	MaxConnIdle time.Duration `mapstructure:"max_conn_idle"`
	MaxConnLife time.Duration `mapstructure:"max_conn_life"`
}

type JWTConfig struct {
	Secret        string `mapstructure:"secret"`
	SigningMethod string `mapstructure:"signing_method"`
}

type WorkersConfig struct {
	OutboxPollIntervalSecs int    `mapstructure:"outbox_poll_interval_secs"`
	ExpiryBatchCron        string `mapstructure:"expiry_batch_cron"`
	ExpiryBatchSize        int    `mapstructure:"expiry_batch_size"`
}

func LoadConfig() *Config {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Default configurations
	v.SetDefault("app.port", "8081")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.read_timeout", "10s")
	v.SetDefault("app.write_timeout", "15s")
	v.SetDefault("app.idle_timeout", "60s")

	v.SetDefault("database.master_dsn", "postgres://postgres:postgres@localhost:5432/loyalty_databases?sslmode=disable")
	v.SetDefault("database.replica_dsn", "postgres://postgres:postgres@localhost:5432/loyalty_databases?sslmode=disable")
	v.SetDefault("database.max_conns", 50)
	v.SetDefault("database.min_conns", 5)
	v.SetDefault("database.max_conn_idle", "5m")
	v.SetDefault("database.max_conn_life", "1h")

	v.SetDefault("jwt.secret", "loyalty-engine-super-secret-key-change-in-production")
	v.SetDefault("jwt.signing_method", "HS256")

	v.SetDefault("workers.outbox_poll_interval_secs", 5)
	v.SetDefault("workers.expiry_batch_cron", "0 * * * *")
	v.SetDefault("workers.expiry_batch_size", 500)

	if err := v.ReadInConfig(); err != nil {
		log.Printf("Notice: No config file found (%v), using defaults and environment variables", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("Fatal: Unable to decode configuration into struct: %v", err)
	}

	return &cfg
}

