--
-- PostgreSQL database dump
--

-- Dumped from database version 11.22
-- Dumped by pg_dump version 11.22

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

DROP INDEX IF EXISTS public.goadmin_dict_type_status_index;
DROP INDEX IF EXISTS public.goadmin_dict_data_type_index;
DROP INDEX IF EXISTS public.goadmin_dict_data_status_index;
DROP INDEX IF EXISTS public.goadmin_department_roles_role_id_index;
DROP INDEX IF EXISTS public.goadmin_department_code_index;
ALTER TABLE IF EXISTS ONLY public.users DROP CONSTRAINT IF EXISTS users_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_users DROP CONSTRAINT IF EXISTS goadmin_users_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_site DROP CONSTRAINT IF EXISTS goadmin_site_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_session DROP CONSTRAINT IF EXISTS goadmin_session_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_roles DROP CONSTRAINT IF EXISTS goadmin_roles_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_permissions DROP CONSTRAINT IF EXISTS goadmin_permissions_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_operation_log DROP CONSTRAINT IF EXISTS goadmin_operation_log_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_menu DROP CONSTRAINT IF EXISTS goadmin_menu_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_dict_type DROP CONSTRAINT IF EXISTS goadmin_dict_type_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_dict_type DROP CONSTRAINT IF EXISTS goadmin_dict_type_code_unique;
ALTER TABLE IF EXISTS ONLY public.goadmin_dict_data DROP CONSTRAINT IF EXISTS goadmin_dict_data_type_value_unique;
ALTER TABLE IF EXISTS ONLY public.goadmin_dict_data DROP CONSTRAINT IF EXISTS goadmin_dict_data_pkey;
ALTER TABLE IF EXISTS ONLY public.goadmin_department_roles DROP CONSTRAINT IF EXISTS goadmin_department_roles_unique;
ALTER TABLE IF EXISTS ONLY public.goadmin_department DROP CONSTRAINT IF EXISTS goadmin_department_pkey;
DROP TABLE IF EXISTS public.users;
DROP TABLE IF EXISTS public.user_like_books;
DROP TABLE IF EXISTS public.goadmin_users;
DROP SEQUENCE IF EXISTS public.goadmin_users_myid_seq;
DROP TABLE IF EXISTS public.goadmin_user_permissions;
DROP TABLE IF EXISTS public.goadmin_site;
DROP SEQUENCE IF EXISTS public.goadmin_site_myid_seq;
DROP TABLE IF EXISTS public.goadmin_session;
DROP SEQUENCE IF EXISTS public.goadmin_session_myid_seq;
DROP TABLE IF EXISTS public.goadmin_roles;
DROP SEQUENCE IF EXISTS public.goadmin_roles_myid_seq;
DROP TABLE IF EXISTS public.goadmin_role_users;
DROP TABLE IF EXISTS public.goadmin_role_permissions;
DROP TABLE IF EXISTS public.goadmin_role_menu;
DROP TABLE IF EXISTS public.goadmin_permissions;
DROP SEQUENCE IF EXISTS public.goadmin_permissions_myid_seq;
DROP TABLE IF EXISTS public.goadmin_operation_log;
DROP SEQUENCE IF EXISTS public.goadmin_operation_log_myid_seq;
DROP TABLE IF EXISTS public.goadmin_menu;
DROP SEQUENCE IF EXISTS public.goadmin_menu_myid_seq;
DROP TABLE IF EXISTS public.goadmin_dict_type;
DROP SEQUENCE IF EXISTS public.goadmin_dict_type_myid_seq;
DROP TABLE IF EXISTS public.goadmin_dict_data;
DROP SEQUENCE IF EXISTS public.goadmin_dict_data_myid_seq;
DROP TABLE IF EXISTS public.goadmin_department_roles;
DROP TABLE IF EXISTS public.goadmin_department;
DROP SEQUENCE IF EXISTS public.goadmin_department_myid_seq;
--
-- Name: goadmin_department_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_department_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: goadmin_department; Type: TABLE; Schema: public; Owner: -
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


