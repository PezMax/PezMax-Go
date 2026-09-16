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

--
-- Data for Name: goadmin_menu; Type: TABLE DATA; Schema: public; Owner: postgres
--

INSERT INTO public.goadmin_menu (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, created_at, updated_at) VALUES (1, 0, 1, 2, 'Admin', NULL, 'fa-tasks', '', NULL, '', '2019-09-10 00:00:00', '2019-09-10 00:00:00');
INSERT INTO public.goadmin_menu (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, created_at, updated_at) VALUES (2, 1, 1, 2, 'Users', NULL, 'fa-users', '/info/manager', NULL, '', '2019-09-10 00:00:00', '2019-09-10 00:00:00');
INSERT INTO public.goadmin_menu (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, created_at, updated_at) VALUES (3, 1, 1, 3, 'Roles', NULL, 'fa-user', '/info/roles', NULL, '', '2019-09-10 00:00:00', '2019-09-10 00:00:00');
INSERT INTO public.goadmin_menu (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, created_at, updated_at) VALUES (4, 1, 1, 4, 'Permission', NULL, 'fa-ban', '/info/permission', NULL, '', '2019-09-10 00:00:00', '2019-09-10 00:00:00');
INSERT INTO public.goadmin_menu (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, created_at, updated_at) VALUES (5, 1, 1, 5, 'Menu', NULL, 'fa-bars', '/menu', NULL, '', '2019-09-10 00:00:00', '2019-09-10 00:00:00');
INSERT INTO public.goadmin_menu (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, created_at, updated_at) VALUES (6, 1, 1, 6, 'Operation log', NULL, 'fa-history', '/info/op', NULL, '', '2019-09-10 00:00:00', '2019-09-10 00:00:00');
INSERT INTO public.goadmin_menu (id, parent_id, type, "order", title, header, icon, uri, uuid, plugin_name, created_at, updated_at) VALUES (7, 0, 1, 1, 'Dashboard', NULL, 'fa-bar-chart', '/', NULL, '', '2019-09-10 00:00:00', '2019-09-10 00:00:00');


--
-- Data for Name: goadmin_role_menu; Type: TABLE DATA; Schema: public; Owner: postgres
--

INSERT INTO public.goadmin_role_menu (role_id, menu_id, created_at, updated_at) VALUES (1, 1, '2019-09-10 00:00:00', '2019-09-10 00:00:00');
INSERT INTO public.goadmin_role_menu (role_id, menu_id, created_at, updated_at) VALUES (1, 7, '2019-09-10 00:00:00', '2019-09-10 00:00:00');
INSERT INTO public.goadmin_role_menu (role_id, menu_id, created_at, updated_at) VALUES (2, 7, '2026-07-07 12:01:33.806603', '2026-07-07 12:01:33.806603');
INSERT INTO public.goadmin_role_menu (role_id, menu_id, created_at, updated_at) VALUES (2, 2, '2026-07-07 12:01:33.809229', '2026-07-07 12:01:33.809229');


--
-- PostgreSQL database dump complete
--

