package kadmin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/bootstrap"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/modules/db/dialect"
	"github.com/gin-gonic/gin"
)

type dictionaryType struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Sort        int64  `json:"sort"`
	Status      int64  `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type dictionaryData struct {
	ID        int64  `json:"id"`
	DictType  string `json:"dictType"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	Color     string `json:"color"`
	CSSClass  string `json:"cssClass"`
	IsDefault bool   `json:"isDefault"`
	Sort      int64  `json:"sort"`
	Status    int64  `json:"status"`
	Remark    string `json:"remark"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type dictionaryOverview struct {
	Types []dictionaryType `json:"types"`
	Data  []dictionaryData `json:"data"`
}

type dictionaryTypePayload struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Sort        int64  `json:"sort"`
	Status      int64  `json:"status"`
}

type dictionaryDataPayload struct {
	DictType  string `json:"dictType"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	Color     string `json:"color"`
	CSSClass  string `json:"cssClass"`
	IsDefault bool   `json:"isDefault"`
	Sort      int64  `json:"sort"`
	Status    int64  `json:"status"`
	Remark    string `json:"remark"`
}

func registerDictionaryRoutes(api *gin.RouterGroup, s *Store) {
	dictGroup := api.Group("/dictionaries", s.requireAuth())
	dictGroup.GET("/overview", s.requirePermission(dictionaryManagePermission), s.dictionaryOverview)
	dictGroup.GET("/types", s.requirePermission(dictionaryManagePermission), s.listDictionaryTypes)
	dictGroup.POST("/types", s.requirePermission(bootstrap.DictionaryTypeCreatePermission), s.createDictionaryType)
	dictGroup.PUT("/types/:id", s.requirePermission(bootstrap.DictionaryTypeUpdatePermission), s.updateDictionaryType)
	dictGroup.DELETE("/types/:id", s.requirePermission(bootstrap.DictionaryTypeDeletePermission), s.deleteDictionaryType)
	dictGroup.GET("/data", s.requirePermission(dictionaryManagePermission), s.listDictionaryData)
	dictGroup.POST("/data", s.requirePermission(bootstrap.DictionaryDataCreatePermission), s.createDictionaryData)
	dictGroup.PUT("/data/:id", s.requirePermission(bootstrap.DictionaryDataUpdatePermission), s.updateDictionaryData)
	dictGroup.DELETE("/data/:id", s.requirePermission(bootstrap.DictionaryDataDeletePermission), s.deleteDictionaryData)
}

func (s *Store) dictionaryOverview(c *gin.Context) {
	types, err := s.loadDictionaryTypes(dictionaryTypeFilter{})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	data, err := s.loadDictionaryData(dictionaryDataFilter{})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, dictionaryOverview{Types: types, Data: data})
}

func (s *Store) listDictionaryTypes(c *gin.Context) {
	types, err := s.loadDictionaryTypes(dictionaryTypeFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Status:  strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, gin.H{
		"items": types,
		"total": len(types),
	})
}

func (s *Store) createDictionaryType(c *gin.Context) {
	var req dictionaryTypePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid dictionary type payload")
		return
	}
	normalizeDictionaryTypePayload(&req)
	if req.Name == "" || req.Code == "" {
		fail(c, http.StatusBadRequest, "dictionary type name and code are required")
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}

	id, err := db.WithDriver(s.conn).Table("goadmin_dict_type").Insert(dialect.H{
		"name":        req.Name,
		"code":        req.Code,
		"description": req.Description,
		"sort":        req.Sort,
		"status":      req.Status,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	dictType, err := s.loadDictionaryType(id)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, dictType)
}

