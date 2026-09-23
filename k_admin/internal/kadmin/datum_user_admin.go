package kadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Desktop platform-user management (legacy /datum/user admin contract):
// paginated list, public detail, create/update/delete and the admin security
// answer reset. Reads mirror the file module (anonymous; payloads never carry
// the password hash); writes sit behind requireDatumAuth — the fine-grained
// datum:user:* permission wiring is deferred to the permissions/audit task.
//
// Status semantics are the ptmj_user ones: '1' normal, '0' banned (the
// opposite of RuoYi's sys_user).

// ---------------------------------------------------------------------------
// reads: list / detail (dispatched from datum_user_auth.go)
// ---------------------------------------------------------------------------

// datumUserAdminPayload is the RuoYi-facing projection shared by list and
// detail. The desktop reads userId/userName/avatar/count/remark/createTime
// off it (rank view, FileInfoDrawer, MainEditor).
func datumUserAdminPayload(user *datumUser) gin.H {
	return gin.H{
		"userId":     user.UserID,
		"userName":   user.UserName,
		"nickName":   user.UserName,
		"avatar":     user.Avatar,
		"count":      user.Count,
		"status":     user.Status,
		"createTime": user.CreateTime,
		"updateTime": user.UpdateTime,
		"remark":     user.Remark,
	}
}

func (s *Store) datumUserList(c *gin.Context) {
	page, size := datumPageParams(c)
	filter := datumUserListFilter{
		Page:     page,
		PageSize: size,
		UserName: strings.TrimSpace(c.Query("userName")),
		Status:   normalizeDatumUserStatus(strings.TrimSpace(c.Query("status"))),
	}
	users, total, err := (&datumUserRepo{conn: s.conn}).listAdmin(filter)
	if err != nil {
		fail(c, http.StatusInternalServerError, "用户列表查询失败")
		return
	}
	rows := make([]gin.H, 0, len(users))
	for index := range users {
		rows = append(rows, datumUserAdminPayload(&users[index]))
	}
	// RuoYi TableDataInfo 形状：rows/total 在顶层（桌面端直接读 response.rows/total）
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"msg":     "ok",
		"rows":    rows,
		"total":   total,
	})
}

func (s *Store) datumUserDetail(c *gin.Context) {
	userID, ok := datumPathInt64(c, "userId")
	if !ok {
		return
	}
	user, err := (&datumUserRepo{conn: s.conn}).findByUserID(userID)
	if err != nil {
		if errors.Is(err, errDatumUserNotFound) {
			fail(c, http.StatusNotFound, "用户不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "用户查询失败")
		return
	}
	success(c, datumUserAdminPayload(user))
}

// ---------------------------------------------------------------------------
// writes: create / update / delete
// ---------------------------------------------------------------------------

// normalizeDatumUserStatus maps the desktop's string/number status forms onto
// the ptmj_user char(1) domain; anything else yields "" (rejected).
func normalizeDatumUserStatus(raw string) string {
	switch raw {
	case "1", "1.0":
		return "1"
	case "0", "0.0":
		return "0"
	default:
		return ""
	}
}

// datumFlexStatus accepts a JSON string, number or null — the desktop round-
// trips status both as the char-column string ('1') and as a number (1).
type datumFlexStatus string

func (v *datumFlexStatus) UnmarshalJSON(raw []byte) error {
	text := strings.Trim(string(raw), `"`)
	if text == "null" {
		text = ""
	}
	*v = datumFlexStatus(text)
	return nil
}

func (v datumFlexStatus) text() string { return string(v) }

type datumUserCreateRequest struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
	Avatar   string `json:"avatar"`
	Count    int64  `json:"count"`
	Status   string `json:"status"`
	Remark   string `json:"remark"`
}

func (s *Store) datumUserCreate(c *gin.Context) {
	var req datumUserCreateRequest
	_ = c.ShouldBind(&req)
	req.UserName = strings.TrimSpace(req.UserName)
	status := "1"
	if strings.TrimSpace(req.Status) != "" {
		status = normalizeDatumUserStatus(strings.TrimSpace(req.Status))
		if status == "" {
			fail(c, http.StatusBadRequest, "状态格式错误：仅支持 0（封禁）或 1（正常）")
			return
		}
	}
	switch {
	case req.UserName == "" || len([]rune(req.UserName)) > 64:
		fail(c, http.StatusBadRequest, "用户名格式错误：需为 1-64 个字符")
		return
	case len(req.Password) < 5 || len(req.Password) > 64:
		fail(c, http.StatusBadRequest, "密码格式错误：长度需在 5 到 64 位之间")
		return
	case len(req.Avatar) > 255:
		fail(c, http.StatusBadRequest, "头像地址不能超过 255 个字符")
		return
	case len([]rune(req.Remark)) > 500:
		fail(c, http.StatusBadRequest, "备注不能超过 500 个字符")
		return
	}
	repo := &datumUserRepo{conn: s.conn}
	if existing, err := repo.findByUserName(req.UserName); err == nil && existing != nil {
		fail(c, http.StatusConflict, "用户名已存在")
		return
	} else if err != nil && !errors.Is(err, errDatumUserNotFound) {
		fail(c, http.StatusInternalServerError, "用户创建失败")
		return
	}
	passwordHash, err := hashDatumSecret(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "用户创建失败")
		return
	}
	userID, err := repo.createAdmin(req.UserName, passwordHash, strings.TrimSpace(req.Avatar), status, req.Count, strings.TrimSpace(req.Remark))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "用户名已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "用户创建失败")
		return
	}
	success(c, gin.H{"userId": userID, "userName": req.UserName, "status": status})
}

