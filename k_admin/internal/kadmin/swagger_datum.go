package kadmin

// This file carries the swag annotations for the datum (PezMax desktop) API
// surface, mirroring swagger_operations.go for the /api side. The functions
// are documentation anchors consumed by swag and are not handlers.
//
// Envelopes: JSON responses use the native KAdmin envelope (SwaggerResponse,
// errors via SwaggerErrorResponse with real HTTP status codes). The legacy
// list endpoints consumed by the desktop (file list, user list, bookmark
// list, report lists) keep the RuoYi TableDataInfo shape — rows/total at the
// top level — and are annotated with SwaggerTableDataResponse.

// ---------------------------------------------------------------------------
// 桌面端认证
// ---------------------------------------------------------------------------

// swaggerDatumCaptchaImage documents GET /datum/user/captchaImage.
// @Summary 获取桌面端登录验证码
// @Tags 桌面端认证
// @Success 200 {object} SwaggerResponse "captchaEnabled/uuid/img（img 为裸 base64 JPEG）"
// @Failure 503 {object} SwaggerErrorResponse
// @Router /datum/user/captchaImage [get]
func swaggerDatumCaptchaImage() {}

// swaggerDatumLogin documents POST /datum/user/login.
// @Summary 桌面端登录
// @Tags 桌面端认证
// @Param payload body datumLoginRequest true "登录信息"
// @Success 200 {object} SwaggerResponse "data.token 为会话令牌"
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 429 {object} SwaggerErrorResponse
// @Router /datum/user/login [post]
func swaggerDatumLogin() {}

// swaggerDatumRegister documents POST /datum/user/register.
// @Summary 桌面端注册
// @Tags 桌面端认证
// @Param payload body datumLoginRequest true "注册信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /datum/user/register [post]
func swaggerDatumRegister() {}

// swaggerDatumGetInfo documents GET /datum/user/getInfo.
// @Summary 获取当前桌面端用户信息
// @Tags 桌面端认证
// @Success 200 {object} SwaggerResponse "data.user/roles/permissions"
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /datum/user/getInfo [get]
func swaggerDatumGetInfo() {}

// swaggerDatumLogout documents POST /datum/user/logout.
// @Summary 桌面端退出登录
// @Tags 桌面端认证
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 503 {object} SwaggerErrorResponse
// @Router /datum/user/logout [post]
func swaggerDatumLogout() {}

// swaggerDatumSecurityQuestions documents GET /datum/user/securityQuestions.
// @Summary 找回密码第一步：校验账号与验证码并返回密保问题
// @Tags 桌面端认证
// @Param userName query string true "用户名"
// @Param uuid query string true "验证码 uuid"
// @Param code query string true "验证码"
// @Success 200 {object} SwaggerResponse "重置工单与密保问题"
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/user/securityQuestions [get]
func swaggerDatumSecurityQuestions() {}

// swaggerDatumResetPasswordBySecurity documents POST /datum/user/resetPasswordBySecurity.
// @Summary 找回密码第二步：验重置工单与密保答案后重置密码
// @Tags 桌面端认证
// @Param payload body object true "工单、三问三答与新密码"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /datum/user/resetPasswordBySecurity [post]
func swaggerDatumResetPasswordBySecurity() {}

// ---------------------------------------------------------------------------
// 桌面端平台用户
// ---------------------------------------------------------------------------

// swaggerDatumUserList documents GET /datum/user/list.
// @Summary 平台用户列表（匿名）
// @Tags 桌面端用户
// @Param userName query string false "用户名模糊"
// @Param status query string false "状态：1 正常 / 0 封禁"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerTableDataResponse "RuoYi TableDataInfo 形状，载荷不含密码哈希"
// @Failure 500 {object} SwaggerErrorResponse
// @Router /datum/user/list [get]
func swaggerDatumUserList() {}

// swaggerDatumUserDetail documents GET /datum/user/{userId}.
// @Summary 平台用户公开详情（匿名）
// @Tags 桌面端用户
// @Param userId path int true "用户 ID"
// @Success 200 {object} SwaggerResponse "公开投影，载荷不含密码哈希"
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/user/{userId} [get]
func swaggerDatumUserDetail() {}

