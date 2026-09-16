BEGIN;

ALTER TABLE public.goadmin_menu
    DROP COLUMN IF EXISTS component;

COMMIT;
