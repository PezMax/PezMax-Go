package config

import (
	"fmt"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/utils"

	"github.com/stretchr/testify/assert"
)

func TestConfig_GetIndexUrl(t *testing.T) {
	Initialize(&Config{
		UrlPrefix: "admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().GetIndexURL(), "/admin")

	testSetCfg(&Config{
		UrlPrefix: "/admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().GetIndexURL(), "/admin")

	testSetCfg(&Config{
		UrlPrefix: "/admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().GetIndexURL(), "/admin")
}

func TestConfig_Index(t *testing.T) {
	testSetCfg(&Config{
		UrlPrefix: "admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().Index(), "/")
}

func TestConfig_Prefix(t *testing.T) {
	testSetCfg(&Config{
		UrlPrefix: "admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().Prefix(), "/admin")

	testSetCfg(&Config{
		UrlPrefix: "/admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().Prefix(), "/admin")
}

func TestConfig_Url(t *testing.T) {
	testSetCfg(&Config{
		UrlPrefix: "admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().Url("/info/user"), "/admin/info/user")

	testSetCfg(&Config{
		UrlPrefix: "/admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().Url("/info/user"), "/admin/info/user")
	assert.Equal(t, Get().Url("/info/user") != "/admin/info/user/", true)
}

func TestConfig_UrlRemovePrefix(t *testing.T) {

	testSetCfg(&Config{
		UrlPrefix: "/admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().URLRemovePrefix("/admin/info/user"), "/info/user")
}

func TestConfig_PrefixFixSlash(t *testing.T) {

	testSetCfg(&Config{
		UrlPrefix: "/admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().PrefixFixSlash(), "/admin")

	testSetCfg(&Config{
		UrlPrefix: "admin",
		IndexUrl:  "/",
	})

	assert.Equal(t, Get().PrefixFixSlash(), "/admin")
}

func TestSet(t *testing.T) {
	testSetCfg(&Config{Theme: "abc"})
	testSetCfg(&Config{Theme: "bcd"})
	assert.Equal(t, Get().Theme, "bcd")
}

func TestStore_URL(t *testing.T) {
	testSetCfg(&Config{
		Store: Store{
			Prefix: "/file",
			Path:   "./uploads",
		},
	})

	assert.Equal(t, Get().Store.URL("/xxxxxx.png"), "/file/xxxxxx.png")

	testSetCfg(&Config{
		Store: Store{
			Prefix: "http://xxxxx.com/xxxx/file",
			Path:   "./uploads",
		},
	})

	assert.Equal(t, Get().Store.URL("/xxxxxx.png"), "http://xxxxx.com/xxxx/file/xxxxxx.png")

	testSetCfg(&Config{
		Store: Store{
			Prefix: "/file",
			Path:   "./uploads",
		},
	})

	assert.Equal(t, Get().Store.URL("http://xxxxx.com/xxxx/file/xxxx.png"), "http://xxxxx.com/xxxx/file/xxxx.png")
}

func TestDatabase_ParamStr(t *testing.T) {
	cfg := Database{
		Driver: DriverMysql,
		Params: map[string]string{
			"parseTime": "true",
		},
	}
	assert.Equal(t, cfg.ParamStr(), "?charset=utf8mb4&parseTime=true")
}

func TestReadFromYaml(t *testing.T) {
	cfg := ReadFromYaml("./config.yaml")
	assert.Equal(t, cfg.Databases.GetDefault().Driver, "mssql")
	assert.Equal(t, cfg.Domain, "localhost")
	assert.Equal(t, cfg.UrlPrefix, "admin")
	assert.Equal(t, cfg.Store.Path, "./uploads")
	assert.Equal(t, cfg.IndexUrl, "/")
	assert.Equal(t, cfg.Debug, true)
	assert.Equal(t, cfg.OpenAdminApi, true)
	assert.Equal(t, cfg.ColorScheme, "skin-black")
}

func TestReadFromINI(t *testing.T) {
	cfg := ReadFromINI("./config.ini")
	assert.Equal(t, cfg.Databases.GetDefault().Driver, "postgresql")
	assert.Equal(t, cfg.Domain, "localhost")
	assert.Equal(t, cfg.UrlPrefix, "admin")
	assert.Equal(t, cfg.Store.Path, "./uploads")
	assert.Equal(t, cfg.IndexUrl, "/")
	assert.Equal(t, cfg.Debug, true)
	assert.Equal(t, cfg.OpenAdminApi, true)
	assert.Equal(t, cfg.ColorScheme, "skin-black")
}

func testSetCfg(cfg *Config) {
	count = 0
	Initialize(cfg)
}

func TestUpdate(t *testing.T) {
	m := map[string]string{
		"access_assets_log_off":             `true`,
		"access_log_off":                    `false`,
		"access_log_path":                   "",
		"allow_del_operation_log":           `false`,
		"animation":                         `{"type":"fadeInUp","duration":0,"delay":0}`,
		"animation_delay":                   `0.00`,
		"animation_duration":                `0`,
		"animation_type":                    `fadeInUp`,
		"app_id":                            `70rv3KwjwjXE`,
		"asset_root_path":                   `./public/`,
		"asset_url":                         "",
		"auth_user_table":                   `goadmin_users`,
		"bootstrap_file_path":               `./../datamodel/bootstrap.go`,
		"color_scheme":                      `skin-black`,
		"custom_403_html":                   "",
		"custom_404_html":                   "",
		"custom_500_html":                   "",
		"custom_foot_html":                  "",
		"custom_head_html":                  ` <link rel="icon" type="image/png" sizes="32x32" href="//quick.go-admin.cn/official/assets/imgs/icons.ico/favicon-32x32.png">`,
		"databases":                         `{"default":{"host":"127.0.0.1","port":"3306","user":"root","pwd":"root","name":"godmin","max_idle_con":50,"max_open_con":150,"driver":"mysql","file":"","dsn":""}}`,
		"debug":                             `true`,
		"domain":                            "",
		"env":                               `test`,
		"error_log_off":                     `false`,
		"error_log_path":                    "",
		"exclude_theme_components":          `null`,
		"extra":                             "",
		"file_upload_engine":                `{"name":"local","config":null}`,
		"footer_info":                       "",
		"go_mod_file_path":                  "",
		"hide_app_info_entrance":            `false`,
		"hide_config_center_entrance":       `false`,
		"hide_plugin_entrance":              `false`,
		"hide_tool_entrance":                `false`,
		"hide_visitor_user_center_entrance": `false`,
		"index_url":                         `/`,
		"info_log_off":                      `false`,
		"info_log_path":                     "",
		"language":                          `zh`,
		"logger_encoder_caller":             `full`,
		"logger_encoder_caller_key":         `caller`,
		"logger_encoder_duration":           `string`,
		"logger_encoder_encoding":           `console`,
		"logger_encoder_level":              `capitalColor`,
		"logger_encoder_level_key":          `level`,
		"logger_encoder_message_key":        `msg`,
		"logger_encoder_name_key":           `logger`,
		"logger_encoder_stacktrace_key":     `stacktrace`,
		"logger_encoder_time":               `iso8601`,
		"logger_encoder_time_key":           `ts`,
		"logger_level":                      `0`,
		"logger_rotate_compress":            `false`,
		"logger_rotate_max_age":             `30`,
		"logger_rotate_max_backups":         `5`,
		"logger_rotate_max_size":            `10`,
		"login_logo":                        "",
		"login_title":                       `GoAdmin`,
		"login_url":                         `/login`,
		"logo":                              `<b>Go</b>Admin`,
		"mini_logo":                         `<b>G</b>A`,
		"no_limit_login_ip":                 `false`,
		"open_admin_api":                    `false`,
		"operation_log_off":                 `false`,
		"plugin_file_path":                  `/go/src/github.com/GoAdminGroup/go-admin/examples/gin/plugins.go`,
		"session_life_time":                 `7200`,
		"site_off":                          `false`,
		"sql_log":                           `true`,
		"store":                             `{"path":"./uploads","prefix":"uploads"}`,
		"theme":                             `sword`,
		"title":                             `GoAdmin`,
		"url_prefix":                        `admin`,
	}
	c := &Config{}
	if err := c.Update(m); err != nil {
		t.Fatalf("update configuration: %v", err)
	}
	assert.Equal(t, "GoAdmin", c.Title)
	assert.Equal(t, "zh", c.Language)
	assert.Equal(t, "sword", c.Theme)
	assert.True(t, c.Debug)
	assert.True(t, c.AccessAssetsLogOff)
	assert.Equal(t, 7200, c.SessionLifeTime)
	assert.Equal(t, "fadeInUp", c.Animation.Type)
	assert.Equal(t, 10, c.Logger.Rotate.MaxSize)
	assert.Equal(t, 5, c.Logger.Rotate.MaxBackups)
	assert.Equal(t, 30, c.Logger.Rotate.MaxAge)
	assert.Nil(t, c.Extra)
	assert.Nil(t, c.Databases, "framework configuration updates must preserve database connection settings")
}

func TestToMap(t *testing.T) {
	c := &Config{
		UrlPrefix: "/admin",
		IndexUrl:  "/",
		MiniLogo:  "<asdfadsf>",
		Animation: PageAnimation{
			Type: "12313213",
		},
		SessionLifeTime:        40,
		ExcludeThemeComponents: []string{"asdfas", "sadfasf"},
	}
	m := c.ToMap()
	fmt.Println(m)
	fmt.Println(m["prefix"], m["animation_type"], m["mini_logo"])

	arr := []string{
		"language", "databases", "domain", "url_prefix", "theme", "store", "title", "logo", "mini_logo", "index_url",
		"site_off", "login_url", "debug", "env", "open_admin_api", "hide_visitor_user_center_entrance",
		"info_log_path", "error_log_path", "access_log_path", "sql_log", "access_log_off", "info_log_off", "error_log_off",
		"access_assets_log_off",
		"logger_rotate_max_size", "logger_rotate_max_backups", "logger_rotate_max_age", "logger_rotate_compress",
		"logger_encoder_time_key", "logger_encoder_level_key", "logger_encoder_name_key", "logger_encoder_caller_key",
		"logger_encoder_message_key", "logger_encoder_stacktrace_key", "logger_encoder_level", "logger_encoder_time",
		"logger_encoder_duration", "logger_encoder_caller", "logger_encoder_encoding", "logger_level",
		"color_scheme", "session_life_time", "asset_url", "file_upload_engine", "custom_head_html", "custom_foot_html",
		"custom_404_html", "custom_403_html", "custom_500_html", "bootstrap_file_path", "go_mod_file_path", "footer_info",
		"app_id", "login_title", "login_logo", "auth_user_table", "exclude_theme_components",
		"extra",
		"animation_type", "animation_duration", "animation_delay",
		"no_limit_login_ip", "allow_del_operation_log", "operation_log_off",
		"hide_config_center_entrance", "hide_app_info_entrance", "hide_tool_entrance", "hide_plugin_entrance",
		"asset_root_path", "prohibit_config_modification",
	}

	for key := range m {
		if !utils.InArray(arr, key) {
			panic(key)
		}
	}

	fmt.Println(len(arr), len(m))
}
