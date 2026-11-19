-- Migration: 000001_init_earning_schema.up.sql
-- Description: Initializes schema_earning and core runtime tables for loyalty earning engine

CREATE SCHEMA IF NOT EXISTS schema_earning;

-- 1. earning_transactions (Partitioned master table)
CREATE TABLE IF NOT EXISTS schema_earning.earning_transactions (
	id uuid DEFAULT gen_random_uuid() NOT NULL,
	member_id uuid NOT NULL,
	partner_id uuid NULL,
	campaign_id uuid NULL,
	transaction_ref varchar(128) NOT NULL,
	idempotency_key varchar(128) NOT NULL,
	source_channel varchar(30) NOT NULL,
	transaction_type varchar(30) NOT NULL,
	transaction_amount numeric(15, 2) NULL,
	currency_code varchar(3) DEFAULT 'IDR'::character varying NOT NULL,
	transaction_date timestamptz NOT NULL,
	metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
	status varchar(20) DEFAULT 'PENDING'::character varying NOT NULL,
	error_message text NULL,
	processed_at timestamptz NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT earning_transactions_pkey PRIMARY KEY (id, created_at),
	CONSTRAINT earning_transactions_source_channel_check CHECK (((source_channel)::text = ANY ((ARRAY['POS'::character varying, 'ECOMMERCE'::character varying, 'MOBILE_APP'::character varying, 'WEB'::character varying, 'API'::character varying, 'KIOSK'::character varying, 'IOT'::character varying])::text[]))),
	CONSTRAINT earning_transactions_transaction_type_check CHECK (((transaction_type)::text = ANY ((ARRAY['PURCHASE'::character varying, 'TOP_UP'::character varying, 'BILL_PAYMENT'::character varying, 'REGISTRATION'::character varying, 'PROFILE_COMPLETION'::character varying, 'CHECK_IN'::character varying, 'REVIEW'::character varying, 'REFERRAL'::character varying, 'SOCIAL_SHARE'::character varying, 'ECO_ACTION'::character varying, 'CUSTOM'::character varying])::text[]))),
	CONSTRAINT earning_transactions_status_check CHECK (((status)::text = ANY ((ARRAY['PENDING'::character varying, 'PROCESSING'::character varying, 'COMPLETED'::character varying, 'FAILED'::character varying, 'REVERSED'::character varying])::text[])))
) PARTITION BY RANGE (created_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_earning_txn_idempotency ON schema_earning.earning_transactions USING btree (idempotency_key, created_at);
CREATE INDEX IF NOT EXISTS idx_earning_txn_member_id ON schema_earning.earning_transactions USING btree (member_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_earning_txn_status ON schema_earning.earning_transactions USING btree (status) WHERE ((status)::text = 'PENDING'::text);
CREATE INDEX IF NOT EXISTS idx_earning_txn_partner_id ON schema_earning.earning_transactions USING btree (partner_id) WHERE (partner_id IS NOT NULL);
CREATE INDEX IF NOT EXISTS idx_earning_txn_campaign_id ON schema_earning.earning_transactions USING btree (campaign_id) WHERE (campaign_id IS NOT NULL);
CREATE INDEX IF NOT EXISTS idx_earning_txn_date ON schema_earning.earning_transactions USING btree (transaction_date DESC);

-- 2. earning_rule_results
CREATE TABLE IF NOT EXISTS schema_earning.earning_rule_results (
	id uuid DEFAULT gen_random_uuid() NOT NULL,
	earning_transaction_id uuid NOT NULL,
	rule_id uuid NOT NULL,
	rule_code varchar(50) NOT NULL,
	base_points numeric(15, 2) NOT NULL,
	tier_multiplier numeric(5, 2) DEFAULT 1.00 NOT NULL,
	campaign_multiplier numeric(5, 2) DEFAULT 1.00 NOT NULL,
	time_multiplier numeric(5, 2) DEFAULT 1.00 NOT NULL,
	bonus_points numeric(15, 2) DEFAULT 0.00 NOT NULL,
	calculated_points numeric(15, 2) NOT NULL,
	final_points numeric(15, 2) NOT NULL,
	was_capped bool DEFAULT false NOT NULL,
	cap_applied numeric(15, 2) NULL,
	evaluation_detail jsonb DEFAULT '{}'::jsonb NOT NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT earning_rule_results_pkey PRIMARY KEY (id),
	CONSTRAINT earning_rule_results_base_points_check CHECK ((base_points >= (0)::numeric)),
	CONSTRAINT earning_rule_results_final_points_check CHECK ((final_points >= (0)::numeric))
);
CREATE INDEX IF NOT EXISTS idx_rule_results_txn_id ON schema_earning.earning_rule_results USING btree (earning_transaction_id);
CREATE INDEX IF NOT EXISTS idx_rule_results_rule_id ON schema_earning.earning_rule_results USING btree (rule_id);
CREATE INDEX IF NOT EXISTS idx_rule_results_created_at ON schema_earning.earning_rule_results USING btree (created_at DESC);

-- 3. point_expiry_buckets
CREATE TABLE IF NOT EXISTS schema_earning.point_expiry_buckets (
	id uuid DEFAULT gen_random_uuid() NOT NULL,
	member_id uuid NOT NULL,
	earning_transaction_id uuid NULL,
	ledger_entry_id uuid NULL,
	original_points numeric(15, 2) NOT NULL,
	remaining_points numeric(15, 2) NOT NULL,
	earned_at timestamptz NOT NULL,
	expires_at timestamptz NOT NULL,
	expiry_policy varchar(30) DEFAULT 'FIXED'::character varying NOT NULL,
	status varchar(20) DEFAULT 'ACTIVE'::character varying NOT NULL,
	expired_at timestamptz NULL,
	consumed_at timestamptz NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	updated_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT point_expiry_buckets_pkey PRIMARY KEY (id),
	CONSTRAINT point_expiry_buckets_original_points_check CHECK ((original_points > (0)::numeric)),
	CONSTRAINT point_expiry_buckets_remaining_points_check CHECK ((remaining_points >= (0)::numeric)),
	CONSTRAINT point_expiry_buckets_expiry_policy_check CHECK (((expiry_policy)::text = ANY ((ARRAY['FIXED'::character varying, 'ROLLING'::character varying, 'END_OF_YEAR'::character varying])::text[]))),
	CONSTRAINT point_expiry_buckets_status_check CHECK (((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'PARTIALLY_CONSUMED'::character varying, 'FULLY_CONSUMED'::character varying, 'EXPIRED'::character varying, 'EXTENDED'::character varying])::text[])))
);
CREATE INDEX IF NOT EXISTS idx_expiry_buckets_member_id ON schema_earning.point_expiry_buckets USING btree (member_id, expires_at ASC);
CREATE INDEX IF NOT EXISTS idx_expiry_buckets_expiring ON schema_earning.point_expiry_buckets USING btree (expires_at ASC) WHERE ((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'PARTIALLY_CONSUMED'::character varying])::text[]));
CREATE INDEX IF NOT EXISTS idx_expiry_buckets_member_active ON schema_earning.point_expiry_buckets USING btree (member_id) WHERE ((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'PARTIALLY_CONSUMED'::character varying])::text[]));

