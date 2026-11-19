-- Migration: 000002_init_partition_tables.down.sql

DROP TABLE IF EXISTS schema_earning.earning_transactions_2027_02;
DROP TABLE IF EXISTS schema_earning.earning_transactions_2027_01;
DROP TABLE IF EXISTS schema_earning.earning_transactions_2026_12;
DROP TABLE IF EXISTS schema_earning.earning_transactions_2026_11;

