-- Migration: 000002_init_partition_tables.up.sql
-- Description: Creates monthly partitions for schema_earning.earning_transactions

CREATE TABLE IF NOT EXISTS schema_earning.earning_transactions_2026_11 PARTITION OF schema_earning.earning_transactions
    FOR VALUES FROM ('2026-11-01 00:00:00+00') TO ('2026-12-01 00:00:00+00');

CREATE TABLE IF NOT EXISTS schema_earning.earning_transactions_2026_12 PARTITION OF schema_earning.earning_transactions
    FOR VALUES FROM ('2026-12-01 00:00:00+00') TO ('2027-01-01 00:00:00+00');

CREATE TABLE IF NOT EXISTS schema_earning.earning_transactions_2027_01 PARTITION OF schema_earning.earning_transactions
    FOR VALUES FROM ('2027-01-01 00:00:00+00') TO ('2027-02-01 00:00:00+00');

CREATE TABLE IF NOT EXISTS schema_earning.earning_transactions_2027_02 PARTITION OF schema_earning.earning_transactions
    FOR VALUES FROM ('2027-02-01 00:00:00+00') TO ('2027-03-01 00:00:00+00');