-- 4. earning_caps
CREATE TABLE IF NOT EXISTS schema_earning.earning_caps (
	id uuid DEFAULT gen_random_uuid() NOT NULL,
	rule_id uuid NULL,
	cap_scope varchar(20) NOT NULL,
	cap_type varchar(20) NOT NULL,
	period_type varchar(20) NOT NULL,
	max_points numeric(15, 2) NOT NULL,
	max_transactions int4 NULL,
	is_active bool DEFAULT true NOT NULL,
	created_by uuid NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	updated_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT earning_caps_pkey PRIMARY KEY (id),
	CONSTRAINT earning_caps_cap_scope_check CHECK (((cap_scope)::text = ANY ((ARRAY['GLOBAL'::character varying, 'PER_MEMBER'::character varying, 'PER_RULE'::character varying, 'PER_MEMBER_PER_RULE'::character varying])::text[]))),
	CONSTRAINT earning_caps_cap_type_check CHECK (((cap_type)::text = ANY ((ARRAY['POINTS'::character varying, 'TRANSACTIONS'::character varying, 'BOTH'::character varying])::text[]))),
	CONSTRAINT earning_caps_period_type_check CHECK (((period_type)::text = ANY ((ARRAY['DAILY'::character varying, 'WEEKLY'::character varying, 'MONTHLY'::character varying, 'YEARLY'::character varying, 'LIFETIME'::character varying])::text[]))),
	CONSTRAINT earning_caps_max_points_check CHECK ((max_points > (0)::numeric))
);
CREATE INDEX IF NOT EXISTS idx_earning_caps_rule_id ON schema_earning.earning_caps USING btree (rule_id) WHERE (rule_id IS NOT NULL);
CREATE INDEX IF NOT EXISTS idx_earning_caps_active ON schema_earning.earning_caps USING btree (cap_scope, period_type) WHERE (is_active = true);

-- 5. earning_cap_counters
CREATE TABLE IF NOT EXISTS schema_earning.earning_cap_counters (
	id uuid DEFAULT gen_random_uuid() NOT NULL,
	cap_id uuid NOT NULL,
	member_id uuid NOT NULL,
	rule_id uuid NULL,
	period_start timestamptz NOT NULL,
	period_end timestamptz NOT NULL,
	accumulated_points numeric(15, 2) DEFAULT 0.00 NOT NULL,
	transaction_count int4 DEFAULT 0 NOT NULL,
	last_updated_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT earning_cap_counters_pkey PRIMARY KEY (id),
	CONSTRAINT earning_cap_counters_unique_window UNIQUE (cap_id, member_id, rule_id, period_start),
	CONSTRAINT earning_cap_counters_accumulated_points_check CHECK ((accumulated_points >= (0)::numeric)),
	CONSTRAINT earning_cap_counters_transaction_count_check CHECK ((transaction_count >= 0))
);
CREATE INDEX IF NOT EXISTS idx_cap_counters_member_period ON schema_earning.earning_cap_counters USING btree (member_id, period_start DESC);
CREATE INDEX IF NOT EXISTS idx_cap_counters_cap_id ON schema_earning.earning_cap_counters USING btree (cap_id, period_start DESC);

