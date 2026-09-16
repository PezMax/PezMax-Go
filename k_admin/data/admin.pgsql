--
-- PostgreSQL database dump
--

-- Dumped from database version 9.5.14
-- Dumped by pg_dump version 10.5

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: plpgsql; Type: EXTENSION; Schema: -; Owner: 
--

CREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;


--
-- Name: EXTENSION plpgsql; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION plpgsql IS 'PL/pgSQL procedural language';


--
-- Name: goadmin_menu_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_menu_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER TABLE public.goadmin_menu_myid_seq OWNER TO postgres;

SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: goadmin_menu; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_menu (
    id integer DEFAULT nextval('public.goadmin_menu_myid_seq'::regclass) NOT NULL,
    parent_id integer DEFAULT 0 NOT NULL,
    type integer DEFAULT 0,
    "order" integer DEFAULT 0 NOT NULL,
    title character varying(50) NOT NULL,
    header character varying(100),
    plugin_name character varying(100) NOT NULL,
    icon character varying(50) NOT NULL,
    uri character varying(3000) NOT NULL,
    uuid character varying(100),
    component character varying(255) NOT NULL DEFAULT ''::character varying,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_menu OWNER TO postgres;

--
-- Name: goadmin_operation_log_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_operation_log_myid_seq
    AS bigint
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.goadmin_operation_log_myid_seq OWNER TO postgres;

--
-- Name: goadmin_operation_log; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_operation_log (
    id bigint DEFAULT nextval('public.goadmin_operation_log_myid_seq'::regclass) NOT NULL,
    user_id integer,
    path character varying(2048) DEFAULT ''::character varying NOT NULL,
    method character varying(16) DEFAULT ''::character varying NOT NULL,
    ip character varying(45) DEFAULT ''::character varying NOT NULL,
    input text DEFAULT ''::text NOT NULL,
    event_id character varying(64),
    event_type character varying(32) DEFAULT 'operation'::character varying NOT NULL,
    level character varying(16) DEFAULT 'info'::character varying NOT NULL,
    source character varying(100) DEFAULT 'goadmin'::character varying NOT NULL,
    module character varying(100) DEFAULT ''::character varying NOT NULL,
    action character varying(100) DEFAULT ''::character varying NOT NULL,
    message text DEFAULT ''::text NOT NULL,
    actor_name character varying(100) DEFAULT ''::character varying NOT NULL,
    request_id character varying(64),
    trace_id character varying(64),
    status_code smallint,
    success boolean,
    duration_ms bigint,
    error_code character varying(100),
    error_message text,
    user_agent text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    occurred_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    expires_at timestamp with time zone,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    CONSTRAINT goadmin_operation_log_event_type_check CHECK (((event_type)::text = ANY ((ARRAY['operation'::character varying, 'request'::character varying, 'auth'::character varying, 'audit'::character varying, 'system'::character varying])::text[]))),
    CONSTRAINT goadmin_operation_log_level_check CHECK (((level)::text = ANY ((ARRAY['debug'::character varying, 'info'::character varying, 'warn'::character varying, 'error'::character varying, 'fatal'::character varying])::text[]))),
    CONSTRAINT goadmin_operation_log_status_code_check CHECK (((status_code IS NULL) OR ((status_code >= 100) AND (status_code <= 599)))),
    CONSTRAINT goadmin_operation_log_duration_ms_check CHECK (((duration_ms IS NULL) OR (duration_ms >= 0))),
    CONSTRAINT goadmin_operation_log_metadata_check CHECK ((jsonb_typeof(metadata) = 'object'::text)),
    CONSTRAINT goadmin_operation_log_expires_at_check CHECK (((expires_at IS NULL) OR (expires_at >= occurred_at)))
);


ALTER TABLE public.goadmin_operation_log OWNER TO postgres;

COMMENT ON TABLE public.goadmin_operation_log IS 'Operation and listener events for GoAdmin and Vben API';
COMMENT ON COLUMN public.goadmin_operation_log.event_id IS 'Producer-generated idempotency key; NULL for legacy synchronous events';
COMMENT ON COLUMN public.goadmin_operation_log.user_id IS 'Optional actor id; no foreign key so audit history survives user deletion';
COMMENT ON COLUMN public.goadmin_operation_log.actor_name IS 'Actor name snapshot captured when the event occurs';
COMMENT ON COLUMN public.goadmin_operation_log.message IS 'Sanitized human-readable event summary';
COMMENT ON COLUMN public.goadmin_operation_log.input IS 'Redacted request or operation input; credentials and tokens must not be stored';
COMMENT ON COLUMN public.goadmin_operation_log.metadata IS 'Sanitized event-specific attributes as a JSON object';
COMMENT ON COLUMN public.goadmin_operation_log.occurred_at IS 'Canonical event time with time zone';
COMMENT ON COLUMN public.goadmin_operation_log.expires_at IS 'Optional retention deadline used by a future cleanup job';

--
-- Name: goadmin_site_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_site_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER TABLE public.goadmin_site_myid_seq OWNER TO postgres;

--
-- Name: goadmin_site; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_site (
    id integer DEFAULT nextval('public.goadmin_site_myid_seq'::regclass) NOT NULL,
    key character varying(100) NOT NULL,
    value text NOT NULL,
    type integer DEFAULT 0,
    description character varying(3000),
    state integer DEFAULT 0,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_site OWNER TO postgres;

--
-- Name: goadmin_department_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_department_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER TABLE public.goadmin_department_myid_seq OWNER TO postgres;

--
-- Name: goadmin_department; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: goadmin_department_roles; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_department_roles (
    department_id integer NOT NULL,
    role_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_department_roles OWNER TO postgres;

--
-- Name: goadmin_dict_type_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_dict_type_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER TABLE public.goadmin_dict_type_myid_seq OWNER TO postgres;

--
-- Name: goadmin_dict_type; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: goadmin_dict_data_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_dict_data_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER TABLE public.goadmin_dict_data_myid_seq OWNER TO postgres;

--
-- Name: goadmin_dict_data; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: goadmin_permissions_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_permissions_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER TABLE public.goadmin_permissions_myid_seq OWNER TO postgres;

--
-- Name: goadmin_permissions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_permissions (
    id integer DEFAULT nextval('public.goadmin_permissions_myid_seq'::regclass) NOT NULL,
    name character varying(50) NOT NULL,
    slug character varying(50) NOT NULL,
    http_method character varying(255),
    http_path text NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_permissions OWNER TO postgres;

--
-- Name: goadmin_role_menu; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_role_menu (
    role_id integer NOT NULL,
    menu_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_role_menu OWNER TO postgres;

--
-- Name: goadmin_role_permissions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_role_permissions (
    role_id integer NOT NULL,
    permission_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_role_permissions OWNER TO postgres;

--
-- Name: goadmin_role_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_role_users (
    role_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_role_users OWNER TO postgres;

--
-- Name: goadmin_roles_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_roles_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER TABLE public.goadmin_roles_myid_seq OWNER TO postgres;

--
-- Name: goadmin_roles; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_roles (
    id integer DEFAULT nextval('public.goadmin_roles_myid_seq'::regclass) NOT NULL,
    name character varying NOT NULL,
    slug character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_roles OWNER TO postgres;

--
-- Name: goadmin_session_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_session_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER TABLE public.goadmin_session_myid_seq OWNER TO postgres;

--
-- Name: goadmin_session; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_session (
    id integer DEFAULT nextval('public.goadmin_session_myid_seq'::regclass) NOT NULL,
    sid character varying(50) NOT NULL,
    "values" character varying(3000) NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_session OWNER TO postgres;

--
-- Name: goadmin_user_permissions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_user_permissions (
    user_id integer NOT NULL,
    permission_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_user_permissions OWNER TO postgres;

--
-- Name: goadmin_users_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_users_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER TABLE public.goadmin_users_myid_seq OWNER TO postgres;

--
-- Name: goadmin_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.goadmin_users (
    id integer DEFAULT nextval('public.goadmin_users_myid_seq'::regclass) NOT NULL,
    username character varying(100) NOT NULL,
    password character varying(100) NOT NULL,
    name character varying(100) NOT NULL,
    avatar character varying(255),
    status character varying(50) DEFAULT 'enable'::character varying NOT NULL,
    remember_token character varying(100),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.goadmin_users OWNER TO postgres;

--
-- Data for Name: goadmin_menu; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_menu (id, parent_id, type, "order", title, plugin_name, header, icon, uri, created_at, updated_at) FROM stdin;
1	0	1	1	Dashboard		\N	lucide:layout-dashboard	/dashboard	2019-09-10 00:00:00	2019-09-10 00:00:00
2	1	1	1	分析页		\N	lucide:area-chart	/dashboard/analytics	2019-09-10 00:00:00	2019-09-10 00:00:00
3	1	1	2	工作台		\N	carbon:workspace	/dashboard/workspace	2019-09-10 00:00:00	2019-09-10 00:00:00
4	0	1	10	KAdmin 管理		\N	lucide:settings-2	/kadmin	2019-09-10 00:00:00	2019-09-10 00:00:00
5	4	1	1	用户管理		\N	lucide:users	/kadmin/users	2019-09-10 00:00:00	2019-09-10 00:00:00
6	4	1	2	权限管理		\N	lucide:shield-check	/kadmin/rbac	2019-09-10 00:00:00	2019-09-10 00:00:00
7	4	1	3	菜单管理		\N	lucide:menu	/kadmin/menus	2019-09-10 00:00:00	2019-09-10 00:00:00
8	4	1	4	字典管理		\N	lucide:book-open	/kadmin/dictionary	2019-09-10 00:00:00	2019-09-10 00:00:00
9	4	1	5	参数配置		\N	lucide:sliders-horizontal	/kadmin/settings	2019-09-10 00:00:00	2019-09-10 00:00:00
10	4	1	6	资源工作台		\N	lucide:folder-kanban	/kadmin/resources	2019-09-10 00:00:00	2019-09-10 00:00:00
11	4	1	7	日志管理		\N	lucide:scroll-text	/kadmin/logs	2026-07-22 00:00:00	2026-07-22 00:00:00
\.


--
-- Data for Name: goadmin_operation_log; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_operation_log (id, user_id, path, method, ip, input, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: goadmin_site; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_site (id, key, value, description, state, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: goadmin_department; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_department (id, name, code, description, sort, status, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: goadmin_department_roles; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_department_roles (department_id, role_id, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: goadmin_dict_type; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_dict_type (id, name, code, description, sort, status, created_at, updated_at) FROM stdin;
1	Gender	sys_gender	Common gender dictionary	1	1	2026-07-06 00:00:00	2026-07-06 00:00:00
2	Status	sys_status	Common enable/disable status dictionary	2	1	2026-07-06 00:00:00	2026-07-06 00:00:00
\.


--
-- Data for Name: goadmin_dict_data; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_dict_data (id, dict_type, label, value, color, css_class, is_default, sort, status, remark, created_at, updated_at) FROM stdin;
1	sys_gender	Male	male	blue		1	1	1		2026-07-06 00:00:00	2026-07-06 00:00:00
2	sys_gender	Female	female	magenta		0	2	1		2026-07-06 00:00:00	2026-07-06 00:00:00
3	sys_status	Enable	enable	green		1	1	1		2026-07-06 00:00:00	2026-07-06 00:00:00
4	sys_status	Disable	disable	red		0	2	1		2026-07-06 00:00:00	2026-07-06 00:00:00
\.


--
-- Data for Name: goadmin_permissions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_permissions (id, name, slug, http_method, http_path, created_at, updated_at) FROM stdin;
1	All permission	*		*	2019-09-10 00:00:00	2019-09-10 00:00:00
2	Dashboard	dashboard	GET,PUT,POST,DELETE	/	2019-09-10 00:00:00	2019-09-10 00:00:00
3	查看请求日志	system:log:list	GET	/api/logs*	2026-07-22 00:00:00	2026-07-22 00:00:00
4	删除请求日志	system:log:delete	DELETE	/api/logs*	2026-07-22 00:00:00	2026-07-22 00:00:00
\.


--
-- Data for Name: goadmin_role_menu; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_role_menu (role_id, menu_id, created_at, updated_at) FROM stdin;
1	1	2019-09-10 00:00:00	2019-09-10 00:00:00
1	2	2019-09-10 00:00:00	2019-09-10 00:00:00
1	3	2019-09-10 00:00:00	2019-09-10 00:00:00
1	4	2019-09-10 00:00:00	2019-09-10 00:00:00
1	5	2019-09-10 00:00:00	2019-09-10 00:00:00
1	6	2019-09-10 00:00:00	2019-09-10 00:00:00
1	7	2019-09-10 00:00:00	2019-09-10 00:00:00
1	8	2019-09-10 00:00:00	2019-09-10 00:00:00
1	9	2019-09-10 00:00:00	2019-09-10 00:00:00
1	10	2019-09-10 00:00:00	2019-09-10 00:00:00
1	11	2026-07-22 00:00:00	2026-07-22 00:00:00
2	1	2019-09-10 00:00:00	2019-09-10 00:00:00
2	2	2019-09-10 00:00:00	2019-09-10 00:00:00
2	3	2019-09-10 00:00:00	2019-09-10 00:00:00
\.


--
-- Data for Name: goadmin_role_permissions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_role_permissions (role_id, permission_id, created_at, updated_at) FROM stdin;
1	1	2019-09-10 00:00:00	2019-09-10 00:00:00
1	2	2019-09-10 00:00:00	2019-09-10 00:00:00
2	2	2019-09-10 00:00:00	2019-09-10 00:00:00
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
0	3	\N	\N
\.


--
-- Data for Name: goadmin_role_users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_role_users (role_id, user_id, created_at, updated_at) FROM stdin;
1	1	2019-09-10 00:00:00	2019-09-10 00:00:00
2	2	2019-09-10 00:00:00	2019-09-10 00:00:00
\.


--
-- Data for Name: goadmin_roles; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_roles (id, name, slug, created_at, updated_at) FROM stdin;
1	Administrator	administrator	2019-09-10 00:00:00	2019-09-10 00:00:00
2	Operator	operator	2019-09-10 00:00:00	2019-09-10 00:00:00
\.


--
-- Data for Name: goadmin_session; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_session (id, sid, "values", created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: goadmin_user_permissions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_user_permissions (user_id, permission_id, created_at, updated_at) FROM stdin;
1	1	2019-09-10 00:00:00	2019-09-10 00:00:00
2	2	2019-09-10 00:00:00	2019-09-10 00:00:00
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
0	1	\N	\N
\.


--
-- Data for Name: goadmin_users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_users (id, username, password, name, avatar, status, remember_token, created_at, updated_at) FROM stdin;
1	admin	$2a$10$OxWYJJGTP2gi00l2x06QuOWqw5VR47MQCJ0vNKnbMYfrutij10Hwe	admin		enable	tlNcBVK9AvfYH7WEnwB1RKvocJu8FfRy4um3DJtwdHuJy0dwFsLOgAc0xUfh	2019-09-10 00:00:00	2019-09-10 00:00:00
2	operator	$2a$10$rVqkOzHjN2MdlEprRflb1eGP0oZXuSrbJLOmJagFsCd81YZm0bsh.	Operator		enable	\N	2019-09-10 00:00:00	2019-09-10 00:00:00
\.


--
-- Name: goadmin_menu_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_menu_myid_seq', 11, true);


--
-- Name: goadmin_operation_log_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_operation_log_myid_seq', 1, true);


--
-- Name: goadmin_permissions_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_permissions_myid_seq', 4, true);


--
-- Name: goadmin_roles_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_roles_myid_seq', 2, true);


--
-- Name: goadmin_site_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_site_myid_seq', 1, true);


--
-- Name: goadmin_department_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_department_myid_seq', 1, false);


--
-- Name: goadmin_dict_type_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_dict_type_myid_seq', 2, true);


--
-- Name: goadmin_dict_data_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_dict_data_myid_seq', 4, true);


--
-- Name: goadmin_session_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_session_myid_seq', 1, true);


--
-- Name: goadmin_users_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_users_myid_seq', 2, true);


--
-- Name: goadmin_menu goadmin_menu_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_menu
    ADD CONSTRAINT goadmin_menu_pkey PRIMARY KEY (id);


--
-- Name: goadmin_operation_log goadmin_operation_log_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_operation_log
    ADD CONSTRAINT goadmin_operation_log_pkey PRIMARY KEY (id);


CREATE UNIQUE INDEX goadmin_operation_log_event_id_unique
    ON public.goadmin_operation_log USING btree (event_id)
    WHERE ((event_id IS NOT NULL) AND ((event_id)::text <> ''::text));

CREATE INDEX goadmin_operation_log_occurred_at_index
    ON public.goadmin_operation_log USING btree (occurred_at DESC, id DESC);

CREATE INDEX goadmin_operation_log_event_type_time_index
    ON public.goadmin_operation_log USING btree (event_type, occurred_at DESC, id DESC);

CREATE INDEX goadmin_operation_log_user_time_index
    ON public.goadmin_operation_log USING btree (user_id, occurred_at DESC, id DESC)
    WHERE (user_id IS NOT NULL);

CREATE INDEX goadmin_operation_log_failure_time_index
    ON public.goadmin_operation_log USING btree (occurred_at DESC, id DESC)
    WHERE (success = false);

CREATE INDEX goadmin_operation_log_request_id_index
    ON public.goadmin_operation_log USING btree (request_id)
    WHERE ((request_id IS NOT NULL) AND ((request_id)::text <> ''::text));

CREATE INDEX goadmin_operation_log_trace_id_index
    ON public.goadmin_operation_log USING btree (trace_id)
    WHERE ((trace_id IS NOT NULL) AND ((trace_id)::text <> ''::text));

CREATE INDEX goadmin_operation_log_expires_at_index
    ON public.goadmin_operation_log USING btree (expires_at)
    WHERE (expires_at IS NOT NULL);


--
-- Name: goadmin_permissions goadmin_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_permissions
    ADD CONSTRAINT goadmin_permissions_pkey PRIMARY KEY (id);


--
-- Name: goadmin_roles goadmin_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_roles
    ADD CONSTRAINT goadmin_roles_pkey PRIMARY KEY (id);


--
-- Name: goadmin_site goadmin_site_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_site
    ADD CONSTRAINT goadmin_site_pkey PRIMARY KEY (id);


--
-- Name: goadmin_department goadmin_department_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_department
    ADD CONSTRAINT goadmin_department_pkey PRIMARY KEY (id);


--
-- Name: goadmin_department_roles goadmin_department_roles_unique; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_department_roles
    ADD CONSTRAINT goadmin_department_roles_unique UNIQUE (department_id, role_id);


--
-- Name: goadmin_dict_type goadmin_dict_type_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_dict_type
    ADD CONSTRAINT goadmin_dict_type_pkey PRIMARY KEY (id);


--
-- Name: goadmin_dict_type goadmin_dict_type_code_unique; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_dict_type
    ADD CONSTRAINT goadmin_dict_type_code_unique UNIQUE (code);


--
-- Name: goadmin_dict_data goadmin_dict_data_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_dict_data
    ADD CONSTRAINT goadmin_dict_data_pkey PRIMARY KEY (id);


--
-- Name: goadmin_dict_data goadmin_dict_data_type_value_unique; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_dict_data
    ADD CONSTRAINT goadmin_dict_data_type_value_unique UNIQUE (dict_type, value);


--
-- Name: goadmin_department_code_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX goadmin_department_code_index ON public.goadmin_department USING btree (code);


--
-- Name: goadmin_department_roles_role_id_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX goadmin_department_roles_role_id_index ON public.goadmin_department_roles USING btree (role_id);


--
-- Name: goadmin_dict_type_status_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX goadmin_dict_type_status_index ON public.goadmin_dict_type USING btree (status);


--
-- Name: goadmin_dict_data_type_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX goadmin_dict_data_type_index ON public.goadmin_dict_data USING btree (dict_type);


--
-- Name: goadmin_dict_data_status_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX goadmin_dict_data_status_index ON public.goadmin_dict_data USING btree (status);


--
-- Name: goadmin_session goadmin_session_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_session
    ADD CONSTRAINT goadmin_session_pkey PRIMARY KEY (id);


--
-- Name: goadmin_users goadmin_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.goadmin_users
    ADD CONSTRAINT goadmin_users_pkey PRIMARY KEY (id);


--
-- Name: SCHEMA public; Type: ACL; Schema: -; Owner: postgres
--

REVOKE ALL ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM postgres;
GRANT ALL ON SCHEMA public TO postgres;
GRANT ALL ON SCHEMA public TO PUBLIC;


--
-- PostgreSQL database dump complete
--

