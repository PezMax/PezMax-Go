package kadmin

import (
	"net/http"
)

// registerDatumBusinessAuditResolver 注册 /datum 前缀的业务审计描述器，覆盖
// datum 管理侧变更操作（平台用户管理、通知管理、举报审核）。属主语义的桌面端
// 流量（资料/书签 CRUD、收藏下载、举报提交等）与缓存清理刻意不匹配——返回
// false 时中间件直通，不会落审计事件。与 generated 模块的
// RegisterBusinessAuditResource 不同，datum 契约是两级路径且更新走 body 传递
// id，因此无法套用单表 CRUD 约定，需要独立分发。
func registerDatumBusinessAuditResolver() {
	businessAuditRegistryMu.Lock()
	defer businessAuditRegistryMu.Unlock()
	businessAuditRegistry["datum"] = func(method string, parts []string) (businessAuditDescriptor, bool) {
		if len(parts) < 2 {
			return businessAuditDescriptor{}, false
		}
		switch parts[1] {
		case "user":
			switch {
			case len(parts) == 2 && method == http.MethodPost:
				return businessAuditDescriptor{Action: "create", Resource: "platform-user", ResourceID: "new"}, true
			case len(parts) == 2 && (method == http.MethodPut || method == http.MethodPatch):
				return businessAuditDescriptor{Action: "update", Resource: "platform-user", ResourceID: "batch"}, true
			case len(parts) == 3 && method == http.MethodDelete:
				return businessAuditDescriptor{Action: "delete", Resource: "platform-user", ResourceID: parts[2]}, true
			}
		case "notification":
			switch {
			case len(parts) == 2 && method == http.MethodPost:
				return businessAuditDescriptor{Action: "create", Resource: "datum-notification", ResourceID: "new"}, true
			case len(parts) == 2 && (method == http.MethodPut || method == http.MethodPatch):
				return businessAuditDescriptor{Action: "update", Resource: "datum-notification", ResourceID: "batch"}, true
			case len(parts) == 3 && method == http.MethodDelete:
				return businessAuditDescriptor{Action: "delete", Resource: "datum-notification", ResourceID: parts[2]}, true
			}
		case "report":
			if len(parts) == 4 && parts[2] == "audit" && method == http.MethodPost {
				return businessAuditDescriptor{Action: "audit", Resource: "report", ResourceID: parts[3]}, true
			}
		case "bookmarkReport":
			if len(parts) == 4 && parts[2] == "audit" && method == http.MethodPost {
				return businessAuditDescriptor{Action: "audit", Resource: "bookmark-report", ResourceID: parts[3]}, true
			}
		}
		return businessAuditDescriptor{}, false
	}
}
