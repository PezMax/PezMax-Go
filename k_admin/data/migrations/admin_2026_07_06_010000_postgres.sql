ALTER TABLE public.goadmin_users
    ADD COLUMN status character varying(50) DEFAULT 'enable'::character varying NOT NULL;

UPDATE public.goadmin_users SET status = 'enable' WHERE status = '';
