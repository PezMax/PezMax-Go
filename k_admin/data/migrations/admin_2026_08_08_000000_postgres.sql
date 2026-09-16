-- Store the Vben page component path on the menu row so generated pages do not
-- require a backend code change to the static route binding map.
ALTER TABLE public.goadmin_menu
    ADD COLUMN IF NOT EXISTS component character varying(255)
        NOT NULL DEFAULT ''::character varying;