func (s *Store) updateDictionaryType(c *gin.Context) {
	typeID, ok := pathID(c)
	if !ok {
		return
	}
	oldType, err := s.loadDictionaryType(typeID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if oldType.ID == 0 {
		fail(c, http.StatusNotFound, "dictionary type not found")
		return
	}

	var req dictionaryTypePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid dictionary type payload")
		return
	}
	normalizeDictionaryTypePayload(&req)
	if req.Name == "" || req.Code == "" {
		fail(c, http.StatusBadRequest, "dictionary type name and code are required")
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}

	_, err = db.WithDriver(s.conn).Table("goadmin_dict_type").Where("id", "=", typeID).Update(dialect.H{
		"name":        req.Name,
		"code":        req.Code,
		"description": req.Description,
		"sort":        req.Sort,
		"status":      req.Status,
		"updated_at":  nowString(),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if oldType.Code != req.Code {
		_, err = db.WithDriver(s.conn).Table("goadmin_dict_data").Where("dict_type", "=", oldType.Code).Update(dialect.H{
			"dict_type":  req.Code,
			"updated_at": nowString(),
		})
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	dictType, err := s.loadDictionaryType(typeID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, dictType)
}

func (s *Store) deleteDictionaryType(c *gin.Context) {
	typeID, ok := pathID(c)
	if !ok {
		return
	}
	dictType, err := s.loadDictionaryType(typeID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if dictType.ID == 0 {
		fail(c, http.StatusNotFound, "dictionary type not found")
		return
	}

	_ = db.WithDriver(s.conn).Table("goadmin_dict_data").Where("dict_type", "=", dictType.Code).Delete()
	if err := db.WithDriver(s.conn).Table("goadmin_dict_type").Where("id", "=", typeID).Delete(); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, true)
}

func (s *Store) listDictionaryData(c *gin.Context) {
	items, err := s.loadDictionaryData(dictionaryDataFilter{
		DictType: strings.TrimSpace(c.Query("dictType")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, gin.H{
		"items": items,
		"total": len(items),
	})
}

func (s *Store) createDictionaryData(c *gin.Context) {
	var req dictionaryDataPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid dictionary data payload")
		return
	}
	normalizeDictionaryDataPayload(&req)
	if req.DictType == "" || req.Label == "" || req.Value == "" {
		fail(c, http.StatusBadRequest, "dictionary type, label and value are required")
		return
	}
	existingType, err := s.loadDictionaryTypeByCode(req.DictType)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existingType.ID == 0 {
		fail(c, http.StatusBadRequest, "dictionary type does not exist")
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}

	id, err := db.WithDriver(s.conn).Table("goadmin_dict_data").Insert(dialect.H{
		"dict_type":  req.DictType,
		"label":      req.Label,
		"value":      req.Value,
		"color":      req.Color,
		"css_class":  req.CSSClass,
		"is_default": boolToInt(req.IsDefault),
		"sort":       req.Sort,
		"status":     req.Status,
		"remark":     req.Remark,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	item, err := s.loadDictionaryDataItem(id)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, item)
}

func (s *Store) updateDictionaryData(c *gin.Context) {
	itemID, ok := pathID(c)
	if !ok {
		return
	}
	var req dictionaryDataPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid dictionary data payload")
		return
	}
	normalizeDictionaryDataPayload(&req)
	if req.DictType == "" || req.Label == "" || req.Value == "" {
		fail(c, http.StatusBadRequest, "dictionary type, label and value are required")
		return
	}
	existingType, err := s.loadDictionaryTypeByCode(req.DictType)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existingType.ID == 0 {
		fail(c, http.StatusBadRequest, "dictionary type does not exist")
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}

	_, err = db.WithDriver(s.conn).Table("goadmin_dict_data").Where("id", "=", itemID).Update(dialect.H{
		"dict_type":  req.DictType,
		"label":      req.Label,
		"value":      req.Value,
		"color":      req.Color,
		"css_class":  req.CSSClass,
		"is_default": boolToInt(req.IsDefault),
		"sort":       req.Sort,
		"status":     req.Status,
		"remark":     req.Remark,
		"updated_at": nowString(),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	item, err := s.loadDictionaryDataItem(itemID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, item)
}

func (s *Store) deleteDictionaryData(c *gin.Context) {
	itemID, ok := pathID(c)
	if !ok {
		return
	}
	if err := db.WithDriver(s.conn).Table("goadmin_dict_data").Where("id", "=", itemID).Delete(); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, true)
}

type dictionaryTypeFilter struct {
	Keyword string
	Status  string
}

type dictionaryDataFilter struct {
	DictType string
	Keyword  string
	Status   string
}

func (s *Store) loadDictionaryType(id int64) (dictionaryType, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_dict_type").Where("id", "=", id).All()
	if err != nil {
		return dictionaryType{}, err
	}
	if len(rows) == 0 {
		return dictionaryType{}, nil
	}
	return dictionaryTypeFromRow(rows[0]), nil
}

func (s *Store) loadDictionaryTypeByCode(code string) (dictionaryType, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_dict_type").Where("code", "=", code).All()
	if err != nil {
		return dictionaryType{}, err
	}
	if len(rows) == 0 {
		return dictionaryType{}, nil
	}
	return dictionaryTypeFromRow(rows[0]), nil
}

func (s *Store) loadDictionaryDataItem(id int64) (dictionaryData, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_dict_data").Where("id", "=", id).All()
	if err != nil {
		return dictionaryData{}, err
	}
	if len(rows) == 0 {
		return dictionaryData{}, nil
	}
	return dictionaryDataFromRow(rows[0]), nil
}

func (s *Store) loadDictionaryTypes(filter dictionaryTypeFilter) ([]dictionaryType, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_dict_type").OrderBy("sort", "asc").All()
	if err != nil {
		return nil, err
	}
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	status := strings.TrimSpace(filter.Status)
	items := make([]dictionaryType, 0, len(rows))
	for _, row := range rows {
		item := dictionaryTypeFromRow(row)
		if status != "" && status != toStringStatus(item.Status) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.Name+" "+item.Code+" "+item.Description), keyword) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) loadDictionaryData(filter dictionaryDataFilter) ([]dictionaryData, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_dict_data").OrderBy("sort", "asc").All()
	if err != nil {
		return nil, err
	}
	dictType := strings.TrimSpace(filter.DictType)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	status := strings.TrimSpace(filter.Status)
	items := make([]dictionaryData, 0, len(rows))
	for _, row := range rows {
		item := dictionaryDataFromRow(row)
		if dictType != "" && item.DictType != dictType {
			continue
		}
		if status != "" && status != toStringStatus(item.Status) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.Label+" "+item.Value+" "+item.Remark), keyword) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func dictionaryTypeFromRow(row map[string]interface{}) dictionaryType {
	return dictionaryType{
		ID:          toInt64(row["id"]),
		Name:        toString(row["name"]),
		Code:        toString(row["code"]),
		Description: toString(row["description"]),
		Sort:        toInt64(row["sort"]),
		Status:      toInt64(row["status"]),
		CreatedAt:   toDateTimeString(row["created_at"]),
		UpdatedAt:   toDateTimeString(row["updated_at"]),
	}
}

func dictionaryDataFromRow(row map[string]interface{}) dictionaryData {
	return dictionaryData{
		ID:        toInt64(row["id"]),
		DictType:  toString(row["dict_type"]),
		Label:     toString(row["label"]),
		Value:     toString(row["value"]),
		Color:     toString(row["color"]),
		CSSClass:  toString(row["css_class"]),
		IsDefault: toInt64(row["is_default"]) == 1,
		Sort:      toInt64(row["sort"]),
		Status:    toInt64(row["status"]),
		Remark:    toString(row["remark"]),
		CreatedAt: toDateTimeString(row["created_at"]),
		UpdatedAt: toDateTimeString(row["updated_at"]),
	}
}

func normalizeDictionaryTypePayload(req *dictionaryTypePayload) {
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	req.Description = strings.TrimSpace(req.Description)
	if req.Sort < 0 {
		req.Sort = 0
	}
}

func normalizeDictionaryDataPayload(req *dictionaryDataPayload) {
	req.DictType = strings.TrimSpace(req.DictType)
	req.Label = strings.TrimSpace(req.Label)
	req.Value = strings.TrimSpace(req.Value)
	req.Color = strings.TrimSpace(req.Color)
	req.CSSClass = strings.TrimSpace(req.CSSClass)
	req.Remark = strings.TrimSpace(req.Remark)
	if req.Sort < 0 {
		req.Sort = 0
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func toStringStatus(status int64) string {
	return strconv.FormatInt(status, 10)
}
