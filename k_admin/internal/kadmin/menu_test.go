package kadmin

import "testing"

func TestExpandAllowedMenuAncestors(t *testing.T) {
	items := []menuItem{
		{ID: 1, ParentID: 0},
		{ID: 2, ParentID: 1},
		{ID: 3, ParentID: 2},
		{ID: 4, ParentID: 1},
	}

	allowed := expandAllowedMenuAncestors(items, map[int64]bool{3: true})
	for _, id := range []int64{1, 2, 3} {
		if !allowed[id] {
			t.Fatalf("expected menu %d to be included", id)
		}
	}
	if allowed[4] {
		t.Fatal("did not expect an ungranted sibling to be included")
	}
}

func TestSortMenuItemsUsesStableHierarchyOrder(t *testing.T) {
	items := []menuItem{
		{ID: 4, ParentID: 1, Order: 0},
		{ID: 3, ParentID: 0, Order: 1},
		{ID: 2, ParentID: 0, Order: 0},
		{ID: 1, ParentID: 0, Order: 0},
	}

	sortMenuItems(items)
	want := []int64{1, 2, 3, 4}
	for index, id := range want {
		if items[index].ID != id {
			t.Fatalf("expected id %d at index %d, got %d", id, index, items[index].ID)
		}
	}
}

func TestBuildMenuTreePreservesSortedHierarchyOrder(t *testing.T) {
	items := []menuItem{
		{ID: 22, ParentID: 20, Type: menuTypeItem, Order: 0, Title: "Workspace", URI: "/dashboard/workspace"},
		{ID: 13, ParentID: 10, Type: menuTypeItem, Order: 1, Title: "Menus", URI: "/kadmin/menus"},
		{ID: 20, ParentID: 0, Type: menuTypeDirectory, Order: 0, Title: "Dashboard", URI: "/dashboard"},
		{ID: 11, ParentID: 10, Type: menuTypeItem, Order: 2, Title: "Users", URI: "/kadmin/users"},
		{ID: 10, ParentID: 0, Type: menuTypeDirectory, Order: 1, Title: "KAdmin", URI: "/kadmin"},
		{ID: 21, ParentID: 20, Type: menuTypeItem, Order: 1, Title: "Analytics", URI: "/dashboard/analytics"},
		{ID: 12, ParentID: 10, Type: menuTypeItem, Order: 0, Title: "RBAC", URI: "/kadmin/rbac"},
	}

	sortMenuItems(items)
	tree := buildMenuTree(items, 0)

	assertOrder := func(label string, menus []vbenMenu, wantIDs, wantOrders []int64) {
		t.Helper()
		if len(menus) != len(wantIDs) {
			t.Fatalf("expected %d %s menus, got %d", len(wantIDs), label, len(menus))
		}
		for index, wantID := range wantIDs {
			if menus[index].ID != wantID {
				t.Fatalf("expected %s menu id %d at index %d, got %d", label, wantID, index, menus[index].ID)
			}
			if got := menus[index].Meta["order"]; got != wantOrders[index] {
				t.Fatalf("expected %s menu %d meta.order %d, got %v", label, wantID, wantOrders[index], got)
			}
		}
	}

	assertOrder("root", tree, []int64{20, 10}, []int64{0, 1})
	assertOrder("dashboard child", tree[0].Children, []int64{22, 21}, []int64{0, 1})
	assertOrder("kadmin child", tree[1].Children, []int64{12, 13, 11}, []int64{0, 1, 2})
	if tree[0].Children[1].Meta["affixTab"] != true {
		t.Fatal("expected analytics route metadata to keep affixTab")
	}

	for _, root := range tree {
		if root.Redirect != root.Children[0].Path {
			t.Fatalf("expected menu %d redirect %q, got %q", root.ID, root.Children[0].Path, root.Redirect)
		}
	}
}

func TestDirectoryWithoutChildrenDoesNotBecomePage(t *testing.T) {
	menu := menuItem{
		ID:        1,
		Type:      menuTypeDirectory,
		Title:     "Empty directory",
		URI:       "/custom-directory",
		Component: "/kadmin/generated/InvalidDirectoryView",
	}.toVbenMenu(nil)

	if menu.Component != "" {
		t.Fatalf("expected directory component to be empty, got %q", menu.Component)
	}
	if menu.Path != "/goadmin/custom-directory" {
		t.Fatalf("invalid directory component changed route path to %q", menu.Path)
	}
}

func TestDatabaseComponentCreatesNativeVbenRoute(t *testing.T) {
	menu := menuItem{
		ID:        41,
		Type:      menuTypeItem,
		Title:     "Products",
		URI:       "/kadmin/products",
		Component: "/kadmin/generated/ProductManagementView",
	}.toVbenMenu(nil)

	if menu.Path != "/kadmin/products" {
		t.Fatalf("database-backed component path = %q, want %q", menu.Path, "/kadmin/products")
	}
	if menu.Component != "/kadmin/generated/ProductManagementView" {
		t.Fatalf("database-backed component = %q", menu.Component)
	}
	if _, exists := menu.Meta["iframeSrc"]; exists {
		t.Fatalf("database-backed component must not use iframe metadata: %#v", menu.Meta)
	}
}