-- 6. expiry_notifications
CREATE TABLE IF NOT EXISTS schema_earning.expiry_notifications (
	id uuid DEFAULT gen_random_uuid() NOT NULL,
	member_id uuid NOT NULL,
	expiry_bucket_id uuid NOT NULL,
	notification_type varchar(30) NOT NULL,
	scheduled_at timestamptz NOT NULL,
	sent_at timestamptz NULL,
	status varchar(20) DEFAULT 'SCHEDULED'::character varying NOT NULL,
	channel varchar(20) DEFAULT 'PUSH'::character varying NOT NULL,
	retry_count int4 DEFAULT 0 NOT NULL,
	error_message text NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT expiry_notifications_pkey PRIMARY KEY (id),
	CONSTRAINT expiry_notifications_notification_type_check CHECK (((notification_type)::text = ANY ((ARRAY['EXPIRY_WARNING_30D'::character varying, 'EXPIRY_WARNING_14D'::character varying, 'EXPIRY_WARNING_7D'::character varying, 'EXPIRY_WARNING_3D'::character varying, 'EXPIRY_WARNING_1D'::character varying, 'EXPIRED'::character varying])::text[]))),
	CONSTRAINT expiry_notifications_status_check CHECK (((status)::text = ANY ((ARRAY['SCHEDULED'::character varying, 'SENT'::character varying, 'FAILED'::character varying, 'CANCELLED'::character varying])::text[]))),
	CONSTRAINT expiry_notifications_channel_check CHECK (((channel)::text = ANY ((ARRAY['PUSH'::character varying, 'EMAIL'::character varying, 'SMS'::character varying, 'WEBHOOK'::character varying, 'IN_APP'::character varying])::text[])))
);
CREATE INDEX IF NOT EXISTS idx_expiry_notif_scheduled ON schema_earning.expiry_notifications USING btree (scheduled_at ASC) WHERE ((status)::text = 'SCHEDULED'::text);
CREATE INDEX IF NOT EXISTS idx_expiry_notif_member ON schema_earning.expiry_notifications USING btree (member_id);
CREATE INDEX IF NOT EXISTS idx_expiry_notif_bucket ON schema_earning.expiry_notifications USING btree (expiry_bucket_id);

-- 7. earning_reversals
CREATE TABLE IF NOT EXISTS schema_earning.earning_reversals (
	id uuid DEFAULT gen_random_uuid() NOT NULL,
	original_transaction_id uuid NOT NULL,
	member_id uuid NOT NULL,
	reversal_reason varchar(50) NOT NULL,
	reversed_points numeric(15, 2) NOT NULL,
	reversal_ref varchar(128) NULL,
	idempotency_key varchar(128) NOT NULL,
	reversed_by uuid NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT earning_reversals_pkey PRIMARY KEY (id),
	CONSTRAINT earning_reversals_reason_check CHECK (((reversal_reason)::text = ANY ((ARRAY['REFUND'::character varying, 'CHARGEBACK'::character varying, 'FRAUD'::character varying, 'MANUAL_ADJUSTMENT'::character varying, 'SYSTEM_ERROR'::character varying])::text[]))),
	CONSTRAINT earning_reversals_reversed_points_check CHECK ((reversed_points > (0)::numeric))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_earning_reversals_idempotency ON schema_earning.earning_reversals USING btree (idempotency_key);
CREATE INDEX IF NOT EXISTS idx_earning_reversals_original ON schema_earning.earning_reversals USING btree (original_transaction_id);
CREATE INDEX IF NOT EXISTS idx_earning_reversals_member ON schema_earning.earning_reversals USING btree (member_id, created_at DESC);

-- 8. earning_daily_summary
CREATE TABLE IF NOT EXISTS schema_earning.earning_daily_summary (
	id uuid DEFAULT gen_random_uuid() NOT NULL,
	member_id uuid NOT NULL,
	summary_date date NOT NULL,
	total_transactions int4 DEFAULT 0 NOT NULL,
	total_transaction_amount numeric(15, 2) DEFAULT 0.00 NOT NULL,
	total_base_points numeric(15, 2) DEFAULT 0.00 NOT NULL,
	total_bonus_points numeric(15, 2) DEFAULT 0.00 NOT NULL,
	total_final_points numeric(15, 2) DEFAULT 0.00 NOT NULL,
	top_rule_id uuid NULL,
	top_rule_code varchar(50) NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	updated_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT earning_daily_summary_pkey PRIMARY KEY (id),
	CONSTRAINT earning_daily_summary_unique UNIQUE (member_id, summary_date)
);
CREATE INDEX IF NOT EXISTS idx_earning_daily_summary_date ON schema_earning.earning_daily_summary USING btree (summary_date DESC);
CREATE INDEX IF NOT EXISTS idx_earning_daily_summary_member ON schema_earning.earning_daily_summary USING btree (member_id, summary_date DESC);

