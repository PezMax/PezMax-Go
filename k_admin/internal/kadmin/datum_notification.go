package kadmin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/gin-gonic/gin"
)

// Desktop notification module (A7): management CRUD for ptmj_notification
// (the desktop hosts a management table page for these) plus the two client
// feeds — popups (per-user, sort-ordered) and scrolling banners. The client
// feed endpoints keep their legacy RuoYi paths (/system/notification/user/*)
// because PezMax-Desktop calls them verbatim.

var datumNotifyTypes = map[string]bool{"1": true, "2": true, "3": true, "4": true, "5": true}

func (s *Store) registerDatumNotificationRoutes(r *gin.Engine, datumGroup *gin.RouterGroup) {
	notifications := datumGroup.Group("/notification", s.requireDatumAuth())
	// gin v1.3: 静态 "list" 与 :notifyId 并存冲突，GET 统一走通配分发器。
	notifications.GET("/*rest", s.datumNotificationGet)
	notifications.POST("", s.datumNotificationCreate)
	notifications.PUT("", s.datumNotificationUpdate)
	notifications.DELETE("/:ids", s.datumNotificationDelete)

	// 旧 RuoYi 根路径（桌面端 home/NotificationCenter/TitleHeader 逐字调用），
	// 契约就是 /system/... 根路径，必须挂在引擎根而非 /datum 组下
	legacy := r.Group("/system/notification/user", s.requireDatumAuth(), cors())
	legacy.GET("/popup", s.datumNotificationPopup)
	legacy.GET("/scroll", s.datumNotificationScroll)
}

// datumNotificationGet dispatches GET /datum/notification/{list|:id}.
func (s *Store) datumNotificationGet(c *gin.Context) {
	rest := strings.Trim(c.Param("rest"), "/")
	switch {
	case rest == "list":
		s.datumNotificationList(c)
	default:
		if notifyID, err := strconv.ParseInt(rest, 10, 64); err == nil && notifyID > 0 {
			c.Params = append(c.Params, gin.Param{Key: "notifyId", Value: rest})
			s.datumNotificationDetail(c)
			return
		}
		fail(c, http.StatusNotFound, "接口不存在")
	}
}

func datumNotificationPayload(n datum.Notification) gin.H {
	return gin.H{
		"notifyId":              n.NotifyID,
		"notifyType":            n.NotifyType,
		"title":                 n.Title,
		"content":               n.Content,
		"status":                n.Status,
		"sort":                  n.Sort,
		"displayMode":           n.DisplayMode,
		"faultStartTime":        n.FaultStartTime,
		"faultEndTime":          n.FaultEndTime,
		"maintenanceStartTime":  n.MaintenanceStartTime,
		"maintenanceEndTime":    n.MaintenanceEndTime,
		"remindBeforeMinutes":   n.RemindBeforeMinutes,
		"uploadUserId":          n.UploadUserID,
		"materialId":            n.MaterialID,
		"materialTitleSnapshot": n.MaterialTitleSnapshot,
		"publishStart":          n.PublishStart,
		"publishEnd":            n.PublishEnd,
		"scrollTimeInterval":    n.ScrollTimeInterval,
		"remark":                n.Remark,
		"createTime":            n.CreateTime,
		"updateTime":            n.UpdateTime,
	}
}

func (s *Store) datumNotificationList(c *gin.Context) {
	page, size := datumPageParams(c)
	filter := datum.NotificationFilter{
		Page:         page,
		PageSize:     size,
		NotifyType:   strings.TrimSpace(c.Query("notifyType")),
		Title:        strings.TrimSpace(c.Query("title")),
		Status:       strings.TrimSpace(c.Query("status")),
		DisplayMode:  strings.TrimSpace(c.Query("displayMode")),
		UploadUserID: datumQueryInt(c, "uploadUserId"),
		MaterialID:   datumQueryInt(c, "materialId"),
	}
	result, err := datum.NewNotificationRepo(s.conn).List(filter)
	if err != nil {
		fail(c, http.StatusInternalServerError, "通知列表查询失败")
		return
	}
	items, _ := result.Items.([]datum.Notification)
	rows := make([]gin.H, 0, len(items))
	for _, item := range items {
		rows = append(rows, datumNotificationPayload(item))
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "msg": "ok", "rows": rows, "total": result.Total})
}

func (s *Store) datumNotificationDetail(c *gin.Context) {
	notifyID, err := strconv.ParseInt(c.Param("notifyId"), 10, 64)
	if err != nil || notifyID <= 0 {
		fail(c, http.StatusBadRequest, "通知 ID 不正确")
		return
	}
	notification, found, err := datum.NewNotificationRepo(s.conn).FindByID(notifyID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "通知查询失败")
		return
	}
	if !found {
		fail(c, http.StatusNotFound, "通知不存在")
		return
	}
	success(c, datumNotificationPayload(notification))
}

type datumNotificationRequest struct {
	NotifyID              int64  `json:"notifyId"`
	NotifyType            string `json:"notifyType"`
	Title                 string `json:"title"`
	Content               string `json:"content"`
	Status                string `json:"status"`
	Sort                  *int64 `json:"sort"`
	DisplayMode           string `json:"displayMode"`
	FaultStartTime        string `json:"faultStartTime"`
	FaultEndTime          string `json:"faultEndTime"`
	MaintenanceStartTime  string `json:"maintenanceStartTime"`
	MaintenanceEndTime    string `json:"maintenanceEndTime"`
	RemindBeforeMinutes   *int64 `json:"remindBeforeMinutes"`
	UploadUserID          *int64 `json:"uploadUserId"`
	MaterialID            *int64 `json:"materialId"`
	MaterialTitleSnapshot string `json:"materialTitleSnapshot"`
	PublishStart          string `json:"publishStart"`
	PublishEnd            string `json:"publishEnd"`
	ScrollTimeInterval    *int64 `json:"scrollTimeInterval"`
	Remark                string `json:"remark"`
}