func TestDatabaseComponentOverridesStaticBindingComponent(t *testing.T) {
	menu := menuItem{
		ID:        42,
		Type:      menuTypeItem,
		Title:     "Users",
		URI:       "/kadmin/users",
		Component: "/kadmin/generated/CustomUserManagementView",
	}.toVbenMenu(nil)

	if menu.Path != "/kadmin/users" {
		t.Fatalf("bound menu path = %q, want %q", menu.Path, "/kadmin/users")
	}
	if menu.Component != "/kadmin/generated/CustomUserManagementView" {
		t.Fatalf("database component must override the static fallback, got %q", menu.Component)
	}
}

func TestDatabaseComponentKeepsNativeURIOverStaticAlias(t *testing.T) {
	menu := menuItem{
		ID:        43,
		Type:      menuTypeItem,
		Title:     "Generated root",
		URI:       "/",
		Component: "/kadmin/generated/GeneratedRootView",
	}.toVbenMenu(nil)

	if menu.Path != "/" {
		t.Fatalf("database-backed alias path = %q, want native URI %q", menu.Path, "/")
	}
	if menu.Component != "/kadmin/generated/GeneratedRootView" {
		t.Fatalf("database-backed alias component = %q", menu.Component)
	}
}

func TestEmptyDatabaseComponentUsesStaticBindingFallback(t *testing.T) {
	menu := menuItem{
		ID:    44,
		Type:  menuTypeItem,
		Title: "Users",
		URI:   "/kadmin/users",
	}.toVbenMenu(nil)

	if menu.Path != "/kadmin/users" || menu.Component != "/kadmin/components/UserManagementView" {
		t.Fatalf("static binding fallback = path %q, component %q", menu.Path, menu.Component)
	}
}

func TestEmptyDatabaseComponentUsesLegacyIframeFallback(t *testing.T) {
	menu := menuItem{
		ID:    45,
		Type:  menuTypeItem,
		Title: "Legacy report",
		URI:   "/reports/daily",
	}.toVbenMenu(nil)

	if menu.Path != "/goadmin/reports/daily" || menu.Component != "IFrameView" {
		t.Fatalf("iframe fallback = path %q, component %q", menu.Path, menu.Component)
	}
	if menu.Meta["iframeSrc"] != "/admin/reports/daily" {
		t.Fatalf("iframe fallback metadata = %#v", menu.Meta)
	}
}

func TestHTTPMenuItemUsesLegacyIframeFallback(t *testing.T) {
	menu := menuItem{
		ID:    46,
		Type:  menuTypeItem,
		Title: "Embedded report",
		URI:   "https://example.com/reports/daily",
	}.toVbenMenu(nil)

	if menu.Component != "IFrameView" {
		t.Fatalf("HTTP menu item component = %q, want IFrameView", menu.Component)
	}
	if menu.Meta["iframeSrc"] != "https://example.com/reports/daily" {
		t.Fatalf("HTTP menu item iframe metadata = %#v", menu.Meta)
	}
	if _, exists := menu.Meta["link"]; exists {
		t.Fatalf("legacy iframe menu must not become an external-link menu: %#v", menu.Meta)
	}
}

func TestExternalMenuRequiresConfirmationMetadata(t *testing.T) {
	menu := menuItem{
		ID:        31,
		Type:      menuTypeExternal,
		Title:     "Docs",
		URI:       "https://example.com/docs",
		Component: "/kadmin/generated/InvalidExternalView",
	}.toVbenMenu(nil)

	if menu.Meta["link"] != "https://example.com/docs" {
		t.Fatalf("expected external link metadata, got %#v", menu.Meta)
	}
	if menu.Meta["openInNewWindow"] != true || menu.Meta["confirmExternal"] != true {
		t.Fatalf("expected confirmed new-window metadata, got %#v", menu.Meta)
	}
	if menu.Component != "" {
		t.Fatalf("expected external menu component to be empty, got %q", menu.Component)
	}
	if _, exists := menu.Meta["iframeSrc"]; exists {
		t.Fatalf("external menu must not use iframe metadata: %#v", menu.Meta)
	}
}

func TestApplyExternalLinkTargetSupportsCurrentPage(t *testing.T) {
	menus := []vbenMenu{
		{Meta: map[string]interface{}{"title": "Directory"}, Children: []vbenMenu{
			{Meta: map[string]interface{}{"link": "https://example.com", "openInNewWindow": true}},
		}},
		{Meta: map[string]interface{}{"title": "Internal"}},
	}

	applyExternalLinkTarget(menus, "current_page")
	if got := menus[0].Children[0].Meta["openInNewWindow"]; got != false {
		t.Fatalf("current-page external target = %v, want false", got)
	}
	if _, exists := menus[1].Meta["openInNewWindow"]; exists {
		t.Fatalf("internal menu must not receive external target metadata: %#v", menus[1].Meta)
	}

	applyExternalLinkTarget(menus, "new_tab")
	if got := menus[0].Children[0].Meta["openInNewWindow"]; got != true {
		t.Fatalf("new-tab external target = %v, want true", got)
	}
}
