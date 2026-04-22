DROP TABLE IF EXISTS admin_logs;
DROP TABLE IF EXISTS stripe_transactions;
ALTER TABLE users
    DROP COLUMN IF EXISTS is_admin,
    DROP COLUMN IF EXISTS subscription_ends_at,
    DROP COLUMN IF EXISTS subscription_tier,
    DROP COLUMN IF EXISTS subscription_status,
    DROP COLUMN IF EXISTS stripe_customer_id;
