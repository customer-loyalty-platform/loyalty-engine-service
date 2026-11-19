-- Migration: 000001_init_earning_schema.down.sql

DROP TABLE IF EXISTS schema_earning.earning_daily_summary CASCADE;
DROP TABLE IF EXISTS schema_earning.earning_reversals CASCADE;
DROP TABLE IF EXISTS schema_earning.expiry_notifications CASCADE;
DROP TABLE IF EXISTS schema_earning.earning_cap_counters CASCADE;
DROP TABLE IF EXISTS schema_earning.earning_caps CASCADE;
DROP TABLE IF EXISTS schema_earning.point_expiry_buckets CASCADE;
DROP TABLE IF EXISTS schema_earning.earning_rule_results CASCADE;
DROP TABLE IF EXISTS schema_earning.earning_transactions CASCADE;
DROP SCHEMA IF EXISTS schema_earning CASCADE;