// swaggerDatumUserRank documents GET /datum/user/rank.
// @Summary 上传排行榜（匿名，哈希缓存协议）
// @Tags 桌面端用户
// @Param hash query string false "客户端缓存哈希，未变化返回 unchanged:true"
// @Success 200 {object} SwaggerResponse
// @Failure 500 {object} SwaggerErrorResponse
// @Router /datum/user/rank [get]
func swaggerDatumUserRank() {}

// swaggerDatumUserCreate documents POST /datum/user.
// @Summary 新建平台用户
// @Tags 桌面端用户
// @Security BearerAuth
// @Param payload body datumUserCreateRequest true "用户信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:user:manage 权限"
// @Failure 409 {object} SwaggerErrorResponse
// @Router /datum/user [post]
func swaggerDatumUserCreate() {}

// swaggerDatumUserUpdate documents PUT /datum/user.
// @Summary 编辑平台用户（局部合并，封禁/解封走 status）
// @Tags 桌面端用户
// @Security BearerAuth
// @Param payload body datumUserUpdateRequest true "编辑字段"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:user:manage 权限"
// @Failure 404 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /datum/user [put]
func swaggerDatumUserUpdate() {}

// swaggerDatumUserDelete documents DELETE /datum/user/{userId}.
// @Summary 删除平台用户（硬删并连带清理密保）
// @Tags 桌面端用户
// @Security BearerAuth
// @Param userId path int true "用户 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:user:manage 权限"
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/user/{userId} [delete]
func swaggerDatumUserDelete() {}

// swaggerDatumAdminResetSecurityAnswers documents POST /datum/user/resetSecurityAnswers.
// @Summary 按用户名重置密保三问三答（成套替换或全空清除）
// @Tags 桌面端用户
// @Security BearerAuth
// @Param payload body datumAdminSecurityResetRequest true "密保三问三答"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:user:manage 权限"
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/user/resetSecurityAnswers [post]
func swaggerDatumAdminResetSecurityAnswers() {}

// swaggerDatumRankCachePurge documents DELETE /datum/user/rank/cache.
// @Summary 清理排行榜哈希缓存
// @Tags 桌面端用户
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:user:manage 权限"
// @Router /datum/user/rank/cache [delete]
func swaggerDatumRankCachePurge() {}

// ---------------------------------------------------------------------------
// 桌面端个人中心（/datum/desktop/user）
// ---------------------------------------------------------------------------

// swaggerDatumProfile documents GET /datum/desktop/user/profile.
// @Summary 获取个人资料
// @Tags 桌面端用户
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile [get]
func swaggerDatumProfile() {}

// swaggerDatumProfileStats documents GET /datum/desktop/user/profile/stats.
// @Summary 获取个人统计（上传/下载/收藏计数）
// @Tags 桌面端用户
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/stats [get]
func swaggerDatumProfileStats() {}

// swaggerDatumUpdateUserName documents PUT /datum/desktop/user/profile/username.
// @Summary 修改用户名
// @Tags 桌面端用户
// @Param payload body object true "新用户名"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/username [put]
func swaggerDatumUpdateUserName() {}

// swaggerDatumUpdateAvatar documents PUT /datum/desktop/user/profile/avatar.
// @Summary 修改头像地址
// @Tags 桌面端用户
// @Param payload body object true "头像 URL"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/avatar [put]
func swaggerDatumUpdateAvatar() {}

// swaggerDatumUploadAvatar documents POST /datum/desktop/user/profile/avatar/upload.
// @Summary 上传头像（multipart）
// @Tags 桌面端用户
// @Param file formData file true "头像文件"
// @Success 200 {object} SwaggerResponse "头像 URL"
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/avatar/upload [post]
func swaggerDatumUploadAvatar() {}

// swaggerDatumVerifyPassword documents POST /datum/desktop/user/profile/password/verify.
// @Summary 校验登录密码（敏感操作前置确认）
// @Tags 桌面端用户
// @Param payload body object true "当前密码"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/password/verify [post]
func swaggerDatumVerifyPassword() {}