--
-- Name: goadmin_department_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_department_roles (
    department_id integer NOT NULL,
    role_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: goadmin_dict_data_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_dict_data_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


--
-- Name: goadmin_dict_data; Type: TABLE; Schema: public; Owner: -
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


--
-- Name: goadmin_dict_type_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_dict_type_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


--
-- Name: goadmin_dict_type; Type: TABLE; Schema: public; Owner: -
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


--
-- Name: goadmin_menu_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_menu_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


--
-- Name: goadmin_menu; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_menu (
    id integer DEFAULT nextval('public.goadmin_menu_myid_seq'::regclass) NOT NULL,
    parent_id integer DEFAULT 0 NOT NULL,
    type integer DEFAULT 0,
    "order" integer DEFAULT 0 NOT NULL,
    title character varying(50) NOT NULL,
    header character varying(100),
    icon character varying(50) NOT NULL,
    uri character varying(50) NOT NULL,
    uuid character varying(100),
    plugin_name character varying(150) NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: goadmin_operation_log_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_operation_log_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


--
-- Name: goadmin_operation_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_operation_log (
    id integer DEFAULT nextval('public.goadmin_operation_log_myid_seq'::regclass) NOT NULL,
    user_id integer NOT NULL,
    path character varying(255) NOT NULL,
    method character varying(10) NOT NULL,
    ip character varying(15) NOT NULL,
    input text NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: goadmin_permissions_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_permissions_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


--
-- Name: goadmin_permissions; Type: TABLE; Schema: public; Owner: -
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


--
-- Name: goadmin_role_menu; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_role_menu (
    role_id integer NOT NULL,
    menu_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: goadmin_role_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_role_permissions (
    role_id integer NOT NULL,
    permission_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: goadmin_role_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_role_users (
    role_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: goadmin_roles_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_roles_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


--
-- Name: goadmin_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_roles (
    id integer DEFAULT nextval('public.goadmin_roles_myid_seq'::regclass) NOT NULL,
    name character varying NOT NULL,
    slug character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: goadmin_session_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_session_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


--
-- Name: goadmin_session; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_session (
    id integer DEFAULT nextval('public.goadmin_session_myid_seq'::regclass) NOT NULL,
    sid character varying(50) NOT NULL,
    "values" character varying(3000) NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: goadmin_site_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_site_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


--
-- Name: goadmin_site; Type: TABLE; Schema: public; Owner: -
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


--
-- Name: goadmin_user_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_user_permissions (
    user_id integer NOT NULL,
    permission_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: goadmin_users_myid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.goadmin_users_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


--
-- Name: goadmin_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goadmin_users (
    id integer DEFAULT nextval('public.goadmin_users_myid_seq'::regclass) NOT NULL,
    username character varying(190) NOT NULL,
    password character varying(80) NOT NULL,
    name character varying(255) NOT NULL,
    avatar character varying(255),
    remember_token character varying(100),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    status character varying(50) DEFAULT 'enable'::character varying NOT NULL
);


--
-- Name: user_like_books; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_like_books (
    id integer,
    user_id integer,
    name character varying,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    name character varying(100),
    homepage character varying(3000),
    email character varying(100),
    birthday timestamp with time zone,
    country character varying(50),
    city character varying(50),
    password character varying(100),
    ip character varying(20),
    certificate character varying(300),
    money integer,
    resume text,
    gender smallint,
    fruit character varying(200),
    drink character varying(200),
    experience smallint,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    member_id integer DEFAULT 0
);


--
-- Data for Name: goadmin_department; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_department (id, name, code, description, sort, status, created_at, updated_at) FROM stdin;
1	管理	onp		0	1	2026-07-04 17:35:42.862376	2026-07-04 17:35:42.862376
2	运营	onb		1	1	2026-07-04 17:42:33.067153	2026-07-04 17:42:33.067153
\.


--
-- Data for Name: goadmin_department_roles; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_department_roles (department_id, role_id, created_at, updated_at) FROM stdin;
1	1	2026-07-04 17:42:39.171653	2026-07-04 17:42:39.171653
1	2	2026-07-04 17:42:39.173343	2026-07-04 17:42:39.173343
2	3	2026-07-04 17:42:52.058756	2026-07-04 17:42:52.058756
\.


--
-- Data for Name: goadmin_dict_data; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_dict_data (id, dict_type, label, value, color, css_class, is_default, sort, status, remark, created_at, updated_at) FROM stdin;
1	sys_gender	Male	male	blue		1	1	1		2026-07-06 00:00:00	2026-07-06 00:00:00
2	sys_gender	Female	female	magenta		0	2	1		2026-07-06 00:00:00	2026-07-06 00:00:00
3	sys_status	Enable	enable	green		1	1	1		2026-07-06 00:00:00	2026-07-06 00:00:00
4	sys_status	Disable	disable	red		0	2	1		2026-07-06 00:00:00	2026-07-06 00:00:00
\.


--
-- Data for Name: goadmin_dict_type; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_dict_type (id, name, code, description, sort, status, created_at, updated_at) FROM stdin;
1	Gender	sys_gender	Common gender dictionary	1	1	2026-07-06 00:00:00	2026-07-06 00:00:00
2	Status	sys_status	Common enable/disable status dictionary	2	1	2026-07-06 00:00:00	2026-07-06 00:00:00
\.


--
-- Data for Name: goadmin_menu; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_menu (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, created_at, updated_at) FROM stdin;
1	0	1	1	Dashboard	\N	lucide:layout-dashboard	/dashboard	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
2	1	1	1	分析页	\N	lucide:area-chart	/dashboard/analytics	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
3	1	1	2	工作台	\N	carbon:workspace	/dashboard/workspace	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
4	0	1	10	KAdmin 管理	\N	lucide:settings-2	/kadmin	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
5	4	1	1	用户管理	\N	lucide:users	/kadmin/users	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
6	4	1	2	权限管理	\N	lucide:shield-check	/kadmin/rbac	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
7	4	1	3	菜单管理	\N	lucide:menu	/kadmin/menus	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
8	4	1	4	字典管理	\N	lucide:book-open	/kadmin/dictionary	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
9	4	1	5	参数配置	\N	lucide:sliders-horizontal	/kadmin/settings	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
10	4	1	6	资源工作台	\N	lucide:folder-kanban	/kadmin/resources	\N		2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
\.


--
-- Data for Name: goadmin_operation_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_operation_log (id, user_id, path, method, ip, input, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: goadmin_permissions; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_permissions (id, name, slug, http_method, http_path, created_at, updated_at) FROM stdin;
1	All permission	*		*	2019-09-10 00:00:00	2019-09-10 00:00:00
2	Dashboard	dashboard	GET,PUT,POST,DELETE	/	2019-09-10 00:00:00	2019-09-10 00:00:00
\.


--
-- Data for Name: goadmin_role_menu; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_role_menu (role_id, menu_id, created_at, updated_at) FROM stdin;
1	1	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
1	2	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
1	3	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
1	4	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
1	5	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
1	6	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
1	7	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
1	8	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
1	9	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
1	10	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
2	1	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
2	2	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
2	3	2026-07-08 17:03:06.37182	2026-07-08 17:03:06.37182
\.


--
-- Data for Name: goadmin_role_permissions; Type: TABLE DATA; Schema: public; Owner: -
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
-- Data for Name: goadmin_role_users; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_role_users (role_id, user_id, created_at, updated_at) FROM stdin;
1	1	2019-09-10 00:00:00	2019-09-10 00:00:00
3	2	2026-07-02 20:49:51.083738	2026-07-02 20:49:51.083738
1	3	2026-07-05 21:32:41.604277	2026-07-05 21:32:41.604277
\.


--
-- Data for Name: goadmin_roles; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_roles (id, name, slug, created_at, updated_at) FROM stdin;
1	Administrator	administrator	2019-09-10 00:00:00	2019-09-10 00:00:00
2	Operator	operator	2019-09-10 00:00:00	2019-09-10 00:00:00
3	运营	aaa	2026-07-02 20:49:45.011599	2026-07-02 20:49:45.011599
\.


--
-- Data for Name: goadmin_session; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_session (id, sid, "values", created_at, updated_at) FROM stdin;
2	f5a99916-36c8-4fd6-8873-6f2be8845cd0	{"user_id":1}	2019-11-27 22:26:11.917665	2019-11-27 22:26:11.917665
3	03263ffc-0043-4b89-a02f-3aa616bbf857	{"user_id":3}	2019-11-27 22:26:12.819931	2019-11-27 22:26:12.819931
\.


--
-- Data for Name: goadmin_site; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_site (id, key, value, type, description, state, created_at, updated_at) FROM stdin;
2	logger_encoder_level_key		0	\N	1	2026-07-01 20:24:07.273426	2026-07-01 20:24:07.273426
3	info_log_path		0	\N	1	2026-07-01 20:24:07.280002	2026-07-01 20:24:07.280002
4	logger_encoder_time_key		0	\N	1	2026-07-01 20:24:07.282358	2026-07-01 20:24:07.282358
5	logger_encoder_encoding		0	\N	1	2026-07-01 20:24:07.284424	2026-07-01 20:24:07.284424
6	auth_user_table	goadmin_users	0	\N	1	2026-07-01 20:24:07.286519	2026-07-01 20:24:07.286519
7	hide_visitor_user_center_entrance	false	0	\N	1	2026-07-01 20:24:07.288581	2026-07-01 20:24:07.288581
8	color_scheme	skin-black	0	\N	1	2026-07-01 20:24:07.290634	2026-07-01 20:24:07.290634
9	custom_head_html		0	\N	1	2026-07-01 20:24:07.292934	2026-07-01 20:24:07.292934
10	hide_plugin_entrance	false	0	\N	1	2026-07-01 20:24:07.295007	2026-07-01 20:24:07.295007
11	open_admin_api	false	0	\N	1	2026-07-01 20:24:07.296498	2026-07-01 20:24:07.296498
12	bootstrap_file_path		0	\N	1	2026-07-01 20:24:07.298193	2026-07-01 20:24:07.298193
13	url_prefix	admin	0	\N	1	2026-07-01 20:24:07.299736	2026-07-01 20:24:07.299736
14	logo	<b>Go</b>Admin	0	\N	1	2026-07-01 20:24:07.30126	2026-07-01 20:24:07.30126
15	custom_foot_html		0	\N	1	2026-07-01 20:24:07.302996	2026-07-01 20:24:07.302996
16	animation_delay	0.00	0	\N	1	2026-07-01 20:24:07.304648	2026-07-01 20:24:07.304648
17	hide_config_center_entrance	false	0	\N	1	2026-07-01 20:24:07.306233	2026-07-01 20:24:07.306233
18	login_logo		0	\N	1	2026-07-01 20:24:07.307674	2026-07-01 20:24:07.307674
19	hide_app_info_entrance	false	0	\N	1	2026-07-01 20:24:07.309322	2026-07-01 20:24:07.309322
20	allow_del_operation_log	false	0	\N	1	2026-07-01 20:24:07.310917	2026-07-01 20:24:07.310917
21	access_assets_log_off	false	0	\N	1	2026-07-01 20:24:07.31247	2026-07-01 20:24:07.31247
22	sql_log	false	0	\N	1	2026-07-01 20:24:07.314264	2026-07-01 20:24:07.314264
23	error_log_off	false	0	\N	1	2026-07-01 20:24:07.315725	2026-07-01 20:24:07.315725
24	logger_rotate_max_size	0	0	\N	1	2026-07-01 20:24:07.317216	2026-07-01 20:24:07.317216
25	logger_encoder_caller		0	\N	1	2026-07-01 20:24:07.319006	2026-07-01 20:24:07.319006
26	hide_tool_entrance	false	0	\N	1	2026-07-01 20:24:07.320511	2026-07-01 20:24:07.320511
27	asset_root_path	./public/	0	\N	1	2026-07-01 20:24:07.322177	2026-07-01 20:24:07.322177
28	app_id	MVIUTh0rSJQb	0	\N	1	2026-07-01 20:24:07.323694	2026-07-01 20:24:07.323694
29	domain		0	\N	1	2026-07-01 20:24:07.325377	2026-07-01 20:24:07.325377
30	logger_encoder_caller_key		0	\N	1	2026-07-01 20:24:07.326993	2026-07-01 20:24:07.326993
31	logger_encoder_stacktrace_key		0	\N	1	2026-07-01 20:24:07.328616	2026-07-01 20:24:07.328616
32	no_limit_login_ip	false	0	\N	1	2026-07-01 20:24:07.330547	2026-07-01 20:24:07.330547
33	access_log_off	false	0	\N	1	2026-07-01 20:24:07.332667	2026-07-01 20:24:07.332667
34	logger_rotate_max_age	0	0	\N	1	2026-07-01 20:24:07.334416	2026-07-01 20:24:07.334416
35	animation_type		0	\N	1	2026-07-01 20:24:07.335977	2026-07-01 20:24:07.335977
36	logger_encoder_name_key		0	\N	1	2026-07-01 20:24:07.337402	2026-07-01 20:24:07.337402
37	info_log_off	false	0	\N	1	2026-07-01 20:24:07.338858	2026-07-01 20:24:07.338858
38	logger_rotate_compress	false	0	\N	1	2026-07-01 20:24:07.340716	2026-07-01 20:24:07.340716
39	logger_encoder_level		0	\N	1	2026-07-01 20:24:07.342389	2026-07-01 20:24:07.342389
40	extra		0	\N	1	2026-07-01 20:24:07.344244	2026-07-01 20:24:07.344244
41	go_mod_file_path		0	\N	1	2026-07-01 20:24:07.345776	2026-07-01 20:24:07.345776
42	mini_logo	<b>G</b>A	0	\N	1	2026-07-01 20:24:07.347309	2026-07-01 20:24:07.347309
43	error_log_path		0	\N	1	2026-07-01 20:24:07.34876	2026-07-01 20:24:07.34876
44	custom_404_html		0	\N	1	2026-07-01 20:24:07.350098	2026-07-01 20:24:07.350098
45	index_url	/info/manager	0	\N	1	2026-07-01 20:24:07.351598	2026-07-01 20:24:07.351598
46	language	zh	0	\N	1	2026-07-01 20:24:07.352894	2026-07-01 20:24:07.352894
47	footer_info		0	\N	1	2026-07-01 20:24:07.35435	2026-07-01 20:24:07.35435
48	theme	adminlte	0	\N	1	2026-07-01 20:24:07.355843	2026-07-01 20:24:07.355843
49	login_url	/login	0	\N	1	2026-07-01 20:24:07.357279	2026-07-01 20:24:07.357279
50	logger_encoder_duration		0	\N	1	2026-07-01 20:24:07.3586	2026-07-01 20:24:07.3586
51	logger_level	0	0	\N	1	2026-07-01 20:24:07.359916	2026-07-01 20:24:07.359916
52	login_title	GoAdmin	0	\N	1	2026-07-01 20:24:07.361232	2026-07-01 20:24:07.361232
53	prohibit_config_modification	false	0	\N	1	2026-07-01 20:24:07.362593	2026-07-01 20:24:07.362593
54	operation_log_off	false	0	\N	1	2026-07-01 20:24:07.364016	2026-07-01 20:24:07.364016
55	env	local	0	\N	1	2026-07-01 20:24:07.36558	2026-07-01 20:24:07.36558
56	logger_rotate_max_backups	0	0	\N	1	2026-07-01 20:24:07.367055	2026-07-01 20:24:07.367055
57	logger_encoder_message_key		0	\N	1	2026-07-01 20:24:07.368565	2026-07-01 20:24:07.368565
58	session_life_time	7200	0	\N	1	2026-07-01 20:24:07.370026	2026-07-01 20:24:07.370026
59	access_log_path		0	\N	1	2026-07-01 20:24:07.371413	2026-07-01 20:24:07.371413
60	logger_encoder_time		0	\N	1	2026-07-01 20:24:07.372975	2026-07-01 20:24:07.372975
61	asset_url		0	\N	1	2026-07-01 20:24:07.374487	2026-07-01 20:24:07.374487
62	file_upload_engine	{"name":"local"}	0	\N	1	2026-07-01 20:24:07.375984	2026-07-01 20:24:07.375984
63	animation_duration	0.00	0	\N	1	2026-07-01 20:24:07.377476	2026-07-01 20:24:07.377476
64	custom_403_html		0	\N	1	2026-07-01 20:24:07.379553	2026-07-01 20:24:07.379553
65	exclude_theme_components	null	0	\N	1	2026-07-01 20:24:07.381548	2026-07-01 20:24:07.381548
66	title	GoAdmin	0	\N	1	2026-07-01 20:24:07.383427	2026-07-01 20:24:07.383427
67	debug	true	0	\N	1	2026-07-01 20:24:07.38536	2026-07-01 20:24:07.38536
68	site_off	false	0	\N	1	2026-07-01 20:24:07.386805	2026-07-01 20:24:07.386805
69	custom_500_html		0	\N	1	2026-07-01 20:24:07.388173	2026-07-01 20:24:07.388173
\.


--
-- Data for Name: goadmin_user_permissions; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_user_permissions (user_id, permission_id, created_at, updated_at) FROM stdin;
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
1	1	2019-11-27 22:26:12.425769	2019-11-27 22:26:12.425769
3	1	2019-11-27 22:26:12.572997	2019-11-27 22:26:12.572997
\.


--
-- Data for Name: goadmin_users; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.goadmin_users (id, username, password, name, avatar, remember_token, created_at, updated_at, status) FROM stdin;
2	operator	$2a$10$rVqkOzHjN2MdlEprRflb1eGP0oZXuSrbJLOmJagFsCd81YZm0bsh.	Operator		\N	2019-09-10 00:00:00	2019-09-10 00:00:00	enable
3	kl	$2a$10$WPrxfToSxGkNOex9c9Kb8eTgck2Iy2UQYR.M6JYxrxbruKRO4ccCm	kl		\N	2026-07-05 21:32:41.597011	2026-07-06 16:28:00	disable
1	admin	$2a$10$T79WWvOFYfvwiX23MgrHOenrdLuEuvvtc39Z0p4USC3LXrB39Up5C	admin		tlNcBVK9AvfYH7WEnwB1RKvocJu8FfRy4um3DJtwdHuJy0dwFsLOgAc0xUfh	2019-09-10 00:00:00	2019-09-10 00:00:00	enable
\.


--
-- Data for Name: user_like_books; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.user_like_books (id, user_id, name, created_at, updated_at) FROM stdin;
1	1	Robinson Crusoe	2020-03-15 09:00:57.409596	2020-03-15 09:00:57.409596
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.users (id, name, homepage, email, birthday, country, city, password, ip, certificate, money, resume, gender, fruit, drink, experience, created_at, updated_at, member_id) FROM stdin;
1	Jack	http://jack.me	jack@163.com	1993-10-21 00:00:00+08	china	guangzhou	123456	127.0.0.1	\N	10	<h1>Jacks Resume</h1>	0	apple	water	0	2020-03-09 15:24:00	2020-03-09 15:24:00	0
\.


--
-- Name: goadmin_department_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_department_myid_seq', 2, true);


--
-- Name: goadmin_dict_data_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_dict_data_myid_seq', 4, true);


--
-- Name: goadmin_dict_type_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_dict_type_myid_seq', 2, true);


--
-- Name: goadmin_menu_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_menu_myid_seq', 10, true);


--
-- Name: goadmin_operation_log_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_operation_log_myid_seq', 1, true);


--
-- Name: goadmin_permissions_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_permissions_myid_seq', 2, true);


--
-- Name: goadmin_roles_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_roles_myid_seq', 3, true);


--
-- Name: goadmin_session_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_session_myid_seq', 1, true);


--
-- Name: goadmin_site_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_site_myid_seq', 69, true);


--
-- Name: goadmin_users_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.goadmin_users_myid_seq', 3, true);


--
-- Name: goadmin_department goadmin_department_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_department
    ADD CONSTRAINT goadmin_department_pkey PRIMARY KEY (id);


--
-- Name: goadmin_department_roles goadmin_department_roles_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_department_roles
    ADD CONSTRAINT goadmin_department_roles_unique UNIQUE (department_id, role_id);


--
-- Name: goadmin_dict_data goadmin_dict_data_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_dict_data
    ADD CONSTRAINT goadmin_dict_data_pkey PRIMARY KEY (id);


--
-- Name: goadmin_dict_data goadmin_dict_data_type_value_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_dict_data
    ADD CONSTRAINT goadmin_dict_data_type_value_unique UNIQUE (dict_type, value);


--
-- Name: goadmin_dict_type goadmin_dict_type_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_dict_type
    ADD CONSTRAINT goadmin_dict_type_code_unique UNIQUE (code);


--
-- Name: goadmin_dict_type goadmin_dict_type_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_dict_type
    ADD CONSTRAINT goadmin_dict_type_pkey PRIMARY KEY (id);


--
-- Name: goadmin_menu goadmin_menu_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_menu
    ADD CONSTRAINT goadmin_menu_pkey PRIMARY KEY (id);


--
-- Name: goadmin_operation_log goadmin_operation_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_operation_log
    ADD CONSTRAINT goadmin_operation_log_pkey PRIMARY KEY (id);


--
-- Name: goadmin_permissions goadmin_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_permissions
    ADD CONSTRAINT goadmin_permissions_pkey PRIMARY KEY (id);


--
-- Name: goadmin_roles goadmin_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_roles
    ADD CONSTRAINT goadmin_roles_pkey PRIMARY KEY (id);


--
-- Name: goadmin_session goadmin_session_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_session
    ADD CONSTRAINT goadmin_session_pkey PRIMARY KEY (id);


--
-- Name: goadmin_site goadmin_site_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_site
    ADD CONSTRAINT goadmin_site_pkey PRIMARY KEY (id);


--
-- Name: goadmin_users goadmin_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goadmin_users
    ADD CONSTRAINT goadmin_users_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: goadmin_department_code_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX goadmin_department_code_index ON public.goadmin_department USING btree (code);


--
-- Name: goadmin_department_roles_role_id_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX goadmin_department_roles_role_id_index ON public.goadmin_department_roles USING btree (role_id);


--
-- Name: goadmin_dict_data_status_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX goadmin_dict_data_status_index ON public.goadmin_dict_data USING btree (status);


--
-- Name: goadmin_dict_data_type_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX goadmin_dict_data_type_index ON public.goadmin_dict_data USING btree (dict_type);


--
-- Name: goadmin_dict_type_status_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX goadmin_dict_type_status_index ON public.goadmin_dict_type USING btree (status);


--
-- PostgreSQL database dump complete
--

