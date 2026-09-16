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
    icon character varying(50) NOT NULL,
    uri character varying(50) NOT NULL,
    uuid character varying(100),
    plugin_name character varying(150) NOT NULL,
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
-- Name: kadmin_files; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_files (
    id bigint GENERATED BY DEFAULT AS IDENTITY NOT NULL,
    object_key character varying(1024) NOT NULL,
    original_name character varying(255) NOT NULL,
    extension character varying(32) DEFAULT ''::character varying NOT NULL,
    content_type character varying(255) NOT NULL,
    size bigint NOT NULL,
    sha256 character(64) NOT NULL,
    storage character varying(16) NOT NULL,
    bucket character varying(255) DEFAULT ''::character varying NOT NULL,
    purpose character varying(32) NOT NULL,
    visibility character varying(16) DEFAULT 'private'::character varying NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    created_by bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    expires_at timestamp with time zone,
    CONSTRAINT kadmin_files_size_check CHECK ((size > 0)),
    CONSTRAINT kadmin_files_sha256_check CHECK ((sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT kadmin_files_storage_check CHECK (((storage)::text = ANY ((ARRAY['pending'::character varying, 'local'::character varying, 'minio'::character varying])::text[]))),
    CONSTRAINT kadmin_files_purpose_check CHECK (((purpose)::text = ANY ((ARRAY['avatar'::character varying, 'editor-image'::character varying, 'attachment'::character varying, 'import-temp'::character varying])::text[]))),
    CONSTRAINT kadmin_files_visibility_check CHECK (((visibility)::text = ANY ((ARRAY['public'::character varying, 'private'::character varying])::text[]))),
    CONSTRAINT kadmin_files_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'ready'::character varying, 'failed'::character varying, 'deleting'::character varying, 'deleted'::character varying])::text[]))),
    CONSTRAINT kadmin_files_deleted_at_check CHECK ((((status)::text = 'deleted'::text AND deleted_at IS NOT NULL) OR ((status)::text <> 'deleted'::text))),
    CONSTRAINT kadmin_files_expires_at_check CHECK (((expires_at IS NULL) OR (expires_at >= created_at)))
);


ALTER TABLE public.kadmin_files OWNER TO postgres;

--
-- Name: kadmin_jobs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_jobs (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    name character varying(100) NOT NULL UNIQUE,
    handler character varying(64) NOT NULL,
    cron_expression character varying(255) NOT NULL,
    parameters jsonb DEFAULT '{}'::jsonb NOT NULL,
    description character varying(500) DEFAULT ''::character varying NOT NULL,
    status character varying(16) DEFAULT 'paused'::character varying NOT NULL,
    built_in boolean DEFAULT false NOT NULL,
    last_run_at timestamp with time zone,
    next_run_at timestamp with time zone,
    created_by bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT kadmin_jobs_status_check CHECK (((status)::text = ANY ((ARRAY['enabled'::character varying, 'paused'::character varying])::text[]))),
    CONSTRAINT kadmin_jobs_parameters_check CHECK ((jsonb_typeof(parameters) = 'object'::text))
);

ALTER TABLE public.kadmin_jobs OWNER TO postgres;