// swaggerDatumUpdatePassword documents PUT /datum/desktop/user/profile/password.
// @Summary 修改登录密码
// @Tags 桌面端用户
// @Param payload body object true "旧密码与新密码"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/password [put]
func swaggerDatumUpdatePassword() {}

// swaggerDatumResetPasswordBySecurityProfile documents PUT /datum/desktop/user/profile/password/by-security.
// @Summary 通过密保答案重置密码
// @Tags 桌面端用户
// @Param payload body object true "三问三答与新密码"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/password/by-security [put]
func swaggerDatumResetPasswordBySecurityProfile() {}

// swaggerDatumGetSecurity documents GET /datum/desktop/user/profile/security.
// @Summary 获取密保问题（不含答案）
// @Tags 桌面端用户
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/security [get]
func swaggerDatumGetSecurity() {}

// swaggerDatumUpdateSecurity documents PUT /datum/desktop/user/profile/security.
// @Summary 设置/更换密保三问三答（需验证当前答案或密码）
// @Tags 桌面端用户
// @Param payload body object true "三问三答"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/security [put]
func swaggerDatumUpdateSecurity() {}

// swaggerDatumVerifySecurityAnswer documents POST /datum/desktop/user/profile/security/answer/verify.
// @Summary 校验密保答案
// @Tags 桌面端用户
// @Param payload body object true "问题与答案"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/user/profile/security/answer/verify [post]
func swaggerDatumVerifySecurityAnswer() {}

// ---------------------------------------------------------------------------
// 桌面端文件（试卷资料）
// ---------------------------------------------------------------------------

// swaggerDatumFileList documents GET /datum/file/list.
// @Summary 试卷文件列表（匿名）
// @Tags 桌面端文件
// @Param keyword query string false "文件名关键词"
// @Param fileType query string false "文件类型"
// @Param subject query string false "科目"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerTableDataResponse "RuoYi TableDataInfo 形状"
// @Failure 500 {object} SwaggerErrorResponse
// @Router /datum/file/list [get]
func swaggerDatumFileList() {}

// ---------------------------------------------------------------------------
// 桌面端电子书（ptmj_ebook 目录）
// ---------------------------------------------------------------------------

// swaggerDatumEbookList documents GET /datum/ebook/list.
// @Summary 电子书列表（匿名，仅已上架）
// @Tags 桌面端电子书
// @Param keyword query string false "书名/作者/出版社关键词"
// @Param subject query string false "学科分类"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerTableDataResponse "RuoYi TableDataInfo 形状"
// @Failure 500 {object} SwaggerErrorResponse
// @Router /datum/ebook/list [get]
func swaggerDatumEbookList() {}

// swaggerDatumEbookContent documents GET /datum/ebook/content.
// @Summary 电子书在线预览流（inline，不落下载记录）
// @Tags 桌面端电子书
// @Param ebookId query int true "电子书 ID"
// @Success 200 {string} string "电子书内容流（pdf/epub 等）"
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/ebook/content [get]
func swaggerDatumEbookContent() {}

// swaggerDatumEbookSubjects documents GET /datum/ebook/subjects.
// @Summary 电子书学科列表（匿名，仅已上架）
// @Tags 桌面端电子书
// @Success 200 {array} SwaggerResponse "学科与计数"
// @Failure 500 {object} SwaggerErrorResponse
// @Router /datum/ebook/subjects [get]
func swaggerDatumEbookSubjects() {}

// swaggerDatumEbookFavoriteAdd documents POST /datum/ebook/favorite.
// @Summary 收藏电子书（属主）
// @Tags 桌面端电子书
// @Param payload body object true "收藏信息 {ebookId, userId}"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /datum/ebook/favorite [post]
func swaggerDatumEbookFavoriteAdd() {}

// swaggerDatumEbookFavoriteStatus documents GET /datum/ebook/favorite.
// @Summary 查询电子书收藏状态（匿名返回未收藏）
// @Tags 桌面端电子书
// @Param ebookId query int true "电子书 ID"
// @Success 200 {object} SwaggerResponse
// @Router /datum/ebook/favorite [get]
func swaggerDatumEbookFavoriteStatus() {}

