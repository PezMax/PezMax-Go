package kadmin

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterMenuManagementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerMenuManagementRoutes(engine.Group("/api"), &Store{})

	wanted := map[string]bool{
		"PUT /api/admin-menus":     false,
		"PUT /api/admin-menus/:id": false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := wanted[key]; exists {
			wanted[key] = true
		}
	}
	for route, registered := range wanted {
		if !registered {
			t.Fatalf("route %s was not registered", route)
		}
	}
}

func TestValidateMenuPositions(t *testing.T) {
	menus := []managedMenu{
		{ID: 1, ParentID: 0, Type: menuTypeDirectory, Order: 0},
		{ID: 2, ParentID: 1, Type: menuTypeItem, Order: 0},
		{ID: 3, ParentID: 0, Type: menuTypeDirectory, Order: 1},
	}

	tests := []struct {
		name      string
		positions []menuPosition
		wantError bool
	}{
		{
			name: "valid reparent and reorder",
			positions: []menuPosition{
				{ID: 3, ParentID: 0, Order: 0},
				{ID: 1, ParentID: 0, Order: 1},
				{ID: 2, ParentID: 3, Order: 0},
			},
		},
		{
			name: "missing menu",
			positions: []menuPosition{
				{ID: 1, ParentID: 0, Order: 0},
				{ID: 2, ParentID: 1, Order: 0},
			},
			wantError: true,
		},
		{
			name: "duplicate sibling order",
			positions: []menuPosition{
				{ID: 1, ParentID: 0, Order: 0},
				{ID: 2, ParentID: 1, Order: 0},
				{ID: 3, ParentID: 0, Order: 0},
			},
			wantError: true,
		},
		{
			name: "cyclic hierarchy",
			positions: []menuPosition{
				{ID: 1, ParentID: 2, Order: 0},
				{ID: 2, ParentID: 1, Order: 0},
				{ID: 3, ParentID: 0, Order: 0},
			},
			wantError: true,
		},
		{
			name: "duplicate menu",
			positions: []menuPosition{
				{ID: 1, ParentID: 0, Order: 0},
				{ID: 1, ParentID: 0, Order: 1},
				{ID: 3, ParentID: 0, Order: 2},
			},
			wantError: true,
		},
		{
			name: "unknown parent",
			positions: []menuPosition{
				{ID: 1, ParentID: 0, Order: 0},
				{ID: 2, ParentID: 99, Order: 0},
				{ID: 3, ParentID: 0, Order: 1},
			},
			wantError: true,
		},
		{
			name: "menu item as parent",
			positions: []menuPosition{
				{ID: 1, ParentID: 0, Order: 0},
				{ID: 2, ParentID: 0, Order: 1},
				{ID: 3, ParentID: 2, Order: 0},
			},
			wantError: true,
		},
		{
			name: "negative order",
			positions: []menuPosition{
				{ID: 1, ParentID: 0, Order: 0},
				{ID: 2, ParentID: 1, Order: -1},
				{ID: 3, ParentID: 0, Order: 1},
			},
			wantError: true,
		},
		{
			name: "non-continuous sibling orders",
			positions: []menuPosition{
				{ID: 1, ParentID: 0, Order: 0},
				{ID: 2, ParentID: 1, Order: 0},
				{ID: 3, ParentID: 0, Order: 2},
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateMenuPositions(menus, test.positions)
			if test.wantError && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantError && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidateMenuType(t *testing.T) {
	for _, menuType := range []int64{menuTypeDirectory, menuTypeItem, menuTypeExternal} {
		if err := validateMenuType(menuType); err != nil {
			t.Fatalf("expected menu type %d to be valid: %v", menuType, err)
		}
	}
	if err := validateMenuType(3); err == nil {
		t.Fatal("expected unsupported menu type to be rejected")
	}
}

func TestValidateExternalMenuURI(t *testing.T) {
	for _, uri := range []string{"https://example.com/docs", "http://example.com", "HTTPS://EXAMPLE.COM"} {
		if err := validateMenuURI(menuTypeExternal, uri); err != nil {
			t.Fatalf("expected external URI %q to be valid: %v", uri, err)
		}
	}
	for _, uri := range []string{"", "/internal", "https://", "javascript:alert(1)"} {
		if err := validateMenuURI(menuTypeExternal, uri); err == nil {
			t.Fatalf("expected external URI %q to be rejected", uri)
		}
	}
	if err := validateMenuURI(menuTypeItem, "/internal"); err != nil {
		t.Fatalf("expected internal menu URI to remain valid: %v", err)
	}
}

func TestValidateMenuComponent(t *testing.T) {
	valid := []string{
		"",
		"/kadmin/generated/ProductManagementView",
		"/dashboard/analytics/index",
	}
	for _, component := range valid {
		if err := validateMenuComponent(menuTypeItem, "/kadmin/products", component); err != nil {
			t.Fatalf("expected component %q to be valid: %v", component, err)
		}
	}

	invalid := []string{
		"/",
		"//kadmin",
		"/kadmin//ProductManagementView",
		"/kadmin/generated/",
		"kadmin/generated/ProductManagementView",
		"/kadmin/../secrets",
		"/kadmin/generated/ProductManagementView.vue",
		"/kadmin\\generated\\ProductManagementView",
		"/kadmin/generated/ProductManagementView?debug=1",
	}
	for _, component := range invalid {
		if err := validateMenuComponent(menuTypeItem, "/kadmin/products", component); err == nil {
			t.Fatalf("expected component %q to be rejected", component)
		}
	}

	if err := validateMenuComponent(menuTypeDirectory, "/kadmin", "/kadmin/generated/DirectoryView"); err == nil {
		t.Fatal("expected directory component to be rejected")
	}
	if err := validateMenuComponent(menuTypeExternal, "https://example.com", "/kadmin/generated/ExternalView"); err == nil {
		t.Fatal("expected external component to be rejected")
	}
	if err := validateMenuComponent(menuTypeItem, "https://example.com", "/kadmin/generated/ExternalView"); err == nil {
		t.Fatal("expected a component with an external menu URI to be rejected")
	}
}

func TestResolveMenuComponent(t *testing.T) {
	existing := "/kadmin/generated/ProductManagementView"
	component, err := resolveMenuComponent(menuTypeItem, "/kadmin/products", existing, nil)
	if err != nil || component != existing {
		t.Fatalf("omitted component = %q, %v; want preserved %q", component, err, existing)
	}

	empty := ""
	component, err = resolveMenuComponent(menuTypeItem, "/kadmin/products", existing, &empty)
	if err != nil || component != "" {
		t.Fatalf("explicit empty component = %q, %v; want cleared", component, err)
	}

	invalid := "/kadmin/generated/ExternalView"
	if _, err = resolveMenuComponent(menuTypeExternal, "https://example.com", existing, &invalid); err == nil {
		t.Fatal("expected explicit external-menu component to be rejected")
	}

	component, err = resolveMenuComponent(menuTypeDirectory, "/kadmin", existing, nil)
	if err != nil || component != "" {
		t.Fatalf("directory component = %q, %v; want cleared", component, err)
	}
}

func TestManagedMenuFromRowReadsComponent(t *testing.T) {
	menu := managedMenuFromRow(map[string]interface{}{
		"id":        int64(1),
		"component": []byte("/kadmin/generated/ProductManagementView"),
	})

	if menu.Component != "/kadmin/generated/ProductManagementView" {
		t.Fatalf("managed menu component = %q", menu.Component)
	}
}

func TestMenuSeedType(t *testing.T) {
	leaf := menuSeed{Title: "leaf"}
	directory := menuSeed{Title: "directory", Children: []menuSeed{leaf}}

	if got := menuSeedType(leaf); got != menuTypeItem {
		t.Fatalf("expected leaf type %d, got %d", menuTypeItem, got)
	}
	if got := menuSeedType(directory); got != menuTypeDirectory {
		t.Fatalf("expected directory type %d, got %d", menuTypeDirectory, got)
	}
}
