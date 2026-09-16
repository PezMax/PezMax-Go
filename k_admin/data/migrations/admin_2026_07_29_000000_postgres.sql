-- Extend KAdmin login audit results for server captcha and administrator unlock events.
-- Runtime initialization performs the same idempotent change for existing deployments.

BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'kadmin_login_audits_result_check'
          AND pg_get_constraintdef(oid) LIKE '%captcha_invalid%'
          AND pg_get_constraintdef(oid) LIKE '%account_unlocked%'
    ) THEN
        ALTER TABLE public.kadmin_login_audits
            DROP CONSTRAINT IF EXISTS kadmin_login_audits_result_check;
        ALTER TABLE public.kadmin_login_audits
            ADD CONSTRAINT kadmin_login_audits_result_check
            CHECK (result IN (
                'success', 'account_not_found', 'invalid_password',
                'account_disabled', 'account_locked', 'account_unlocked',
                'captcha_invalid', 'system_error'
            ));
    END IF;
END $$;

COMMIT;