// swaggerDatumEbookFavoriteRemove documents DELETE /datum/ebook/favorite/{ebookId}.
// @Summary 取消收藏电子书（属主）
// @Tags 桌面端电子书
// @Param ebookId path int true "电子书 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/ebook/favorite/{ebookId} [delete]
func swaggerDatumEbookFavoriteRemove() {}

// swaggerDatumEbookReportCreate documents POST /datum/ebook/report.
// @Summary 举报电子书（属主；一用户一书一条待审举报）
// @Tags 桌面端电子书
// @Param payload body object true "举报信息 {ebookId, userId, reason, remark}"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /datum/ebook/report [post]
func swaggerDatumEbookReportCreate() {}

// swaggerDatumEbookUpload documents POST /datum/ebook/upload.
// @Summary 上传电子书（属主；落库待审并计入排行榜计数器）
// @Tags 桌面端电子书
// @Accept multipart/form-data
// @Param file formData file true "电子书文件（pdf/epub/mobi/azw3）"
// @Param ebookName formData string false "书名（默认取文件名）"
// @Param author formData string false "作者"
// @Param publisher formData string false "出版社"
// @Param ebookSubject formData string true "学科分类"
// @Param ebookType formData int true "类型：1-教材，2-教辅/参考书，3-课外读物，4-其他"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/ebook/upload [post]
func swaggerDatumEbookUpload() {}

// swaggerDatumEbookDelete documents DELETE /datum/ebook/{ebookId}.
// @Summary 删除自己的电子书（软删，回退排行榜计数器）
// @Tags 桌面端电子书
// @Param ebookId path int true "电子书 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/ebook/{ebookId} [delete]
func swaggerDatumEbookDelete() {}

// swaggerDatumEbookReportList documents GET /datum/ebook/report/list.
// @Summary 我的电子书举报列表（与 /datum/report/list 同契约）
// @Tags 桌面端电子书
// @Param userId query int false "用户 ID（须为会话用户）"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerTableDataResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /datum/ebook/report/list [get]
func swaggerDatumEbookReportList() {}

// swaggerDatumMyEbookFavorites documents GET /datum/desktop/ebook/favorite/list/{userId}.
// @Summary 我的电子书收藏列表（联查书目详情）
// @Tags 桌面端活动
// @Param userId path int true "用户 ID"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerTableDataResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /datum/desktop/ebook/favorite/list/{userId} [get]
func swaggerDatumMyEbookFavorites() {}

// swaggerDatumMyEbookFavoriteRemove documents DELETE /datum/desktop/ebook/favorite/{userId}/{ebookId}.
// @Summary 取消收藏电子书（属主）
// @Tags 桌面端活动
// @Param userId path int true "用户 ID"
// @Param ebookId path int true "电子书 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/desktop/ebook/favorite/{userId}/{ebookId} [delete]
func swaggerDatumMyEbookFavoriteRemove() {}

// swaggerDatumFileTree documents GET /datum/file/tree.
// @Summary 试卷文件树（匿名，哈希缓存协议）
// @Tags 桌面端文件
// @Param hash query string false "客户端缓存哈希，未变化返回 unchanged:true"
// @Success 200 {object} SwaggerResponse "类型→科目→年份三层 folder 节点 + file 叶子"
// @Failure 500 {object} SwaggerErrorResponse
// @Router /datum/file/tree [get]
func swaggerDatumFileTree() {}

// swaggerDatumFileSubjects documents GET /datum/file/subjects.
// @Summary 科目列表（匿名）
// @Tags 桌面端文件
// @Success 200 {object} SwaggerResponse
// @Router /datum/file/subjects [get]
func swaggerDatumFileSubjects() {}

// swaggerDatumFileSchools documents GET /datum/file/schools.
// @Summary 学校列表（匿名）
// @Tags 桌面端文件
// @Success 200 {object} SwaggerResponse
// @Router /datum/file/schools [get]
func swaggerDatumFileSchools() {}

