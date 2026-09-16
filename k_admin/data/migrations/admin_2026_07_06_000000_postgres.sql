CREATE SEQUENCE public.goadmin_dict_type_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;

ALTER TABLE public.goadmin_dict_type_myid_seq OWNER TO postgres;

CREATE TABLE public.goadmin_dict_type (
    id integer DEFAULT nextval('public.goadmin_dict_type_myid_seq'::regclass) NOT NULL,
    name character varying(100) NOT NULL,
    code character varying(100) NOT NULL,
    description character varying(3000),
    sort integer DEFAULT 0 NOT NULL,
    status integer DEFAULT 1 NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);

ALTER TABLE public.goadmin_dict_type OWNER TO postgres;

ALTER TABLE ONLY public.goadmin_dict_type
    ADD CONSTRAINT goadmin_dict_type_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.goadmin_dict_type
    ADD CONSTRAINT goadmin_dict_type_code_unique UNIQUE (code);

CREATE INDEX goadmin_dict_type_status_index ON public.goadmin_dict_type USING btree (status);

CREATE SEQUENCE public.goadmin_dict_data_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;

ALTER TABLE public.goadmin_dict_data_myid_seq OWNER TO postgres;

CREATE TABLE public.goadmin_dict_data (
    id integer DEFAULT nextval('public.goadmin_dict_data_myid_seq'::regclass) NOT NULL,
    dict_type character varying(100) NOT NULL,
    label character varying(100) NOT NULL,
    value character varying(100) NOT NULL,
    color character varying(50) DEFAULT ''::character varying NOT NULL,
    css_class character varying(100) DEFAULT ''::character varying NOT NULL,
    is_default integer DEFAULT 0 NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    status integer DEFAULT 1 NOT NULL,
    remark character varying(3000),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);

ALTER TABLE public.goadmin_dict_data OWNER TO postgres;

ALTER TABLE ONLY public.goadmin_dict_data
    ADD CONSTRAINT goadmin_dict_data_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.goadmin_dict_data
    ADD CONSTRAINT goadmin_dict_data_type_value_unique UNIQUE (dict_type, value);

CREATE INDEX goadmin_dict_data_type_index ON public.goadmin_dict_data USING btree (dict_type);
CREATE INDEX goadmin_dict_data_status_index ON public.goadmin_dict_data USING btree (status);

INSERT INTO public.goadmin_dict_type (id, name, code, description, sort, status, created_at, updated_at)
VALUES
    (1,'Gender','sys_gender','Common gender dictionary',1,1,'2026-07-06 00:00:00','2026-07-06 00:00:00'),
    (2,'Status','sys_status','Common enable/disable status dictionary',2,1,'2026-07-06 00:00:00','2026-07-06 00:00:00');

INSERT INTO public.goadmin_dict_data (id, dict_type, label, value, color, css_class, is_default, sort, status, remark, created_at, updated_at)
VALUES
    (1,'sys_gender','Male','male','blue','',1,1,1,'','2026-07-06 00:00:00','2026-07-06 00:00:00'),
    (2,'sys_gender','Female','female','magenta','',0,2,1,'','2026-07-06 00:00:00','2026-07-06 00:00:00'),
    (3,'sys_status','Enable','enable','green','',1,1,1,'','2026-07-06 00:00:00','2026-07-06 00:00:00'),
    (4,'sys_status','Disable','disable','red','',0,2,1,'','2026-07-06 00:00:00','2026-07-06 00:00:00');

SELECT pg_catalog.setval('public.goadmin_dict_type_myid_seq', 2, true);
SELECT pg_catalog.setval('public.goadmin_dict_data_myid_seq', 4, true);
