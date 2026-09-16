package kadmin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSystemConfigPathUsesProjectRoot(t *testing.T) {
	root := systemConfigProjectRoot()
	want := filepath.Join(root, "data", "system_config.json")
	if got := resolveSystemConfigPath("", root); got != want {
		t.Fatalf("default system config path = %q, want %q", got, want)
	}
	if !filepath.IsAbs(want) {
		t.Fatalf("system config path is not absolute: %q", want)
	}
}

func TestSystemConfigRelativeEnvironmentPathUsesProjectRoot(t *testing.T) {
	root := systemConfigProjectRoot()
	want := filepath.Join(root, "custom", "settings.json")
	if got := resolveSystemConfigPath(filepath.Join("custom", "settings.json"), root); got != want {
		t.Fatalf("relative system config path = %q, want %q", got, want)
	}
}

func TestSystemConfigAbsoluteEnvironmentPathIsPreserved(t *testing.T) {
	want := filepath.Join(t.TempDir(), "settings.json")
	if got := resolveSystemConfigPath(want, `D:\unrelated`); got != want {
		t.Fatalf("absolute system config path = %q, want %q", got, want)
	}
}

func TestFindKAdminProjectRootFromNestedDirectory(t *testing.T) {
	root := systemConfigProjectRoot()
	nested := filepath.Join(root, "internal", "kadmin")
	if got := findKAdminProjectRoot(nested); got != root {
		t.Fatalf("project root = %q, want %q", got, root)
	}
}

func TestSystemConfigPathDoesNotFollowWorkingDirectory(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	temporaryWorkingDirectory := t.TempDir()
	t.Cleanup(func() {
		if err := os.Chdir(originalWorkingDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	t.Setenv(systemConfigPathEnv, "")
	if err := os.Chdir(temporaryWorkingDirectory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	want := filepath.Join(systemConfigProjectRoot(), "data", "system_config.json")
	if got := systemConfigPath(); got != want {
		t.Fatalf("system config path = %q, want %q", got, want)
	}
}

func TestSystemConfigReadWriteUsesExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "system_config.json")
	t.Setenv(systemConfigPathEnv, path)
	store := &Store{}
	want := map[string]string{
		"auth.default_password":           "changed",
		"auth.default_username":           "admin",
		"security.captcha_enabled":        "true",
		"ui.theme_mode":                   "dark",
		"navigation.external_link_target": "current_page",
		"custom.relative_path_verified":   "yes",
	}
	if err := store.writeSystemConfig(want); err != nil {
		t.Fatalf("write system config: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat system config: %v", err)
	}
	got, err := store.readSystemConfig()
	if err != nil {
		t.Fatalf("read system config: %v", err)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("config %s = %q, want %q", key, got[key], value)
		}
	}
}

func TestExternalLinkTargetConfigDefaultsAndNormalizes(t *testing.T) {
	defaults := defaultSystemConfigValues()
	if got := defaults["navigation.external_link_target"]; got != "new_tab" {
		t.Fatalf("external link target default = %q, want new_tab", got)
	}

	meta, ok := systemConfigMetaByKey("navigation.external_link_target")
	if !ok {
		t.Fatal("external link target metadata was not found")
	}
	if got := normalizeSystemConfigValue(meta, "current_page"); got != "current_page" {
		t.Fatalf("normalized target = %q, want current_page", got)
	}
	if got := normalizeSystemConfigValue(meta, "popup"); got != "new_tab" {
		t.Fatalf("invalid target fallback = %q, want new_tab", got)
	}
}

func TestSecurityConfigValidationRejectsUnsafeRangesAndWhitelist(t *testing.T) {
	values := defaultSystemConfigValues()
	values["security.login_failure_threshold"] = "1"
	if err := validateSecurityConfigValues(values); err == nil {
		t.Fatal("login failure threshold below minimum should fail")
	}

	values = defaultSystemConfigValues()
	values["security.login_ip_whitelist"] = "127.0.0.1,10.0.0.0/8,not-an-ip"
	if err := validateSecurityConfigValues(values); err == nil {
		t.Fatal("invalid login IP whitelist should fail")
	}

	values["security.login_ip_whitelist"] = "127.0.0.1,10.0.0.0/8"
	if err := validateSecurityConfigValues(values); err != nil {
		t.Fatalf("valid security config failed: %v", err)
	}
}

func TestDefaultSecurityPolicyValues(t *testing.T) {
	values := defaultSystemConfigValues()
	for key, want := range map[string]string{
		"security.captcha_ttl_seconds":          "120",
		"security.idempotency_ttl_seconds":      "300",
		"security.login_failure_threshold":      "5",
		"security.login_failure_window_minutes": "15",
		"security.login_ip_failure_threshold":   "20",
		"security.login_lock_enabled":           "true",
		"security.login_lock_minutes":           "15",
	} {
		if got := values[key]; got != want {
			t.Fatalf("default %s = %q, want %q", key, got, want)
		}
	}
}