// swaggerDatumFileSchoolsCheck documents GET /datum/file/schools/check.
// @Summary 校验学校是否在册（匿名）
// @Tags 桌面端文件
// @Param name query string true "学校名"
// @Success 200 {object} SwaggerResponse
// @Router /datum/file/schools/check [get]
func swaggerDatumFileSchoolsCheck() {}

// swaggerDatumFileSearch documents GET /datum/file/search.
// @Summary 试卷搜索（匿名）
// @Tags 桌面端文件
// @Param keyword query string true "关键词"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Router /datum/file/search [get]
func swaggerDatumFileSearch() {}

// swaggerDatumFileDetail documents GET /datum/file/{fileId}.
// @Summary 试卷详情（匿名仅出已审核，属主凭会话可见待审件）
// @Tags 桌面端文件
// @Param fileId path int true "文件 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/file/{fileId} [get]
func swaggerDatumFileDetail() {}

// swaggerDatumFileUpload documents POST /datum/file.
// @Summary 上传试卷（multipart，属主）
// @Tags 桌面端文件
// @Param file formData file true "试卷文件（doc/docx/pdf/jpg/png/webp/txt/md/ppt/pptx，≤200MB）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 413 {object} SwaggerErrorResponse
// @Router /datum/file [post]
func swaggerDatumFileUpload() {}

// swaggerDatumFileUpdate documents PUT /datum/file.
// @Summary 编辑试卷元信息（属主，局部合并）
// @Tags 桌面端文件
// @Param payload body object true "编辑字段"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/file [put]
func swaggerDatumFileUpdate() {}

// swaggerDatumFileDelete documents DELETE /datum/file/{fileId}.
// @Summary 删除试卷（属主）
// @Tags 桌面端文件
// @Param fileId path int true "文件 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/file/{fileId} [delete]
func swaggerDatumFileDelete() {}

// swaggerDatumTreeCachePurge documents DELETE /datum/file-tree/cache.
// @Summary 强制刷新文件树哈希缓存
// @Tags 桌面端文件
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:file:manage 权限"
// @Router /datum/file-tree/cache [delete]
func swaggerDatumTreeCachePurge() {}

// ---------------------------------------------------------------------------
// 桌面端书签
// ---------------------------------------------------------------------------

// swaggerDatumBookmarkList documents GET /datum/bookmark/list.
// @Summary 书签列表（匿名仅出已审核，带 userId 出该用户全部状态）
// @Tags 桌面端书签
// @Param userId query int false "用户 ID"
// @Param title query string false "标题模糊"
// @Param keyword query string false "关键词模糊"
// @Param url query string false "URL 精确"
// @Param resourceType query string false "资源类型"
// @Param collection query string false "专辑"
// @Param subject query string false "科目"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerTableDataResponse "RuoYi TableDataInfo 形状"
// @Failure 500 {object} SwaggerErrorResponse
// @Router /datum/bookmark/list [get]
func swaggerDatumBookmarkList() {}

// swaggerDatumBookmarkDetail documents GET /datum/bookmark/{bookmarkId}.
// @Summary 书签详情（待审仅属主可见）
// @Tags 桌面端书签
// @Param bookmarkId path int true "书签 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/bookmark/{bookmarkId} [get]
func swaggerDatumBookmarkDetail() {}

// swaggerDatumBookmarkFavoriteList documents GET /datum/bookmark/favorite/list.
// @Summary 我的书签收藏列表
// @Tags 桌面端书签
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/bookmark/favorite/list [get]
func swaggerDatumBookmarkFavoriteList() {}

// swaggerDatumBookmarkCreate documents POST /datum/bookmark.
// @Summary 新增书签（属主）
// @Tags 桌面端书签
// @Param payload body object true "书签信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/bookmark [post]
func swaggerDatumBookmarkCreate() {}

// swaggerDatumBookmarkUpdate documents PUT /datum/bookmark.
// @Summary 编辑书签（属主，部分字段合并，兼容字符串/数字 id）
// @Tags 桌面端书签
// @Param payload body object true "编辑字段"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/bookmark [put]
func swaggerDatumBookmarkUpdate() {}

