-- Roll back KAdmin captcha and administrator unlock login audit result values.

BEGIN;

UPDATE public.kadmin_login_audits
SET result = 'system_error',
    failure_reason = CASE
        WHEN result = 'captcha_invalid' THEN '验证码校验失败（已回滚结果类型）'
        ELSE '管理员解除临时登录锁定（已回滚结果类型）'
    END
WHERE result IN ('captcha_invalid', 'account_unlocked');

ALTER TABLE public.kadmin_login_audits
    DROP CONSTRAINT IF EXISTS kadmin_login_audits_result_check;
ALTER TABLE public.kadmin_login_audits
    ADD CONSTRAINT kadmin_login_audits_result_check
    CHECK (result IN (
        'success', 'account_not_found', 'invalid_password',
        'account_disabled', 'account_locked', 'system_error'
    ));

COMMIT;
