package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Config struct {
	MasterDSN   string
	ReplicaDSN  string
	MaxConns    int32
	MinConns    int32
	MaxConnIdle time.Duration
	MaxConnLife time.Duration
}

type PostgresDB struct {
	Master  *pgxpool.Pool
	Replica *pgxpool.Pool
}

func NewPostgresClient(ctx context.Context, cfg Config, log *zap.Logger) (*PostgresDB, error) {
	// Configure Master Connection Pool
	masterConfig, err := pgxpool.ParseConfig(cfg.MasterDSN)
	if err != nil {
		return nil, fmt.Errorf("unable to parse master dsn: %w", err)
	}

	masterConfig.MaxConns = cfg.MaxConns
	masterConfig.MinConns = cfg.MinConns
	masterConfig.MaxConnIdleTime = cfg.MaxConnIdle
	masterConfig.MaxConnLifetime = cfg.MaxConnLife

	masterPool, err := pgxpool.NewWithConfig(ctx, masterConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to master postgres: %w", err)
	}

	// Attempt master ping
	if err := masterPool.Ping(ctx); err != nil {
		log.Warn("Master database ping failed (will retry on demand)", zap.Error(err))
	} else {
		log.Info("Connected to PostgreSQL master pool")
	}

	// Configure Read-Replica Connection Pool
	var replicaPool *pgxpool.Pool
	if cfg.ReplicaDSN != "" && cfg.ReplicaDSN != cfg.MasterDSN {
		replicaConfig, err := pgxpool.ParseConfig(cfg.ReplicaDSN)
		if err == nil {
			replicaConfig.MaxConns = cfg.MaxConns
			replicaConfig.MinConns = cfg.MinConns
			replicaConfig.MaxConnIdleTime = cfg.MaxConnIdle
			replicaConfig.MaxConnLifetime = cfg.MaxConnLife

			replicaPool, err = pgxpool.NewWithConfig(ctx, replicaConfig)
			if err != nil {
				log.Warn("Replica pool initialization failed, falling back to master", zap.Error(err))
				replicaPool = masterPool
			} else if err := replicaPool.Ping(ctx); err != nil {
				log.Warn("Replica ping failed, falling back to master", zap.Error(err))
				replicaPool = masterPool
			} else {
				log.Info("Connected to PostgreSQL replica pool")
			}
		} else {
			replicaPool = masterPool
		}
	} else {
		replicaPool = masterPool
	}

	return &PostgresDB{
		Master:  masterPool,
		Replica: replicaPool,
	}, nil
}

func (db *PostgresDB) Close() {
	if db.Master != nil {
		db.Master.Close()
	}
	if db.Replica != nil && db.Replica != db.Master {
		db.Replica.Close()
	}
}