// swaggerDatumBookmarkDelete documents DELETE /datum/bookmark/{bookmarkId}.
// @Summary 删除书签（属主）
// @Tags 桌面端书签
// @Param bookmarkId path int true "书签 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/bookmark/{bookmarkId} [delete]
func swaggerDatumBookmarkDelete() {}

// swaggerDatumBookmarkUploadCover documents POST /datum/bookmark/uploadCover.
// @Summary 上传书签封面（multipart ≤5MB，先验属主）
// @Tags 桌面端书签
// @Param file formData file true "封面文件（JPG/PNG/GIF/WEBP）"
// @Success 200 {object} SwaggerResponse "{bookmarkId,url}"
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /datum/bookmark/uploadCover [post]
func swaggerDatumBookmarkUploadCover() {}

// ---------------------------------------------------------------------------
// 桌面端活动（下载/收藏）
// ---------------------------------------------------------------------------

// swaggerDatumDownloadDetail documents GET /datum/download/{downloadId}.
// @Summary 下载记录详情（属主）
// @Tags 桌面端活动
// @Param downloadId path int true "下载记录 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/download/{downloadId} [get]
func swaggerDatumDownloadDetail() {}

// swaggerDatumDownloadStream documents GET /datum/download/file.
// @Summary 下载文件流（属主）
// @Tags 桌面端活动
// @Success 200 {string} string "文件流"
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/download/file [get]
func swaggerDatumDownloadStream() {}

// swaggerDatumEbookDownloadStream documents GET /datum/download/ebook.
// @Summary 下载电子书流（属主；落 ptmj_ebook_download 记录）
// @Tags 桌面端电子书
// @Param ebookId query int true "电子书 ID"
// @Success 200 {string} string "电子书文件流（attachment）"
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/download/ebook [get]
func swaggerDatumEbookDownloadStream() {}

// swaggerDatumDownloadCreate documents POST /datum/download.
// @Summary 新增下载记录（属主）
// @Tags 桌面端活动
// @Param payload body object true "下载信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/download [post]
func swaggerDatumDownloadCreate() {}

// swaggerDatumDownloadUpdate documents PUT /datum/download.
// @Summary 编辑下载记录（属主）
// @Tags 桌面端活动
// @Param payload body object true "编辑字段"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/download [put]
func swaggerDatumDownloadUpdate() {}

// swaggerDatumDownloadDelete documents DELETE /datum/download/{ids}.
// @Summary 删除下载记录（属主，支持批量）
// @Tags 桌面端活动
// @Param ids path string true "下载记录 ID（逗号分隔）"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/download/{ids} [delete]
func swaggerDatumDownloadDelete() {}

// swaggerDatumMyDownloads documents GET /datum/desktop/download/list/{userId}.
// @Summary 我的下载列表
// @Tags 桌面端活动
// @Param userId path int true "用户 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/download/list/{userId} [get]
func swaggerDatumMyDownloads() {}

// swaggerDatumMyDownloadRemove documents DELETE /datum/desktop/download/{userId}/{fileId}.
// @Summary 删除我的下载记录
// @Tags 桌面端活动
// @Param userId path int true "用户 ID"
// @Param fileId path int true "文件 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/download/{userId}/{fileId} [delete]
func swaggerDatumMyDownloadRemove() {}

// swaggerDatumMyFavorites documents GET /datum/desktop/favorite/list/{userId}.
// @Summary 我的收藏列表
// @Tags 桌面端活动
// @Param userId path int true "用户 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/favorite/list/{userId} [get]
func swaggerDatumMyFavorites() {}

// swaggerDatumMyFavoriteRemove documents DELETE /datum/desktop/favorite/{userId}/{fileId}.
// @Summary 取消收藏
// @Tags 桌面端活动
// @Param userId path int true "用户 ID"
// @Param fileId path int true "文件 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/favorite/{userId}/{fileId} [delete]
func swaggerDatumMyFavoriteRemove() {}

