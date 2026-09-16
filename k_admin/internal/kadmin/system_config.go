package kadmin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/bootstrap"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

const systemConfigPathEnv = "KADMIN_SYSTEM_CONFIG_PATH"

type systemConfigItem struct {
	Key         string   `json:"key"`
	Value       string   `json:"value"`
	Label       string   `json:"label,omitempty"`
	Type        string   `json:"type,omitempty"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
	Builtin     bool     `json:"builtin"`
	Future      bool     `json:"future,omitempty"`
}

type systemConfigPayload struct {
	Items []systemConfigItem `json:"items"`
}

type systemConfigOverview struct {
	FilePath string             `json:"filePath"`
	Items    []systemConfigItem `json:"items"`
}

type publicLoginConfig struct {
	CaptchaEnabled bool `json:"captchaEnabled"`
}

type systemConfigMeta struct {
	Key         string
	Label       string
	Type        string
	Default     string
	Options     []string
	Description string
	Future      bool
	Order       int
}

var systemConfigMetas = []systemConfigMeta{
	{
		Key:         "auth.default_username",
		Label:       "默认账号",
		Type:        "text",
		Default:     "admin",
		Description: "本地默认管理员账号",
		Order:       10,
	},
	{
		Key:         "auth.default_password",
		Label:       "默认密码",
		Type:        "password",
		Default:     "admin",
		Description: "本地默认管理员密码",
		Order:       20,
	},
	{
		Key:         "security.captcha_enabled",
		Label:       "验证码",
		Type:        "boolean",
		Default:     "false",
		Description: "登录验证码开关",
		Order:       30,
	},
	{
		Key:         "security.captcha_ttl_seconds",
		Label:       "验证码有效期（秒）",
		Type:        "text",
		Default:     "120",
		Description: "服务端验证码一次性挑战有效期，范围 30-600 秒",
		Order:       31,
	},
	{
		Key:         "security.login_lock_enabled",
		Label:       "登录失败自动锁定",
		Type:        "boolean",
		Default:     "true",
		Description: "使用 Redis 在多实例间共享登录失败计数和临时锁定状态",
		Order:       32,
	},
	{
		Key:         "security.login_failure_threshold",
		Label:       "账号失败阈值",
		Type:        "text",
		Default:     "5",
		Description: "失败窗口内同一账号允许的失败次数，范围 2-20",
		Order:       33,
	},
	{
		Key:         "security.login_ip_failure_threshold",
		Label:       "IP 失败阈值",
		Type:        "text",
		Default:     "20",
		Description: "失败窗口内同一 IP 允许的失败次数，范围 5-100",
		Order:       34,
	},
	{
		Key:         "security.login_failure_window_minutes",
		Label:       "失败统计窗口（分钟）",
		Type:        "text",
		Default:     "15",
		Description: "登录失败计数窗口，范围 1-1440 分钟",
		Order:       35,
	},
	{
		Key:         "security.login_lock_minutes",
		Label:       "自动锁定时长（分钟）",
		Type:        "text",
		Default:     "15",
		Description: "达到阈值后的临时锁定时长，范围 1-1440 分钟",
		Order:       36,
	},
	{
		Key:         "security.login_ip_whitelist",
		Label:       "登录 IP 白名单",
		Type:        "text",
		Default:     "127.0.0.1,::1",
		Description: "逗号分隔的 IP 或 CIDR；仅跳过 IP 维度锁定，账号维度仍生效",
		Order:       37,
	},
	{
		Key:         "security.idempotency_ttl_seconds",
		Label:       "幂等结果有效期（秒）",
		Type:        "text",
		Default:     "300",
		Description: "创建、导入和立即执行等接口的幂等结果保留时间，范围 30-86400 秒",
		Order:       38,
	},
	{
		Key:         "ui.theme_mode",
		Label:       "界面模式",
		Type:        "select",
		Default:     "auto",
		Options:     []string{"auto", "light", "dark"},
		Description: "跟随电脑主题 / 白天 / 黑夜模式",
		Order:       40,
	},
	{
		Key:         "navigation.external_link_target",
		Label:       "外链跳转方式",
		Type:        "select",
		Default:     "new_tab",
		Options:     []string{"new_tab", "current_page"},
		Description: "外链确认后在新标签页或当前页面打开",
		Order:       50,
	},
}

func registerSystemConfigRoutes(api *gin.RouterGroup, s *Store) {
	api.GET("/system/config/login", s.publicLoginConfig)
	api.GET("/system/config/login/", s.publicLoginConfig)

	group := api.Group("/system/config", s.requireAuth())
	group.GET("", s.requirePermission(systemConfigManagePermission), s.systemConfig)
	group.GET("/", s.requirePermission(systemConfigManagePermission), s.systemConfig)
	group.PUT("", s.requirePermission(bootstrap.SystemConfigUpdatePermission), s.updateSystemConfig)
	group.PUT("/", s.requirePermission(bootstrap.SystemConfigUpdatePermission), s.updateSystemConfig)
}

func (s *Store) publicLoginConfig(c *gin.Context) {
	values, err := s.readSystemConfig()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, publicLoginConfig{
		CaptchaEnabled: normalizeSystemConfigValue(
			systemConfigMeta{Type: "boolean"},
			values["security.captcha_enabled"],
		) == "true",
	})
}

func (s *Store) systemConfig(c *gin.Context) {
	values, err := s.readSystemConfig()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, systemConfigOverview{
		FilePath: systemConfigPath(),
		Items:    buildSystemConfigItems(values),
	})
}

func (s *Store) updateSystemConfig(c *gin.Context) {
	var req systemConfigPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid system config payload")
		return
	}

	values, err := normalizeSystemConfigPayload(req.Items)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.writeSystemConfig(values); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	success(c, systemConfigOverview{
		FilePath: systemConfigPath(),
		Items:    buildSystemConfigItems(values),
	})
}

func (s *Store) checkConfiguredDefaultAdmin(username, password string) (models.UserModel, bool) {
	values, err := s.readSystemConfig()
	if err != nil {
		return models.User(), false
	}

	configUsername := strings.TrimSpace(values["auth.default_username"])
	configPassword := values["auth.default_password"]
	if configUsername == "" || username != configUsername || password != configPassword {
		return models.User(), false
	}

	user := models.User().SetConn(s.conn).FindByUserName(configUsername)
	if user.IsEmpty() {
		return models.User(), false
	}
	return user.WithRoles().WithPermissions().WithMenus(), true
}

func (s *Store) readSystemConfig() (map[string]string, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	path := systemConfigPath()
	values := defaultSystemConfigValues()

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := ensureSystemConfigDir(path); err != nil {
				return nil, err
			}
			if err := writeSystemConfigFile(path, values); err != nil {
				return nil, err
			}
			return values, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return values, nil
	}

	var fileValues map[string]string
	if err := json.Unmarshal(content, &fileValues); err != nil {
		return nil, err
	}
	for key, value := range fileValues {
		key = strings.TrimSpace(key)
		if key != "" {
			values[key] = value
		}
	}
	return values, nil
}

func (s *Store) writeSystemConfig(values map[string]string) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	path := systemConfigPath()
	if err := ensureSystemConfigDir(path); err != nil {
		return err
	}
	return writeSystemConfigFile(path, values)
}

func systemConfigPath() string {
	return resolveSystemConfigPath(
		strings.TrimSpace(os.Getenv(systemConfigPathEnv)),
		systemConfigProjectRoot(),
	)
}

func resolveSystemConfigPath(configuredPath, projectRoot string) string {
	path := strings.TrimSpace(configuredPath)
	if path == "" {
		path = filepath.Join("data", "system_config.json")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(projectRoot, path))
}

func systemConfigProjectRoot() string {
	if workingDirectory, err := os.Getwd(); err == nil {
		if root := findKAdminProjectRoot(workingDirectory); root != "" {
			return root
		}
	}

	if executable, err := os.Executable(); err == nil {
		if root := findKAdminProjectRoot(filepath.Dir(executable)); root != "" {
			return root
		}
	}

	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		if root := findKAdminProjectRoot(filepath.Dir(sourceFile)); root != "" {
			return root
		}
	}

	if executable, err := os.Executable(); err == nil {
		return filepath.Dir(executable)
	}
	if workingDirectory, err := os.Getwd(); err == nil {
		return workingDirectory
	}
	return "."
}

func findKAdminProjectRoot(start string) string {
	directory, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if isKAdminProjectRoot(directory) {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return ""
		}
		directory = parent
	}
}

func isKAdminProjectRoot(directory string) bool {
	moduleFile, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil || !strings.Contains(string(moduleFile), "module github.com/GoAdminGroup/go-admin") {
		return false
	}
	info, err := os.Stat(filepath.Join(directory, "internal", "kadmin"))
	return err == nil && info.IsDir()
}

func ensureSystemConfigDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func writeSystemConfigFile(path string, values map[string]string) error {
	content, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0644)
}

func defaultSystemConfigValues() map[string]string {
	values := make(map[string]string, len(systemConfigMetas))
	for _, meta := range systemConfigMetas {
		values[meta.Key] = meta.Default
	}
	return values
}

func normalizeSystemConfigPayload(items []systemConfigItem) (map[string]string, error) {
	values := defaultSystemConfigValues()
	seen := make(map[string]bool, len(items))

	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			return nil, errors.New("config key is required")
		}
		if seen[key] {
			return nil, errors.New("duplicate config key: " + key)
		}
		seen[key] = true
		if !validSystemConfigKey(key) {
			return nil, errors.New("invalid config key: " + key)
		}

		value := item.Value
		if meta, ok := systemConfigMetaByKey(key); ok {
			value = normalizeSystemConfigValue(meta, value)
		}
		values[key] = value
	}
	if err := validateSecurityConfigValues(values); err != nil {
		return nil, err
	}
	return values, nil
}

func validateSecurityConfigValues(values map[string]string) error {
	ranges := []struct {
		key     string
		minimum int
		maximum int
	}{
		{key: "security.captcha_ttl_seconds", minimum: 30, maximum: 600},
		{key: "security.login_failure_threshold", minimum: 2, maximum: 20},
		{key: "security.login_ip_failure_threshold", minimum: 5, maximum: 100},
		{key: "security.login_failure_window_minutes", minimum: 1, maximum: 1440},
		{key: "security.login_lock_minutes", minimum: 1, maximum: 1440},
		{key: "security.idempotency_ttl_seconds", minimum: 30, maximum: 86400},
	}
	for _, item := range ranges {
		value, err := strconv.Atoi(strings.TrimSpace(values[item.key]))
		if err != nil || value < item.minimum || value > item.maximum {
			return fmt.Errorf("%s must be between %d and %d", item.key, item.minimum, item.maximum)
		}
	}
	for _, item := range splitConfigList(values["security.login_ip_whitelist"]) {
		if net.ParseIP(item) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(item); err != nil {
			return fmt.Errorf("security.login_ip_whitelist contains invalid IP or CIDR %q", item)
		}
	}
	return nil
}

func validSystemConfigKey(key string) bool {
	for _, r := range key {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '.', '_', '-', ':':
			continue
		default:
			return false
		}
	}
	return true
}

func normalizeSystemConfigValue(meta systemConfigMeta, value string) string {
	switch meta.Type {
	case "boolean":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "on":
			return "true"
		default:
			return "false"
		}
	case "select":
		value = strings.TrimSpace(value)
		for _, option := range meta.Options {
			if value == option {
				return value
			}
		}
		return meta.Default
	default:
		return strings.TrimSpace(value)
	}
}

func buildSystemConfigItems(values map[string]string) []systemConfigItem {
	items := make([]systemConfigItem, 0, len(values))
	added := make(map[string]bool, len(values))

	metas := append([]systemConfigMeta(nil), systemConfigMetas...)
	sort.SliceStable(metas, func(i, j int) bool {
		return metas[i].Order < metas[j].Order
	})
	for _, meta := range metas {
		value, ok := values[meta.Key]
		if !ok {
			value = meta.Default
		}
		items = append(items, systemConfigItem{
			Key:         meta.Key,
			Value:       normalizeSystemConfigValue(meta, value),
			Label:       meta.Label,
			Type:        meta.Type,
			Options:     meta.Options,
			Description: meta.Description,
			Builtin:     true,
			Future:      meta.Future,
		})
		added[meta.Key] = true
	}

	customKeys := make([]string, 0, len(values))
	for key := range values {
		if !added[key] {
			customKeys = append(customKeys, key)
		}
	}
	sort.Strings(customKeys)
	for _, key := range customKeys {
		items = append(items, systemConfigItem{
			Key:     key,
			Value:   values[key],
			Label:   key,
			Type:    "text",
			Builtin: false,
		})
	}

	return items
}

func systemConfigMetaByKey(key string) (systemConfigMeta, bool) {
	for _, meta := range systemConfigMetas {
		if meta.Key == key {
			return meta, true
		}
	}
	return systemConfigMeta{}, false
}
