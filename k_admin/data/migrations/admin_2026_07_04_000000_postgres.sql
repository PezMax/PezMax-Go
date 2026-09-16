CREATE SEQUENCE public.goadmin_department_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;

ALTER TABLE public.goadmin_department_myid_seq OWNER TO postgres;

CREATE TABLE public.goadmin_department (
    id integer DEFAULT nextval('public.goadmin_department_myid_seq'::regclass) NOT NULL,
    name character varying(100) NOT NULL,
    code character varying(100) DEFAULT ''::character varying NOT NULL,
    description character varying(3000),
    sort integer DEFAULT 0 NOT NULL,
    status integer DEFAULT 1 NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);

ALTER TABLE public.goadmin_department OWNER TO postgres;

CREATE INDEX goadmin_department_code_index ON public.goadmin_department USING btree (code);

ALTER TABLE ONLY public.goadmin_department
    ADD CONSTRAINT goadmin_department_pkey PRIMARY KEY (id);

CREATE TABLE public.goadmin_department_roles (
    department_id integer NOT NULL,
    role_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);

ALTER TABLE public.goadmin_department_roles OWNER TO postgres;

ALTER TABLE ONLY public.goadmin_department_roles
    ADD CONSTRAINT goadmin_department_roles_unique UNIQUE (department_id, role_id);

CREATE INDEX goadmin_department_roles_role_id_index ON public.goadmin_department_roles USING btree (role_id);
