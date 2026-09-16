package kadmin

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loginlogs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/notifications"
	"github.com/GoAdminGroup/go-admin/modules/auth"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/modules/db/dialect"
	"github.com/gin-gonic/gin"
)

type managedUser struct {
	ID            int64    `json:"id"`
	Username      string   `json:"username"`
	Name          string   `json:"name"`
	Avatar        string   `json:"avatar"`
	Status        string   `json:"status"`
	RoleIDs       []int64  `json:"roleIds"`
	Roles         []string `json:"roles"`
	DepartmentIDs []int64  `json:"departmentIds"`
	Departments   []string `json:"departments"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
}

type managedUserRoleOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type managedUserDepartmentOption struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	RoleIDs []int64 `json:"roleIds"`
}

type managedUserOptionsResponse struct {
	Roles       []managedUserRoleOption       `json:"roles"`
	Departments []managedUserDepartmentOption `json:"departments"`
}

type managedUserDepartment struct {
	ID   int64
	Name string
}

type managedUserListResponse struct {
	Items []managedUser `json:"items"`
	Total int           `json:"total"`
}

type userPayload struct {
	Username string  `json:"username"`
	Password string  `json:"password"`
	Name     string  `json:"name"`
	Avatar   string  `json:"avatar"`
	Status   string  `json:"status"`
	RoleIDs  []int64 `json:"roleIds"`
}

type resetPasswordPayload struct {
	Password string `json:"password"`
}

type userStatusPayload struct {
	Status string `json:"status"`
}

type unlockUserPayload struct {
	IP string `json:"ip"`
}

type importUsersPayload struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

func registerUserManagementRoutes(api *gin.RouterGroup, s *Store) {
	usersGroup := api.Group("/users", s.requireAuth())
	usersGroup.GET("", s.requirePermission(userManagePermission), s.listManagedUsers)
	usersGroup.POST("", s.requirePermission(userCreatePermission), s.createManagedUser)
	usersGroup.GET("/export", s.requirePermission(userExportPermission), s.exportManagedUsers)
	usersGroup.GET("/options", s.requirePermission(userManagePermission), s.managedUserOptions)
	usersGroup.POST("/import", s.requirePermission(userImportPermission), s.importManagedUsers)
	usersGroup.PUT("/:id/status", s.requirePermission(userStatusPermission), s.updateManagedUserStatus)
	usersGroup.PUT("/:id", s.requirePermission(userUpdatePermission), s.updateManagedUser)
	usersGroup.DELETE("/:id", s.requirePermission(userDeletePermission), s.deleteManagedUser)
	usersGroup.PUT("/:id/password", s.requirePermission(userPasswordPermission), s.resetManagedUserPassword)
	usersGroup.PUT("/:id/unlock", s.requirePermission(userUnlockPermission), s.unlockManagedUser)
}

func (s *Store) listManagedUsers(c *gin.Context) {
	users, err := s.loadManagedUsers()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	keyword := strings.ToLower(strings.TrimSpace(c.Query("keyword")))
	departmentKeyword := strings.ToLower(strings.TrimSpace(c.Query("department")))
	roleKeyword := strings.ToLower(strings.TrimSpace(c.Query("role")))
	status := strings.TrimSpace(c.Query("status"))
	if keyword != "" || departmentKeyword != "" || roleKeyword != "" || status != "" {
		filtered := make([]managedUser, 0, len(users))
		for _, user := range users {
			if matchesManagedUserFilter(user, keyword, departmentKeyword, roleKeyword, status) {
				filtered = append(filtered, user)
			}
		}
		users = filtered
	}

	success(c, managedUserListResponse{Items: users, Total: len(users)})
}

func (s *Store) managedUserOptions(c *gin.Context) {
	roles, err := s.loadRBACRoles()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	departments, err := s.loadRBACDepartments()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response := managedUserOptionsResponse{
		Roles:       make([]managedUserRoleOption, 0, len(roles)),
		Departments: make([]managedUserDepartmentOption, 0, len(departments)),
	}
	for _, role := range roles {
		response.Roles = append(response.Roles, managedUserRoleOption{ID: role.ID, Name: role.Name, Slug: role.Slug})
	}
	for _, department := range departments {
		response.Departments = append(response.Departments, managedUserDepartmentOption{
			ID: department.ID, Name: department.Name, RoleIDs: append([]int64(nil), department.RoleIDs...),
		})
	}
	success(c, response)
}

func (s *Store) createManagedUser(c *gin.Context) {
	var req userPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid user payload")
		return
	}
	normalizeUserPayload(&req)
	if req.Username == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, "username and password are required")
		return
	}
	if err := validatePassword(req.Password); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if exists, err := s.usernameExists(req.Username, 0); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	} else if exists {
		fail(c, http.StatusBadRequest, "username already exists")
		return
	}
	if req.Name == "" {
		req.Name = req.Username
	}

	id, err := db.WithDriver(s.conn).Table("goadmin_users").Insert(dialect.H{
		"username": req.Username,
		"password": auth.EncodePassword([]byte(req.Password)),
		"name":     req.Name,
		"avatar":   req.Avatar,
		"status":   req.Status,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.replaceUserRoles(id, req.RoleIDs); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	user, err := s.loadManagedUser(id)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.pushSystemNotification(
		"新用户创建",
		"管理员创建了新用户 "+user.Username+"（"+user.Name+"）",
		"/kadmin/users",
		notifications.TypeInfo,
	)
	success(c, user)
}

// pushSystemNotification writes an in-site notification without failing the
// caller when the notification pipeline itself errors.
func (s *Store) pushSystemNotification(title, content, link, notificationType string) {
	notifier := notifications.NewNotifier(s.conn)
	if _, err := notifier.Push(notifications.Payload{
		Title: title, Content: content, Link: link, Type: notificationType,
	}); err != nil {
		log.Printf("写入站内通知失败: %v", err)
	}
}

func (s *Store) updateManagedUser(c *gin.Context) {
	userID, ok := pathID(c)
	if !ok {
		return
	}
	var req userPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid user payload")
		return
	}
	normalizeUserPayload(&req)
	if req.Username == "" {
		fail(c, http.StatusBadRequest, "username is required")
		return
	}
	if req.Password != "" {
		fail(c, http.StatusBadRequest, "use reset password api to update password")
		return
	}
	if exists, err := s.usernameExists(req.Username, userID); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	} else if exists {
		fail(c, http.StatusBadRequest, "username already exists")
		return
	}
	if req.Name == "" {
		req.Name = req.Username
	}
	if protectedAdminUser(userID) {
		req.Status = "enable"
	}

	_, err := db.WithDriver(s.conn).
		Table("goadmin_users").
		Where("id", "=", userID).
		Update(dialect.H{
			"username":   req.Username,
			"name":       req.Name,
			"avatar":     req.Avatar,
			"status":     req.Status,
			"updated_at": nowString(),
		})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !protectedAdminUser(userID) {
		if err := s.replaceUserRoles(userID, req.RoleIDs); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	user, err := s.loadManagedUser(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, user)
}

func (s *Store) updateManagedUserStatus(c *gin.Context) {
	userID, ok := pathID(c)
	if !ok {
		return
	}
	if protectedAdminUser(userID) {
		fail(c, http.StatusBadRequest, "admin user cannot be disabled")
		return
	}

	var req userStatusPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid user status payload")
		return
	}
	status := strings.TrimSpace(req.Status)
	if status != "enable" && status != "disable" {
		fail(c, http.StatusBadRequest, "invalid user status")
		return
	}

	_, err := db.WithDriver(s.conn).
		Table("goadmin_users").
		Where("id", "=", userID).
		Update(dialect.H{
			"status":     status,
			"updated_at": nowString(),
		})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	user, err := s.loadManagedUser(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, user)
}

func (s *Store) deleteManagedUser(c *gin.Context) {
	userID, ok := pathID(c)
	if !ok {
		return
	}
	if protectedAdminUser(userID) {
		fail(c, http.StatusBadRequest, "admin user cannot be deleted")
		return
	}

	_ = db.WithDriver(s.conn).Table("goadmin_role_users").Where("user_id", "=", userID).Delete()
	_ = db.WithDriver(s.conn).Table("goadmin_user_permissions").Where("user_id", "=", userID).Delete()
	if err := db.WithDriver(s.conn).Table("goadmin_users").Where("id", "=", userID).Delete(); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, true)
}

func (s *Store) resetManagedUserPassword(c *gin.Context) {
	userID, ok := pathID(c)
	if !ok {
		return
	}
	var req resetPasswordPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid password payload")
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	if err := validatePassword(req.Password); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := db.WithDriver(s.conn).
		Table("goadmin_users").
		Where("id", "=", userID).
		Update(dialect.H{
			"password":   auth.EncodePassword([]byte(req.Password)),
			"updated_at": nowString(),
		})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, true)
}

func (s *Store) unlockManagedUser(c *gin.Context) {
	userID, ok := pathID(c)
	if !ok {
		return
	}
	user, err := s.loadManagedUser(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if user.ID == 0 {
		fail(c, http.StatusNotFound, "user not found")
		return
	}
	var req unlockUserPayload
	_ = c.ShouldBindJSON(&req)
	if err := s.security.unlockLogin(user.Username, strings.TrimSpace(req.IP)); err != nil {
		fail(c, http.StatusServiceUnavailable, "login security storage unavailable")
		return
	}
	targetID := user.ID
	s.recordLoginAttempt(c, time.Now(), user.Username, &targetID, loginlogs.ResultAccountUnlocked, "管理员解除临时登录锁定")
	success(c, true)
}

func (s *Store) exportManagedUsers(c *gin.Context) {
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "csv")))
	users, err := s.loadManagedUsers()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	switch format {
	case "sql":
		content := exportUsersSQL(users)
		c.Header("Content-Disposition", `attachment; filename="users.sql"`)
		c.Data(http.StatusOK, "application/sql; charset=utf-8", []byte(content))
	case "xlsx", "excel":
		content, err := exportUsersExcel(users)
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.Header("Content-Disposition", `attachment; filename="users.xlsx"`)
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
	default:
		content, err := exportUsersCSV(users)
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.Header("Content-Disposition", `attachment; filename="users.csv"`)
		c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte(content))
	}
}

func (s *Store) importManagedUsers(c *gin.Context) {
	var req importUsersPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid import payload")
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "csv"
	}

	var payloads []userPayload
	var err error
	switch format {
	case "sql":
		payloads, err = parseUsersSQL(req.Content)
	case "xlsx", "excel":
		payloads, err = parseUsersExcel(req.Content)
	default:
		payloads, err = parseUsersCSV(req.Content)
	}
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	imported := 0
	for _, payload := range payloads {
		normalizeUserPayload(&payload)
		if payload.Username == "" {
			continue
		}
		if payload.Password == "" {
			payload.Password = "Admin@123456"
		}
		if err := validatePassword(payload.Password); err != nil {
			fail(c, http.StatusBadRequest, fmt.Sprintf("%s: %s", payload.Username, err.Error()))
			return
		}
		if payload.Name == "" {
			payload.Name = payload.Username
		}
		exists, err := s.usernameExists(payload.Username, 0)
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if exists {
			continue
		}
		id, err := db.WithDriver(s.conn).Table("goadmin_users").Insert(dialect.H{
			"username": payload.Username,
			"password": auth.EncodePassword([]byte(payload.Password)),
			"name":     payload.Name,
			"avatar":   payload.Avatar,
			"status":   payload.Status,
		})
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if err := s.replaceUserRoles(id, payload.RoleIDs); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		imported++
	}
	success(c, gin.H{"imported": imported})
}

func (s *Store) loadManagedUser(id int64) (managedUser, error) {
	users, err := s.loadManagedUsers()
	if err != nil {
		return managedUser{}, err
	}
	for _, user := range users {
		if user.ID == id {
			return user, nil
		}
	}
	return managedUser{}, nil
}

func (s *Store) loadManagedUsers() ([]managedUser, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_users").
		OrderBy("id", "asc").
		All()
	if err != nil {
		return nil, err
	}

	roleIDsByUser, roleNamesByUser, err := s.loadUserRoles()
	if err != nil {
		return nil, err
	}
	departmentIDsByUser, departmentNamesByUser, err := s.loadUserDepartments(roleIDsByUser)
	if err != nil {
		return nil, err
	}

	users := make([]managedUser, 0, len(rows))
	for _, row := range rows {
		id := toInt64(row["id"])
		roleIDs := uniqueInt64(roleIDsByUser[id])
		roleNames := roleNamesByUser[id]
		if protectedAdminUser(id) && len(roleIDs) == 0 {
			roleIDs = []int64{1}
		}
		if protectedAdminUser(id) && len(roleNames) == 0 {
			roleNames = []string{"Administrator"}
		}
		users = append(users, managedUser{
			ID:            id,
			Username:      toString(row["username"]),
			Name:          toString(row["name"]),
			Avatar:        toString(row["avatar"]),
			Status:        normalizeUserStatus(toString(row["status"])),
			RoleIDs:       roleIDs,
			Roles:         roleNames,
			DepartmentIDs: uniqueInt64(departmentIDsByUser[id]),
			Departments:   departmentNamesByUser[id],
			CreatedAt:     toDateTimeString(row["created_at"]),
			UpdatedAt:     toDateTimeString(row["updated_at"]),
		})
	}
	return users, nil
}

func (s *Store) loadUserDepartments(roleIDsByUser map[int64][]int64) (map[int64][]int64, map[int64][]string, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_department_roles").
		LeftJoin("goadmin_department", "goadmin_department.id", "=", "goadmin_department_roles.department_id").
		Select("goadmin_department_roles.department_id", "goadmin_department_roles.role_id", "goadmin_department.name", "goadmin_department.code").
		All()
	if err != nil {
		return nil, nil, err
	}

	departmentsByRole := make(map[int64][]managedUserDepartment)
	for _, row := range rows {
		departmentID := toInt64(row["department_id"])
		roleID := toInt64(row["role_id"])
		name := toString(row["name"])
		if name == "" {
			name = toString(row["code"])
		}
		if departmentID == 0 || roleID == 0 || name == "" {
			continue
		}
		departmentsByRole[roleID] = append(departmentsByRole[roleID], managedUserDepartment{
			ID:   departmentID,
			Name: name,
		})
	}

	departmentIDsByUser := make(map[int64][]int64)
	departmentNamesByUser := make(map[int64][]string)
	for userID, roleIDs := range roleIDsByUser {
		seenID := make(map[int64]bool)
		seenName := make(map[string]bool)
		for _, roleID := range uniqueInt64(roleIDs) {
			for _, department := range departmentsByRole[roleID] {
				if !seenID[department.ID] {
					departmentIDsByUser[userID] = append(departmentIDsByUser[userID], department.ID)
					seenID[department.ID] = true
				}
				if !seenName[department.Name] {
					departmentNamesByUser[userID] = append(departmentNamesByUser[userID], department.Name)
					seenName[department.Name] = true
				}
			}
		}
	}
	return departmentIDsByUser, departmentNamesByUser, nil
}

func (s *Store) usernameExists(username string, exceptID int64) (bool, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_users").
		Where("username", "=", username).
		All()
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if toInt64(row["id"]) != exceptID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) replaceUserRoles(userID int64, roleIDs []int64) error {
	if protectedAdminUser(userID) {
		return nil
	}
	_ = db.WithDriver(s.conn).Table("goadmin_role_users").Where("user_id", "=", userID).Delete()
	for _, roleID := range uniqueInt64(roleIDs) {
		if roleID == 0 {
			continue
		}
		if err := s.insertRoleUser(roleID, userID); err != nil {
			return err
		}
	}
	return nil
}

func protectedAdminUser(userID int64) bool {
	return userID == 1
}

func normalizeUserPayload(req *userPayload) {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Name = strings.TrimSpace(req.Name)
	req.Avatar = strings.TrimSpace(req.Avatar)
	req.Status = normalizeUserStatus(req.Status)
	req.RoleIDs = uniqueInt64(req.RoleIDs)
}

func normalizeUserStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "enable"
	}
	return status
}

func matchesManagedUserFilter(user managedUser, keyword, departmentKeyword, roleKeyword, status string) bool {
	if keyword != "" &&
		!strings.Contains(strings.ToLower(user.Username), keyword) &&
		!strings.Contains(strings.ToLower(user.Name), keyword) {
		return false
	}
	if departmentKeyword != "" && !containsKeyword(user.Departments, departmentKeyword) {
		return false
	}
	if roleKeyword != "" && !containsKeyword(user.Roles, roleKeyword) {
		return false
	}
	if status != "" && user.Status != status {
		return false
	}
	return true
}

func containsKeyword(items []string, keyword string) bool {
	for _, item := range items {
		if strings.Contains(strings.ToLower(item), keyword) {
			return true
		}
	}
	return false
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	hasLetter := false
	hasDigit := false
	for _, r := range password {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			hasLetter = true
		}
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return fmt.Errorf("password must contain letters and numbers")
	}
	return nil
}

func exportUsersCSV(users []managedUser) (string, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(buf)
	if err := writer.Write([]string{"username", "name", "avatar", "status", "role_ids", "roles", "created_at", "updated_at"}); err != nil {
		return "", err
	}
	for _, user := range users {
		if err := writer.Write([]string{
			user.Username,
			user.Name,
			user.Avatar,
			user.Status,
			joinInt64(user.RoleIDs),
			strings.Join(user.Roles, "|"),
			user.CreatedAt,
			user.UpdatedAt,
		}); err != nil {
			return "", err
		}
	}
	writer.Flush()
	return buf.String(), writer.Error()
}

func exportUsersSQL(users []managedUser) string {
	var builder strings.Builder
	builder.WriteString("-- goadmin_users export\n")
	builder.WriteString("-- password values are intentionally omitted; reset after import if needed.\n")
	for _, user := range users {
		builder.WriteString(fmt.Sprintf(
			"INSERT INTO goadmin_users (username, password, name, avatar, status) VALUES ('%s', '', '%s', '%s', '%s');\n",
			sqlQuote(user.Username),
			sqlQuote(user.Name),
			sqlQuote(user.Avatar),
			sqlQuote(user.Status),
		))
	}
	return builder.String()
}

func exportUsersExcel(users []managedUser) ([]byte, error) {
	file := excelize.NewFile()
	sheet := "Sheet1"
	headers := []string{"username", "password", "name", "avatar", "status", "role_ids", "roles", "created_at", "updated_at"}
	for i, header := range headers {
		file.SetCellValue(sheet, fmt.Sprintf("%s1", excelColumn(i)), header)
	}
	for rowIndex, user := range users {
		row := rowIndex + 2
		values := []string{
			user.Username,
			"",
			user.Name,
			user.Avatar,
			user.Status,
			joinInt64(user.RoleIDs),
			strings.Join(user.Roles, "|"),
			user.CreatedAt,
			user.UpdatedAt,
		}
		for colIndex, value := range values {
			file.SetCellValue(sheet, fmt.Sprintf("%s%d", excelColumn(colIndex), row), value)
		}
	}
	buf, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseUsersCSV(content string) ([]userPayload, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(content, "\xEF\xBB\xBF")))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return parseUserRows(records), nil
}

func parseUsersExcel(content string) ([]userPayload, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
	if err != nil {
		return nil, fmt.Errorf("invalid excel content")
	}
	file, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	rows := file.GetRows("Sheet1")
	if len(rows) == 0 {
		return nil, nil
	}
	return parseUserRows(rows), nil
}

func parseUserRows(records [][]string) []userPayload {
	start := 0
	header := make(map[string]int)
	for i, value := range records[0] {
		header[strings.ToLower(strings.TrimSpace(value))] = i
	}
	if _, ok := header["username"]; ok {
		start = 1
	}

	users := make([]userPayload, 0, len(records)-start)
	for _, record := range records[start:] {
		statusFallback := 3
		roleIDsFallback := 4
		if start == 0 {
			statusFallback = -1
			roleIDsFallback = 3
		}
		valueAt := func(key string, fallback int) string {
			if idx, ok := header[key]; ok && idx < len(record) {
				return record[idx]
			}
			if fallback >= 0 && fallback < len(record) {
				return record[fallback]
			}
			return ""
		}
		users = append(users, userPayload{
			Username: valueAt("username", 0),
			Password: valueAt("password", -1),
			Name:     valueAt("name", 1),
			Avatar:   valueAt("avatar", 2),
			Status:   valueAt("status", statusFallback),
			RoleIDs:  parseRoleIDs(valueAt("role_ids", roleIDsFallback)),
		})
	}
	return users
}

func parseUsersSQL(content string) ([]userPayload, error) {
	users := make([]userPayload, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		valuesIndex := strings.Index(strings.ToUpper(line), "VALUES")
		if valuesIndex < 0 {
			continue
		}
		values := strings.TrimSpace(line[valuesIndex+len("VALUES"):])
		values = strings.TrimSuffix(strings.TrimPrefix(values, "("), ");")
		parts, err := splitSQLValues(values)
		if err != nil {
			return nil, err
		}
		if len(parts) < 4 {
			continue
		}
		users = append(users, userPayload{
			Username: parts[0],
			Password: parts[1],
			Name:     parts[2],
			Avatar:   parts[3],
			Status:   valueOrDefault(parts, 4, "enable"),
		})
	}
	return users, nil
}

func valueOrDefault(values []string, index int, fallback string) string {
	if index >= 0 && index < len(values) {
		value := strings.TrimSpace(values[index])
		if value != "" {
			return value
		}
	}
	return fallback
}

func splitSQLValues(values string) ([]string, error) {
	parts := make([]string, 0)
	var builder strings.Builder
	inQuote := false
	for i := 0; i < len(values); i++ {
		ch := values[i]
		if ch == '\'' {
			if inQuote && i+1 < len(values) && values[i+1] == '\'' {
				builder.WriteByte('\'')
				i++
				continue
			}
			inQuote = !inQuote
			continue
		}
		if ch == ',' && !inQuote {
			parts = append(parts, strings.TrimSpace(builder.String()))
			builder.Reset()
			continue
		}
		builder.WriteByte(ch)
	}
	if inQuote {
		return nil, fmt.Errorf("invalid sql values")
	}
	parts = append(parts, strings.TrimSpace(builder.String()))
	return parts, nil
}

func parseRoleIDs(value string) []int64 {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '|' || r == ';' || r == ' '
	})
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, _ := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if id != 0 {
			ids = append(ids, id)
		}
	}
	return uniqueInt64(ids)
}

func joinInt64(values []int64) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatInt(value, 10))
	}
	return strings.Join(parts, "|")
}

func sqlQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func excelColumn(index int) string {
	col := ""
	index++
	for index > 0 {
		index--
		col = string(rune('A'+index%26)) + col
		index /= 26
	}
	return col
}