// swaggerDatumMyBookmarkFavorites documents GET /datum/desktop/bookmark/favorite/list/{userId}.
// @Summary 我的书签收藏列表（桌面个人中心）
// @Tags 桌面端活动
// @Param userId path int true "用户 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/bookmark/favorite/list/{userId} [get]
func swaggerDatumMyBookmarkFavorites() {}

// swaggerDatumMyBookmarkFavoriteRemove documents DELETE /datum/desktop/bookmark/favorite/{userId}/{bookmarkId}.
// @Summary 取消书签收藏
// @Tags 桌面端活动
// @Param userId path int true "用户 ID"
// @Param bookmarkId path int true "书签 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/desktop/bookmark/favorite/{userId}/{bookmarkId} [delete]
func swaggerDatumMyBookmarkFavoriteRemove() {}

// swaggerDatumFavoriteAdd documents POST /datum/favorite.
// @Summary 收藏试卷
// @Tags 桌面端活动
// @Param payload body object true "收藏信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/favorite [post]
func swaggerDatumFavoriteAdd() {}

// swaggerDatumFavoriteStatus documents GET /datum/favorite/{fileId}.
// @Summary 查询试卷收藏状态
// @Tags 桌面端活动
// @Param fileId path int true "文件 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/favorite/{fileId} [get]
func swaggerDatumFavoriteStatus() {}

// swaggerDatumBookmarkFavoriteAdd documents POST /datum/bookmark/favorite.
// @Summary 收藏书签
// @Tags 桌面端活动
// @Param payload body object true "收藏信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/bookmark/favorite [post]
func swaggerDatumBookmarkFavoriteAdd() {}

// ---------------------------------------------------------------------------
// 桌面端通知
// ---------------------------------------------------------------------------

// swaggerDatumNotificationList documents GET /datum/notification/list.
// @Summary 通知列表
// @Tags 桌面端通知
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/notification/list [get]
func swaggerDatumNotificationList() {}

// swaggerDatumNotificationDetail documents GET /datum/notification/{notifyId}.
// @Summary 通知详情
// @Tags 桌面端通知
// @Param notifyId path int true "通知 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/notification/{notifyId} [get]
func swaggerDatumNotificationDetail() {}

// swaggerDatumNotificationCreate documents POST /datum/notification.
// @Summary 新增通知（5 类型：1 版本更新 2 系统故障 3 系统维护 4 资料下架 5 公告）
// @Tags 桌面端通知
// @Security BearerAuth
// @Param payload body object true "通知内容"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:notification:manage 权限"
// @Router /datum/notification [post]
func swaggerDatumNotificationCreate() {}

// swaggerDatumNotificationUpdate documents PUT /datum/notification.
// @Summary 编辑通知
// @Tags 桌面端通知
// @Security BearerAuth
// @Param payload body object true "编辑字段"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:notification:manage 权限"
// @Router /datum/notification [put]
func swaggerDatumNotificationUpdate() {}

// swaggerDatumNotificationDelete documents DELETE /datum/notification/{ids}.
// @Summary 删除通知（支持批量）
// @Tags 桌面端通知
// @Security BearerAuth
// @Param ids path string true "通知 ID（逗号分隔）"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:notification:manage 权限"
// @Router /datum/notification/{ids} [delete]
func swaggerDatumNotificationDelete() {}

// swaggerDatumNotificationPopup documents GET /system/notification/user/popup.
// @Summary 用户弹窗通知（桌面端铃铛/首页弹窗）
// @Tags 桌面端通知
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /system/notification/user/popup [get]
func swaggerDatumNotificationPopup() {}

// swaggerDatumNotificationScroll documents GET /system/notification/user/scroll.
// @Summary 用户滚动提醒
// @Tags 桌面端通知
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /system/notification/user/scroll [get]
func swaggerDatumNotificationScroll() {}

// ---------------------------------------------------------------------------
// 桌面端举报
// ---------------------------------------------------------------------------

// swaggerDatumReportList documents GET /datum/report/list.
// @Summary 资料举报列表
// @Tags 桌面端举报
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerTableDataResponse "RuoYi TableDataInfo 形状"
// @Failure 500 {object} SwaggerErrorResponse
// @Router /datum/report/list [get]
func swaggerDatumReportList() {}