// datumUserUpdateRequest merges partial edits: unprovided fields keep their
// stored values. Ban/unban rides the same endpoint as status '0'/'1'.
type datumUserUpdateRequest struct {
	UserID   datumFlexID     `json:"userId"`
	UserName *string         `json:"userName"`
	Password *string         `json:"password"`
	Avatar   *string         `json:"avatar"`
	Count    *int64          `json:"count"`
	Status   *datumFlexStatus `json:"status"`
	Remark   *string         `json:"remark"`
}

func (s *Store) datumUserUpdate(c *gin.Context) {
	actorID, _ := datumUserIDFrom(c)
	var req datumUserUpdateRequest
	_ = c.ShouldBind(&req)
	userID := req.UserID.value()
	if userID <= 0 {
		fail(c, http.StatusBadRequest, "用户 ID 不能为空")
		return
	}
	repo := &datumUserRepo{conn: s.conn}
	if _, err := repo.findByUserID(userID); err != nil {
		if errors.Is(err, errDatumUserNotFound) {
			fail(c, http.StatusNotFound, "用户不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "用户查询失败")
		return
	}
	update := datumUserAdminUpdate{}
	if req.UserName != nil {
		userName := strings.TrimSpace(*req.UserName)
		if userName == "" || len([]rune(userName)) > 64 {
			fail(c, http.StatusBadRequest, "用户名格式错误：需为 1-64 个字符")
			return
		}
		if clash, err := repo.findByUserName(userName); err == nil && clash != nil && clash.UserID != userID {
			fail(c, http.StatusConflict, "用户名已存在")
			return
		} else if err != nil && !errors.Is(err, errDatumUserNotFound) {
			fail(c, http.StatusInternalServerError, "用户更新失败")
			return
		}
		update.UserName = &userName
	}
	if req.Password != nil && *req.Password != "" {
		// 空字符串视为未提供，避免管理端整行回显时误清密码
		if len(*req.Password) < 5 || len(*req.Password) > 64 {
			fail(c, http.StatusBadRequest, "密码格式错误：长度需在 5 到 64 位之间")
			return
		}
		passwordHash, err := hashDatumSecret(*req.Password)
		if err != nil {
			fail(c, http.StatusInternalServerError, "用户更新失败")
			return
		}
		update.PasswordHash = &passwordHash
	}
	if req.Avatar != nil {
		avatar := strings.TrimSpace(*req.Avatar)
		if len(avatar) > 255 {
			fail(c, http.StatusBadRequest, "头像地址不能超过 255 个字符")
			return
		}
		update.Avatar = &avatar
	}
	if req.Count != nil {
		if *req.Count < 0 {
			fail(c, http.StatusBadRequest, "上传数不能为负数")
			return
		}
		update.Count = req.Count
	}
	if req.Status != nil && strings.TrimSpace(req.Status.text()) != "" {
		status := normalizeDatumUserStatus(strings.TrimSpace(req.Status.text()))
		if status == "" {
			fail(c, http.StatusBadRequest, "状态格式错误：仅支持 0（封禁）或 1（正常）")
			return
		}
		update.Status = &status
	}
	if req.Remark != nil {
		if len([]rune(*req.Remark)) > 500 {
			fail(c, http.StatusBadRequest, "备注不能超过 500 个字符")
			return
		}
		remark := strings.TrimSpace(*req.Remark)
		update.Remark = &remark
	}
	// 操作人留痕：会话即普通 ptmj 用户，查不到就留空
	if actor, err := repo.findByUserID(actorID); err == nil {
		update.UpdateBy = actor.UserName
	}
	if _, err := repo.updateAdmin(userID, update); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "用户名已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "用户更新失败")
		return
	}
	// 封禁/解封与上传数都会改变排行榜成员或名次，失效排行哈希缓存（同上传链路）
	if update.Status != nil || update.Count != nil {
		s.invalidateDatumRank()
	}
	success(c, true)
}

func (s *Store) datumUserDelete(c *gin.Context) {
	actorID, _ := datumUserIDFrom(c)
	userID, ok := datumPathInt64(c, "userId")
	if !ok {
		return
	}
	if userID == actorID {
		fail(c, http.StatusBadRequest, "不能删除当前登录账号")
		return
	}
	deleted, err := (&datumUserRepo{conn: s.conn}).deleteByID(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "用户删除失败")
		return
	}
	if !deleted {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	// ptmj_user 无软删除列，硬删后用户直接退出排行榜候选
	s.invalidateDatumRank()
	success(c, true)
}

// ---------------------------------------------------------------------------
// admin security answer reset
// ---------------------------------------------------------------------------

type datumAdminSecurityResetRequest struct {
	UserName              string `json:"userName"`
	SecurityQuestionOne   string `json:"securityQuestionOne"`
	SecurityAnswerOne     string `json:"securityAnswerOne"`
	SecurityQuestionTwo   string `json:"securityQuestionTwo"`
	SecurityAnswerTwo     string `json:"securityAnswerTwo"`
	SecurityQuestionThree string `json:"securityQuestionThree"`
	SecurityAnswerThree   string `json:"securityAnswerThree"`
}

// datumAdminResetSecurityAnswers resets a named user's security Q&A. With a
// full 3-question set the row is replaced (answers re-hashed server side);
// with everything blank the row is cleared so the user re-arms it from the
// desktop profile page. Partial sets are rejected.
func (s *Store) datumAdminResetSecurityAnswers(c *gin.Context) {
	var req datumAdminSecurityResetRequest
	_ = c.ShouldBind(&req)
	req.UserName = strings.TrimSpace(req.UserName)
	if req.UserName == "" {
		fail(c, http.StatusBadRequest, "用户名不能为空")
		return
	}
	repo := &datumUserRepo{conn: s.conn}
	user, err := repo.findByUserName(req.UserName)
	if err != nil {
		if errors.Is(err, errDatumUserNotFound) {
			fail(c, http.StatusNotFound, "用户不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "密保重置失败")
		return
	}
	questions := []string{
		strings.TrimSpace(req.SecurityQuestionOne),
		strings.TrimSpace(req.SecurityQuestionTwo),
		strings.TrimSpace(req.SecurityQuestionThree),
	}
	answers := []string{
		strings.TrimSpace(req.SecurityAnswerOne),
		strings.TrimSpace(req.SecurityAnswerTwo),
		strings.TrimSpace(req.SecurityAnswerThree),
	}
	provided := 0
	for index := 0; index < 3; index++ {
		if questions[index] != "" {
			provided++
		}
		if answers[index] != "" {
			provided++
		}
	}
	switch provided {
	case 0:
		if err := repo.clearSecurity(user.UserID); err != nil {
			fail(c, http.StatusInternalServerError, "密保重置失败")
			return
		}
	case 6:
		hashes := make([]string, 0, 3)
		for _, answer := range answers {
			hash, hashErr := hashDatumSecret(answer)
			if hashErr != nil {
				fail(c, http.StatusInternalServerError, "密保重置失败")
				return
			}
			hashes = append(hashes, hash)
		}
		if err := repo.updateSecurity(user.UserID, strings.Join(questions, "|"), strings.Join(hashes, "|")); err != nil {
			fail(c, http.StatusInternalServerError, "密保重置失败")
			return
		}
	default:
		fail(c, http.StatusBadRequest, "密保问题与答案需三组成套提供，或全部留空以清除")
		return
	}
	success(c, gin.H{"userId": user.UserID, "userName": user.UserName, "cleared": provided == 0})
}

// ---------------------------------------------------------------------------
// repo: admin queries over ptmj_user
// ---------------------------------------------------------------------------

type datumUserListFilter struct {
	Page     int
	PageSize int
	Status   string
	UserName string
}

func (r *datumUserRepo) listAdmin(filter datumUserListFilter) ([]datumUser, int64, error) {
	where := " WHERE 1=1"
	args := make([]interface{}, 0, 2)
	if filter.UserName != "" {
		where += " AND user_name ILIKE ?"
		args = append(args, "%"+filter.UserName+"%")
	}
	if filter.Status != "" {
		where += " AND status = ?"
		args = append(args, filter.Status)
	}
	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_user`+where, args...)
	if err != nil {
		return nil, 0, err
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = toDatumInt64(countRows[0]["count"])
	}
	queryArgs := append(append([]interface{}{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.conn.Query(`SELECT `+datumUserColumns+` FROM ptmj_user`+where+` ORDER BY user_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	users := make([]datumUser, 0, len(rows))
	for _, row := range rows {
		users = append(users, *r.scanUser(row))
	}
	return users, total, nil
}

func (r *datumUserRepo) createAdmin(userName, passwordHash, avatar, status string, count int64, remark string) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO ptmj_user (user_name, password, avatar, count, status, remark, create_time, update_time)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING user_id`,
		userName, passwordHash, avatar, count, status, remark)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, errors.New("ptmj_user insert returned no id")
	}
	return toDatumInt64(rows[0]["user_id"]), nil
}

// datumUserAdminUpdate carries pointer fields so only provided columns enter
// the SET clause; update_by always rides along as the fake-DB discriminator.
type datumUserAdminUpdate struct {
	UserName     *string
	PasswordHash *string
	Avatar       *string
	Count        *int64
	Status       *string
	Remark       *string
	UpdateBy     string
}

func (r *datumUserRepo) updateAdmin(userID int64, update datumUserAdminUpdate) (bool, error) {
	sets := make([]string, 0, 7)
	args := make([]interface{}, 0, 7)
	if update.UserName != nil {
		sets = append(sets, "user_name = ?")
		args = append(args, *update.UserName)
	}
	if update.PasswordHash != nil {
		sets = append(sets, "password = ?")
		args = append(args, *update.PasswordHash)
	}
	if update.Avatar != nil {
		sets = append(sets, "avatar = ?")
		args = append(args, *update.Avatar)
	}
	if update.Count != nil {
		sets = append(sets, "count = ?")
		args = append(args, *update.Count)
	}
	if update.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *update.Status)
	}
	if update.Remark != nil {
		sets = append(sets, "remark = ?")
		args = append(args, *update.Remark)
	}
	if len(sets) == 0 {
		return true, nil
	}
	sets = append(sets, "update_by = ?", "update_time = CURRENT_TIMESTAMP")
	args = append(args, update.UpdateBy, userID)
	result, err := r.conn.Exec(`UPDATE ptmj_user SET `+strings.Join(sets, ", ")+` WHERE user_id = ?`, args...)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	return affected > 0, nil
}

// deleteByID hard-deletes the account (ptmj_user has no del_flag) plus its
// security row. Active datum sessions are not enumerated: the next login is
// refused on the missing row and getInfo degrades to 账号不存在.
func (r *datumUserRepo) deleteByID(userID int64) (bool, error) {
	result, err := r.conn.Exec(`DELETE FROM ptmj_user WHERE user_id = ?`, userID)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return false, nil
	}
	_, _ = r.conn.Exec(`DELETE FROM ptmj_security WHERE user_id = ?`, userID)
	return true, nil
}

func (r *datumUserRepo) clearSecurity(userID int64) error {
	_, err := r.conn.Exec(`DELETE FROM ptmj_security WHERE user_id = ?`, userID)
	return err
}
