package kadmin

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/fileurl"
	"github.com/gin-gonic/gin"
)

// Desktop ebook module：电子书目录在 ptmj_ebook（后台经 codegen 模块管理，
// 审核语义与 ptmj_file 对齐）。桌面端契约：
//
//   - GET /datum/ebook/list     分页的已上架电子书（仅 ebook_status=1；
//     不做年份/学校细分，页面直接按书名展示，关键词匹配书名/作者/出版社）
//   - GET /datum/ebook/content  在线预览流（inline；pdf/epub 常见格式，
//     pdf 走 Chromium 内建阅读器，epub 由渲染端 epub.js 解析）
//   - 下载设专有端点 GET /datum/download/ebook?ebookId=（datum 会话鉴权、
//     落 ptmj_ebook_download 记录、复用同一条 nginx 边缘整形路径）。
//
// 电子书可携带的内容格式由 datumAllowedExts 保证（pdf/epub/mobi/azw3）。

const (
	datumEbookPreviewFormats = "pdf/epub 在线预览；mobi/azw3 请下载后阅读"
	datumEbookDefaultSubject = "综合"
)

func (s *Store) registerDatumEbookRoutes(datumGroup *gin.RouterGroup) {
	// gin v1.3 的 GET 路由树不允许静态子节点与 :param 并存，统一走通配分发器。
	ebooks := datumGroup.Group("/ebook")
	ebooks.GET("/*rest", s.datumEbookGet)
}

// datumEbookGet dispatches the read-only ebook routes: /list 与 /content。
func (s *Store) datumEbookGet(c *gin.Context) {
	rest := strings.Trim(c.Param("rest"), "/")
	switch {
	case rest == "list":
		s.datumEbookList(c)
	case rest == "content":
		s.datumEbookContent(c)
	default:
		fail(c, http.StatusNotFound, "接口不存在")
	}
}

// datumEbookPayload shapes one ptmj_ebook row for the desktop bookshelf:
// 封面与正文 URL 统一走 datumReadableFileURL（minio:// 改写为公网可读地址）。
func datumEbookPayload(ebook datum.Ebook) gin.H {
	return gin.H{
		"id":           ebook.EbookID,
		"ebookId":      ebook.EbookID,
		"label":        ebook.EbookName,
		"ebookName":    ebook.EbookName,
		"author":       ebook.Author,
		"publisher":    ebook.Publisher,
		"coverUrl":     datumReadableFileURL(ebook.CoverURL),
		"ebookUrl":     datumReadableFileURL(ebook.EbookURL),
		"ebookSize":    ebook.EbookSize,
		"ebookFormat":  ebook.EbookFormat,
		"ebookSubject": ebook.EbookSubject,
		"ebookType":    ebook.EbookType,
		"ebookStatus":  ebook.EbookStatus,
		"userId":       ebook.UserID,
		"remark":       ebook.Remark,
		"type":         "ebook",
	}
}

// datumEbookList serves the paged approved ebook list. Anonymous browsing
// matches /datum/file/list 语义（只出已过审内容）；书名/作者/出版社关键词联想。
func (s *Store) datumEbookList(c *gin.Context) {
	page, size := datumPageParams(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	if len(keyword) > 100 {
		keyword = keyword[:100]
	}
	result, err := datum.NewEbookRepo(s.conn).List(datum.EbookFilter{
		Page:         page,
		PageSize:     size,
		Keyword:      keyword,
		EbookSubject: strings.TrimSpace(c.Query("subject")),
		EbookType:    datumQueryInt(c, "ebookType"),
		OnlyApproved: true,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "电子书列表查询失败")
		return
	}
	items, _ := result.Items.([]datum.Ebook)
	rows := make([]gin.H, 0, len(items))
	for _, ebook := range items {
		rows = append(rows, datumEbookPayload(ebook))
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"msg":     "ok",
		"rows":    rows,
		"total":   result.Total,
	})
}

// datumEbookContent implements GET /datum/ebook/content?ebookId= — the
// inline preview stream. 与试卷预览同语义：已过审内容匿名可读，属主可看
// 自己未过审的书；inline 便于 Chromium 内建 PDF 阅读器与 epub.js 拉流，
// 不落下载记录（下载走 /datum/download/ebook）。
func (s *Store) datumEbookContent(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	ebookID := toDatumInt64(strings.TrimSpace(c.Query("ebookId")))
	if ebookID <= 0 {
		fail(c, http.StatusBadRequest, "电子书 ID 不能为空")
		return
	}
	ebook, found, err := datum.NewEbookRepo(s.conn).FindByID(ebookID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "电子书查询失败")
		return
	}
	if !found || (ebook.EbookStatus != 1 && ebook.UserID != userID) {
		fail(c, http.StatusNotFound, "电子书不存在或未上架")
		return
	}
	bucket, objectKey, parseErr := fileurl.Parse(ebook.EbookURL)
	if parseErr != nil {
		fail(c, http.StatusNotFound, "电子书内容不可用")
		return
	}
	body, info, err := openDatumObject(c.Request.Context(), bucket, objectKey)
	if err != nil {
		fail(c, http.StatusNotFound, "电子书内容不可用")
		return
	}
	defer body.Close()

	contentType := mimeByExt(strings.ToLower(ebook.EbookFormat))
	fileName := path.Base(strings.ReplaceAll(ebook.EbookName, "\\", "/"))
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": fileName})
	if disposition == "" {
		disposition = "inline"
	}
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", contentType)
	// 强 ETag 支撑客户端缓存与 If-Range 续拉：对象键写入后不变，
	// id+size 足以标识字节表示（与 /datum/download/file 同款约定）。
	if info.Size > 0 {
		c.Header("ETag", fmt.Sprintf(`"ebook-%d-%d"`, ebook.EbookID, info.Size))
	}
	if seeker, ok := body.(io.ReadSeeker); ok {
		http.ServeContent(c.Writer, c.Request, fileName, time.Time{}, seeker)
		return
	}
	if info.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	c.DataFromReader(http.StatusOK, info.Size, contentType, body, map[string]string{})
}