// swaggerDatumReportDetail documents GET /datum/report/{reportId}.
// @Summary 资料举报详情
// @Tags 桌面端举报
// @Param reportId path int true "举报 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/report/{reportId} [get]
func swaggerDatumReportDetail() {}

// swaggerDatumReportTimeline documents GET /datum/report/timeline/{fileId}.
// @Summary 举报处理时间线
// @Tags 桌面端举报
// @Param fileId path int true "文件 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/report/timeline/{fileId} [get]
func swaggerDatumReportTimeline() {}

// swaggerDatumReportCreate documents POST /datum/report.
// @Summary 提交资料举报
// @Tags 桌面端举报
// @Param payload body object true "举报信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse "已有待处理举报"
// @Router /datum/report [post]
func swaggerDatumReportCreate() {}

// swaggerDatumReportUpdate documents PUT /datum/report.
// @Summary 编辑举报（属主，待处理状态）
// @Tags 桌面端举报
// @Param payload body object true "编辑字段"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/report [put]
func swaggerDatumReportUpdate() {}

// swaggerDatumReportDelete documents DELETE /datum/report/{ids}.
// @Summary 撤回举报（属主，支持批量）
// @Tags 桌面端举报
// @Param ids path string true "举报 ID（逗号分隔）"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/report/{ids} [delete]
func swaggerDatumReportDelete() {}

// swaggerDatumReportAudit documents POST /datum/report/audit/{reportId}.
// @Summary 审核资料举报
// @Tags 桌面端举报
// @Security BearerAuth
// @Param reportId path int true "举报 ID"
// @Param payload body object true "审核结果与备注"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:report:audit 权限"
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/report/audit/{reportId} [post]
func swaggerDatumReportAudit() {}

// swaggerDatumBookmarkReportList documents GET /datum/bookmarkReport/list.
// @Summary 书签举报列表
// @Tags 桌面端举报
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} SwaggerTableDataResponse "RuoYi TableDataInfo 形状"
// @Failure 500 {object} SwaggerErrorResponse
// @Router /datum/bookmarkReport/list [get]
func swaggerDatumBookmarkReportList() {}

// swaggerDatumBookmarkReportDetail documents GET /datum/bookmarkReport/{reportId}.
// @Summary 书签举报详情
// @Tags 桌面端举报
// @Param reportId path int true "举报 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/bookmarkReport/{reportId} [get]
func swaggerDatumBookmarkReportDetail() {}

// swaggerDatumBookmarkReportCreate documents POST /datum/bookmarkReport.
// @Summary 提交书签举报
// @Tags 桌面端举报
// @Param payload body object true "举报信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse "已有待处理举报"
// @Router /datum/bookmarkReport [post]
func swaggerDatumBookmarkReportCreate() {}

// swaggerDatumBookmarkReportUpdate documents PUT /datum/bookmarkReport.
// @Summary 编辑书签举报（属主，待处理状态）
// @Tags 桌面端举报
// @Param payload body object true "编辑字段"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/bookmarkReport [put]
func swaggerDatumBookmarkReportUpdate() {}

// swaggerDatumBookmarkReportDelete documents DELETE /datum/bookmarkReport/{ids}.
// @Summary 撤回书签举报（属主，支持批量）
// @Tags 桌面端举报
// @Param ids path string true "举报 ID（逗号分隔）"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /datum/bookmarkReport/{ids} [delete]
func swaggerDatumBookmarkReportDelete() {}

// swaggerDatumBookmarkReportAudit documents POST /datum/bookmarkReport/audit/{reportId}.
// @Summary 审核书签举报
// @Tags 桌面端举报
// @Security BearerAuth
// @Param reportId path int true "举报 ID"
// @Param payload body object true "审核结果与备注"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse "缺少 datum:bookmarkReport:audit 权限"
// @Failure 404 {object} SwaggerErrorResponse
// @Router /datum/bookmarkReport/audit/{reportId} [post]
func swaggerDatumBookmarkReportAudit() {}