func (req *datumNotificationRequest) validate() string {
	if !datumNotifyTypes[strings.TrimSpace(req.NotifyType)] {
		return "通知类型不正确：1-版本更新 2-系统故障 3-系统维护 4-资料下架 5-日常滚动"
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || len([]rune(req.Title)) > 200 {
		return "标题不能为空且不超过 200 字"
	}
	req.Status = strings.TrimSpace(req.Status)
	if req.Status != "0" && req.Status != "1" {
		return "状态不正确：0-启用 1-禁用"
	}
	req.DisplayMode = strings.TrimSpace(req.DisplayMode)
	if req.DisplayMode != "0" && req.DisplayMode != "1" {
		return "展示形态不正确：0-弹窗 1-滚动字幕"
	}
	if strings.TrimSpace(req.NotifyType) == "4" && (req.MaterialID == nil || *req.MaterialID <= 0) {
		return "资料下架通知必须指定资料 ID"
	}
	return ""
}

func (req *datumNotificationRequest) payload() datum.NotificationPayload {
	nilIfEmpty := func(value string) interface{} {
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return strings.TrimSpace(value)
	}
	nilIfPositive := func(value *int64) interface{} {
		if value == nil || *value <= 0 {
			return nil
		}
		return *value
	}
	scrollInterval := interface{}(30)
	if req.ScrollTimeInterval != nil && *req.ScrollTimeInterval >= 0 {
		scrollInterval = *req.ScrollTimeInterval
	}
	return datum.NotificationPayload{
		NotifyType:            strings.TrimSpace(req.NotifyType),
		Title:                 req.Title,
		Content:               req.Content,
		Status:                req.Status,
		Sort:                  derefInt64(req.Sort),
		DisplayMode:           req.DisplayMode,
		FaultStartTime:        nilIfEmpty(req.FaultStartTime),
		FaultEndTime:          nilIfEmpty(req.FaultEndTime),
		MaintenanceStartTime:  nilIfEmpty(req.MaintenanceStartTime),
		MaintenanceEndTime:    nilIfEmpty(req.MaintenanceEndTime),
		RemindBeforeMinutes:   nilIfPositive(req.RemindBeforeMinutes),
		UploadUserID:          nilIfPositive(req.UploadUserID),
		MaterialID:            nilIfPositive(req.MaterialID),
		MaterialTitleSnapshot: strings.TrimSpace(req.MaterialTitleSnapshot),
		PublishStart:          nilIfEmpty(req.PublishStart),
		PublishEnd:            nilIfEmpty(req.PublishEnd),
		ScrollTimeInterval:    scrollInterval,
		Remark:                strings.TrimSpace(req.Remark),
	}
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func (s *Store) datumNotificationCreate(c *gin.Context) {
	var req datumNotificationRequest
	_ = c.ShouldBind(&req)
	if message := req.validate(); message != "" {
		fail(c, http.StatusBadRequest, message)
		return
	}
	notifyID, err := datum.NewNotificationRepo(s.conn).Create(req.payload())
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "该资料的下架通知已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "通知创建失败")
		return
	}
	success(c, gin.H{"notifyId": notifyID})
}

func (s *Store) datumNotificationUpdate(c *gin.Context) {
	var req datumNotificationRequest
	_ = c.ShouldBind(&req)
	if message := req.validate(); message != "" {
		fail(c, http.StatusBadRequest, message)
		return
	}
	if req.NotifyID <= 0 {
		fail(c, http.StatusBadRequest, "通知 ID 不能为空")
		return
	}
	if err := datum.NewNotificationRepo(s.conn).Update(req.NotifyID, req.payload()); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "通知不存在")
			return
		}
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "该资料的下架通知已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "通知更新失败")
		return
	}
	success(c, true)
}

func (s *Store) datumNotificationDelete(c *gin.Context) {
	ids := make([]int64, 0)
	for _, raw := range strings.Split(c.Param("ids"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		fail(c, http.StatusBadRequest, "通知 ID 不能为空")
		return
	}
	deleted, err := datum.NewNotificationRepo(s.conn).DeleteByIDs(ids)
	if err != nil {
		fail(c, http.StatusInternalServerError, "通知删除失败")
		return
	}
	success(c, gin.H{"deleted": deleted})
}

func (s *Store) datumNotificationPopup(c *gin.Context) {
	claimed := datumQueryInt(c, "userId")
	userID, ok := datumSessionUser(c, claimed)
	if !ok {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	notifications, err := datum.NewNotificationRepo(s.conn).ActivePopup(now, userID)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "通知服务暂不可用，请稍后重试")
		return
	}
	rows := make([]gin.H, 0, len(notifications))
	for _, item := range notifications {
		rows = append(rows, datumNotificationPayload(item))
	}
	success(c, rows)
}

func (s *Store) datumNotificationScroll(c *gin.Context) {
	now := time.Now().Format("2006-01-02 15:04:05")
	notifications, err := datum.NewNotificationRepo(s.conn).ActiveScroll(now)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "通知服务暂不可用，请稍后重试")
		return
	}
	rows := make([]gin.H, 0, len(notifications))
	for _, item := range notifications {
		rows = append(rows, datumNotificationPayload(item))
	}
	success(c, rows)
}