--
-- Name: kadmin_job_logs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_job_logs (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    job_id bigint REFERENCES public.kadmin_jobs(id) ON DELETE SET NULL,
    job_name character varying(100) NOT NULL,
    handler character varying(64) NOT NULL,
    trigger character varying(16) NOT NULL,
    status character varying(16) NOT NULL,
    output text DEFAULT ''::text NOT NULL,
    error_message text DEFAULT ''::text NOT NULL,
    duration_ms bigint DEFAULT 0 NOT NULL,
    triggered_by bigint,
    started_at timestamp with time zone NOT NULL,
    finished_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT kadmin_job_logs_trigger_check CHECK (((trigger)::text = ANY ((ARRAY['scheduled'::character varying, 'manual'::character varying])::text[]))),
    CONSTRAINT kadmin_job_logs_status_check CHECK (((status)::text = ANY ((ARRAY['running'::character varying, 'success'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT kadmin_job_logs_duration_check CHECK ((duration_ms >= 0)),
    CONSTRAINT kadmin_job_logs_finished_check CHECK (((finished_at IS NULL) OR (finished_at >= started_at)))
);

ALTER TABLE public.kadmin_job_logs OWNER TO postgres;

--
-- Name: kadmin_monitor_settings; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_monitor_settings (
    id smallint NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    updated_by bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT kadmin_monitor_settings_pkey PRIMARY KEY (id),
    CONSTRAINT kadmin_monitor_settings_singleton_check CHECK ((id = 1))
);

ALTER TABLE public.kadmin_monitor_settings OWNER TO postgres;

INSERT INTO public.kadmin_monitor_settings (id, enabled, updated_by)
VALUES (1, false, 0);

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
    username character varying(190) NOT NULL,
    password character varying(80) NOT NULL,
    name character varying(255) NOT NULL,
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

COPY public.goadmin_menu (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, component, created_at, updated_at) FROM stdin;
1	0	0	0	Dashboard		lucide:layout-dashboard	/dashboard				2026-07-08 17:03:06.37182	2026-07-31 22:22:52
2	1	1	0	分析页		lucide:area-chart	/dashboard/analytics				2026-07-08 17:03:06.37182	2026-07-31 22:22:52
3	1	1	1	工作台		carbon:workspace	/dashboard/workspace				2026-07-08 17:03:06.37182	2026-07-31 22:22:52
4	0	0	2	KAdmin 管理		lucide:settings-2	/kadmin				2026-07-08 17:03:06.37182	2026-07-31 22:22:52
5	14	1	1	用户管理		lucide:users	/kadmin/users				2026-07-08 17:03:06.37182	2026-07-31 22:22:52
6	14	1	0	权限管理	\N	lucide:shield-check	/kadmin/rbac	\N			2026-07-08 17:03:06.37182	2026-07-31 22:22:52
7	4	1	1	菜单管理	\N	lucide:menu	/kadmin/menus	\N			2026-07-08 17:03:06.37182	2026-07-31 22:22:52
8	18	1	0	字典管理	\N	lucide:book-open	/kadmin/dictionary	\N			2026-07-08 17:03:06.37182	2026-07-31 22:22:52
9	18	1	2	参数配置	\N	lucide:sliders-horizontal	/kadmin/settings	\N			2026-07-08 17:03:06.37182	2026-07-31 22:22:52
10	4	1	0	资源工作台		lucide:folder-kanban	/kadmin/resources				2026-07-08 17:03:06.37182	2026-07-31 22:22:52
12	4	1	2	日志管理	\N	lucide:scroll-text	/kadmin/logs	\N			2026-07-22 15:15:48.436343	2026-07-31 22:22:52
14	0	0	3	用户管理		lucide:user-cog					2026-07-25 12:25:36	2026-07-31 22:22:52
17	18	1	1	定时任务	\N	lucide:clock-3	/kadmin/jobs	\N			2026-07-26 14:31:43	2026-07-31 22:22:52
18	0	0	1	系统设置		lucide:settings					2026-07-26 15:33:16	2026-07-31 22:22:52
19	18	1	3	系统监控	\N	lucide:monitor-cog	/kadmin/monitor	\N			2026-07-26 16:54:52	2026-07-31 22:22:52
21	0	2	4	项目支持		carbon:api	https://github.com/wlklssd/k_admin				2026-07-27 17:15:30	2026-07-31 22:22:52
22	4	1	3	登录审计	\N	lucide:shield-check	/kadmin/login-audits	\N			2026-07-28 14:08:47	2026-07-31 22:22:52
23	0	2	5	Swagger		carbon:api	http://127.0.0.1:9033/swagger/index.html				2026-07-28 14:51:46	2026-07-31 22:22:52
24	18	1	4	接口负载排行	\N	lucide:bar-chart-3	/kadmin/load-ranking	\N			2026-07-31 21:26:39	2026-07-31 22:22:52
25	4	1	12	代码生成	\N	lucide:wand-2	/kadmin/codegen	\N			2026-08-14 17:21:05	2026-08-14 17:21:05
26	0	0	20	业务模块	\N	lucide:package	/business	\N			2026-08-14 17:21:05	2026-08-14 17:21:05
\.


--
-- Data for Name: goadmin_operation_log; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_operation_log (id, user_id, path, method, ip, input, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: goadmin_site; Type: TABLE DATA; Schema: public; Owner: postgres
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
3	查看请求日志	system:log:list	GET	/api/logs*	2026-07-22 15:15:48.436343	2026-07-22 15:15:48.436343
4	删除请求日志	system:log:delete	DELETE	/api/logs*	2026-07-22 15:15:48.436343	2026-07-22 15:15:48.436343
8	上传文件	system:file:upload	POST	/api/files	2026-07-25 18:28:17.964672	2026-07-25 18:28:17.964672
9	读取文件	system:file:read	GET	/api/files*	2026-07-25 18:28:17.964672	2026-07-25 18:28:17.964672
10	删除文件	system:file:delete	DELETE	/api/files/*	2026-07-25 18:28:17.964672	2026-07-25 18:28:17.964672
11	查看定时任务	system:job:list	GET	/api/jobs*	2026-07-26 14:30:31	2026-07-26 14:30:31
12	创建定时任务	system:job:create	POST	/api/jobs	2026-07-26 14:30:31	2026-07-26 14:30:31
13	修改定时任务	system:job:update	PUT,PATCH	/api/jobs/*	2026-07-26 14:30:31	2026-07-26 14:30:31
14	删除定时任务	system:job:delete	DELETE	/api/jobs/*	2026-07-26 14:30:31	2026-07-26 14:30:31
15	立即执行任务	system:job:run	POST	/api/jobs/*/run	2026-07-26 14:30:31	2026-07-26 14:30:31
16	查看任务日志	system:job-log:list	GET	/api/job-logs*	2026-07-26 14:30:31	2026-07-26 14:30:31
17	查看系统监控	system:monitor:view	GET	/api/system-monitor	2026-07-26 16:54:52	2026-07-26 16:54:52
18	启停系统监控	system:monitor:update	PATCH	/api/system-monitor/status	2026-07-26 16:54:52	2026-07-26 16:54:52
19	查看登录审计	system:login-log:list	GET	/api/login-audits*	2026-07-28 14:08:47	2026-07-28 14:08:47
20	清理登录审计	system:login-log:delete	DELETE,POST	/api/login-audits*	2026-07-28 14:08:47	2026-07-28 14:08:47
21	设置登录审计保留周期	system:login-log:retention	PATCH	/api/login-audits/retention	2026-07-28 14:08:47	2026-07-28 14:08:47
22	查看接口负载排行	system:load-rank:view	GET	/api/load-ranking*	2026-07-31 21:26:39	2026-07-31 21:26:39
23	启停接口采样	system:load-rank:update	PATCH	/api/load-ranking/status	2026-07-31 21:26:39	2026-07-31 21:26:39
24	用户管理	system:user:manage	GET,POST,PUT,DELETE	/api/users*	2026-07-31 22:16:55	2026-07-31 22:16:55
25	权限管理	system:rbac:manage	GET,POST,PUT,DELETE	/api/rbac*	2026-07-31 22:16:55	2026-07-31 22:16:55
26	菜单管理	system:menu:manage	GET,POST,PUT,DELETE	/api/admin-menus*	2026-07-31 22:16:55	2026-07-31 22:16:55
27	字典管理	system:dict:manage	GET,POST,PUT,DELETE	/api/dictionaries*	2026-07-31 22:16:55	2026-07-31 22:16:55
28	参数配置	system:config:manage	GET,PUT	/api/system/config*	2026-07-31 22:16:55	2026-07-31 22:16:55
29	查看代码生成	system:codegen:list	GET	/api/codegen*	2026-08-14 17:21:05	2026-08-14 17:21:05
30	导入与配置代码生成	system:codegen:import	POST,PUT,DELETE	/api/codegen*	2026-08-14 17:21:05	2026-08-14 17:21:05
31	预览与生成代码	system:codegen:generate	POST	/api/codegen*	2026-08-14 17:21:05	2026-08-14 17:21:05
36	查看站内通知	system:notification:list	GET,PATCH,DELETE	/api/notifications*	2026-08-14 18:23:04	2026-08-14 18:23:04
37	发送站内通知	system:notification:create	POST	/api/notifications*	2026-08-14 18:23:04	2026-08-14 18:23:04
\.


--
-- Data for Name: goadmin_role_menu; Type: TABLE DATA; Schema: public; Owner: postgres
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
1	12	2026-07-22 15:15:48.436343	2026-07-22 15:15:48.436343
2	1	2026-07-31 21:42:02.064686	2026-07-31 21:42:02.064686
2	2	2026-07-31 21:42:02.066442	2026-07-31 21:42:02.066442
2	3	2026-07-31 21:42:02.067796	2026-07-31 21:42:02.067796
2	8	2026-07-31 21:42:02.069223	2026-07-31 21:42:02.069223
2	17	2026-07-31 21:42:02.070832	2026-07-31 21:42:02.070832
2	9	2026-07-31 21:42:02.072427	2026-07-31 21:42:02.072427
2	19	2026-07-31 21:42:02.073807	2026-07-31 21:42:02.073807
2	18	2026-07-31 21:42:02.075527	2026-07-31 21:42:02.075527
\.


--
-- Data for Name: goadmin_role_permissions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_role_permissions (role_id, permission_id, created_at, updated_at) FROM stdin;
1	1	2019-09-10 00:00:00	2019-09-10 00:00:00
1	2	2019-09-10 00:00:00	2019-09-10 00:00:00
2	2	2019-09-10 00:00:00	2019-09-10 00:00:00
1	3	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
1	4	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
1	8	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
1	9	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
1	10	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
1	24	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
1	25	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
1	26	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
1	27	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
1	28	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	11	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	12	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	13	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	14	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	15	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	16	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	17	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	18	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	27	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
2	28	2026-08-01 13:22:54.958637	2026-08-01 13:22:54.958637
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
2	f5a99916-36c8-4fd6-8873-6f2be8845cd0	{"user_id":1}	2019-11-27 22:26:11.917665	2019-11-27 22:26:11.917665
3	03263ffc-0043-4b89-a02f-3aa616bbf857	{"user_id":3}	2019-11-27 22:26:12.819931	2019-11-27 22:26:12.819931
\.


--
-- Data for Name: goadmin_user_permissions; Type: TABLE DATA; Schema: public; Owner: postgres
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
-- Data for Name: goadmin_users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_users (id, username, password, name, avatar, status, remember_token, created_at, updated_at) FROM stdin;
1	admin	$2a$10$OxWYJJGTP2gi00l2x06QuOWqw5VR47MQCJ0vNKnbMYfrutij10Hwe	admin		enable	tlNcBVK9AvfYH7WEnwB1RKvocJu8FfRy4um3DJtwdHuJy0dwFsLOgAc0xUfh	2019-09-10 00:00:00	2019-09-10 00:00:00
2	operator	$2a$10$rVqkOzHjN2MdlEprRflb1eGP0oZXuSrbJLOmJagFsCd81YZm0bsh.	Operator		enable	\N	2019-09-10 00:00:00	2019-09-10 00:00:00
\.


--
-- Name: goadmin_menu_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_menu_myid_seq', 26, true);


--
-- Name: goadmin_operation_log_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_operation_log_myid_seq', 1, true);


--
-- Name: goadmin_permissions_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_permissions_myid_seq', 37, true);


--
-- Name: goadmin_roles_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_roles_myid_seq', 2, true);


--
-- Name: goadmin_session_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_session_myid_seq', 1, true);

--
-- Name: goadmin_site_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_site_myid_seq', 69, true);


--
-- Name: goadmin_dict_type_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_dict_type_myid_seq', 2, true);


--
-- Name: goadmin_dict_data_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_dict_data_myid_seq', 4, true);


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


--
-- Name: kadmin_files kadmin_files_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.kadmin_files
    ADD CONSTRAINT kadmin_files_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.kadmin_files
    ADD CONSTRAINT kadmin_files_object_key_unique UNIQUE (object_key);


CREATE INDEX kadmin_files_created_by_time_index
    ON public.kadmin_files USING btree (created_by, created_at DESC, id DESC);


CREATE INDEX kadmin_files_purpose_time_index
    ON public.kadmin_files USING btree (purpose, created_at DESC, id DESC);


CREATE INDEX kadmin_files_cleanup_index
    ON public.kadmin_files USING btree (status, updated_at, id)
    WHERE ((status)::text = ANY ((ARRAY['pending'::character varying, 'failed'::character varying, 'deleting'::character varying])::text[]));


CREATE INDEX kadmin_files_expiration_index
    ON public.kadmin_files USING btree (expires_at, id)
    WHERE ((expires_at IS NOT NULL) AND ((status)::text = 'ready'::text));


CREATE UNIQUE INDEX kadmin_jobs_builtin_handler_unique
    ON public.kadmin_jobs USING btree (handler)
    WHERE (built_in = true);


CREATE INDEX kadmin_jobs_status_next_run_index
    ON public.kadmin_jobs USING btree (status, next_run_at, id);


CREATE INDEX kadmin_job_logs_job_time_index
    ON public.kadmin_job_logs USING btree (job_id, started_at DESC, id DESC);


CREATE INDEX kadmin_job_logs_status_time_index
    ON public.kadmin_job_logs USING btree (status, started_at DESC, id DESC);


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
-- Name: goadmin_department_myid_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.goadmin_department_myid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 99999999
    CACHE 1;


ALTER SEQUENCE public.goadmin_department_myid_seq OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

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
-- Data for Name: goadmin_department; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_department (id, name, code, description, sort, status, created_at, updated_at) FROM stdin;
1	管理	onp		0	1	2026-07-04 17:35:42.862376	2026-07-04 17:35:42.862376
2	运营	onb		1	1	2026-07-04 17:42:33.067153	2026-07-04 17:42:33.067153
\.


--
-- Data for Name: goadmin_department_roles; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.goadmin_department_roles (department_id, role_id, created_at, updated_at) FROM stdin;
1	1	2026-07-04 17:42:39.171653	2026-07-04 17:42:39.171653
1	2	2026-07-04 17:42:39.173343	2026-07-04 17:42:39.173343
2	3	2026-07-04 17:42:52.058756	2026-07-04 17:42:52.058756
\.


--
-- Name: goadmin_department_myid_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.goadmin_department_myid_seq', 2, true);


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
-- Name: goadmin_department_code_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX goadmin_department_code_index ON public.goadmin_department USING btree (code);


--
-- Name: goadmin_department_roles_role_id_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX goadmin_department_roles_role_id_index ON public.goadmin_department_roles USING btree (role_id);


--
-- Name: kadmin_login_audits; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_login_audits (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    account character varying(100) DEFAULT ''::character varying NOT NULL,
    user_id bigint,
    ip character varying(45) DEFAULT ''::character varying NOT NULL,
    user_agent character varying(1024) DEFAULT ''::character varying NOT NULL,
    browser character varying(100) DEFAULT ''::character varying NOT NULL,
    os character varying(100) DEFAULT ''::character varying NOT NULL,
    status character varying(16) NOT NULL,
    result character varying(32) NOT NULL,
    failure_reason character varying(255) DEFAULT ''::character varying NOT NULL,
    duration_ms bigint DEFAULT 0 NOT NULL,
    occurred_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT kadmin_login_audits_status_check CHECK (((status)::text = ANY ((ARRAY['success'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT kadmin_login_audits_result_check CHECK (((result)::text = ANY ((ARRAY['success'::character varying, 'account_not_found'::character varying, 'invalid_password'::character varying, 'account_disabled'::character varying, 'account_locked'::character varying, 'account_unlocked'::character varying, 'captcha_invalid'::character varying, 'system_error'::character varying])::text[]))),
    CONSTRAINT kadmin_login_audits_duration_check CHECK ((duration_ms >= 0))
);

ALTER TABLE public.kadmin_login_audits OWNER TO postgres;


--
-- Name: kadmin_login_audits_account_time_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX kadmin_login_audits_account_time_index ON public.kadmin_login_audits USING btree (account, occurred_at DESC, id DESC);


--
-- Name: kadmin_login_audits_ip_time_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX kadmin_login_audits_ip_time_index ON public.kadmin_login_audits USING btree (ip, occurred_at DESC, id DESC);


--
-- Name: kadmin_login_audits_status_time_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX kadmin_login_audits_status_time_index ON public.kadmin_login_audits USING btree (status, occurred_at DESC, id DESC);


--
-- Name: kadmin_login_audits_result_time_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX kadmin_login_audits_result_time_index ON public.kadmin_login_audits USING btree (result, occurred_at DESC, id DESC);


--
-- Name: kadmin_login_audit_settings; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_login_audit_settings (
    id smallint NOT NULL,
    retention_days integer DEFAULT 90 NOT NULL,
    updated_by bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT kadmin_login_audit_settings_pkey PRIMARY KEY (id),
    CONSTRAINT kadmin_login_audit_settings_singleton_check CHECK ((id = 1)),
    CONSTRAINT kadmin_login_audit_settings_retention_check CHECK (((retention_days >= 1) AND (retention_days <= 3650)))
);

ALTER TABLE public.kadmin_login_audit_settings OWNER TO postgres;

INSERT INTO public.kadmin_login_audit_settings (id, retention_days)
VALUES (1, 90);


--
-- Name: kadmin_loadrank_settings; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_loadrank_settings (
    id smallint NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    updated_by bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT kadmin_loadrank_settings_pkey PRIMARY KEY (id),
    CONSTRAINT kadmin_loadrank_settings_singleton_check CHECK ((id = 1))
);

ALTER TABLE public.kadmin_loadrank_settings OWNER TO postgres;

INSERT INTO public.kadmin_loadrank_settings (id, enabled, updated_by)
VALUES (1, false, 0);


--
-- Name: kadmin_http_metric_buckets; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_http_metric_buckets (
    bucket_start timestamp with time zone NOT NULL,
    route character varying(512) NOT NULL,
    method character varying(16) NOT NULL,
    status_code smallint NOT NULL,
    request_count bigint DEFAULT 0 NOT NULL,
    error_count bigint DEFAULT 0 NOT NULL,
    total_duration_ms bigint DEFAULT 0 NOT NULL,
    max_duration_ms bigint DEFAULT 0 NOT NULL,
    CONSTRAINT kadmin_http_metric_buckets_pkey PRIMARY KEY (bucket_start, route, method, status_code),
    CONSTRAINT kadmin_http_metric_buckets_status_code_check CHECK (((status_code >= 100) AND (status_code <= 599))),
    CONSTRAINT kadmin_http_metric_buckets_counts_check CHECK (((request_count >= 0) AND (error_count >= 0) AND (total_duration_ms >= 0) AND (max_duration_ms >= 0)))
);

ALTER TABLE public.kadmin_http_metric_buckets OWNER TO postgres;


--
-- Name: kadmin_codegen_tables; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_codegen_tables (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    table_name character varying(128) NOT NULL UNIQUE,
    module_name character varying(64) NOT NULL,
    class_name character varying(64) NOT NULL,
    business_name character varying(128) NOT NULL,
    route_prefix character varying(64) NOT NULL,
    columns jsonb DEFAULT '[]'::jsonb NOT NULL,
    generated boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT kadmin_codegen_tables_columns_check CHECK ((jsonb_typeof(columns) = 'array'::text))
);

ALTER TABLE public.kadmin_codegen_tables OWNER TO postgres;


--
-- Name: kadmin_notifications; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kadmin_notifications (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    title character varying(200) NOT NULL,
    content character varying(1000) DEFAULT ''::character varying NOT NULL,
    link character varying(500) DEFAULT ''::character varying NOT NULL,
    type character varying(16) DEFAULT 'info'::character varying NOT NULL,
    is_read boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT kadmin_notifications_type_check CHECK (((type)::text = ANY ((ARRAY['info'::character varying, 'success'::character varying, 'warning'::character varying])::text[])))
);

ALTER TABLE public.kadmin_notifications OWNER TO postgres;


--
-- Name: kadmin_notifications_read_time_index; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX kadmin_notifications_read_time_index ON public.kadmin_notifications USING btree (is_read, created_at DESC, id DESC);

-- Name: SCHEMA public; Type: ACL; Schema: -; Owner: postgres
--

REVOKE ALL ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM postgres;
GRANT ALL ON SCHEMA public TO postgres;
GRANT ALL ON SCHEMA public TO PUBLIC;


--
-- PostgreSQL database dump complete
--
