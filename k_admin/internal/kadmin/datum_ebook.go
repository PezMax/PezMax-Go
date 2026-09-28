package kadmin

import (
	"net/http"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/gin-gonic/gin"
)

// Desktop ebook module：电子书就是 ptmj_file 中 fileType=7 的文件（见
// docs/pezmax-remaining-tasks.md——电子书模块与文件模块类似，共享 QoS），
// 复用鉴权、分页、hash 文件树与下载记录，这里只提供页面专有契约：
//
//   - GET /datum/ebook/list      分页的已过审电子书列表（行结构与
//     /datum/file/list 完全一致，页面可直接复用表格组件）
//   - 下载不设专有端点：与全部文件下载共用统一的
//     GET /datum/download/file?fileId=（同一条 datum 会话鉴权、同一条
//     ptmj_file_download 记录、同一条 nginx 边缘整形路径）。
//
// 电子书可携带的内容格式由 datumAllowedExts 保证（pdf/epub/mobi/azw3）。

const datumEbookFileType = 7

func (s *Store) registerDatumEbookRoutes(datumGroup *gin.RouterGroup) {
	// gin v1.3 的 GET 路由树不允许静态子节点与 :param 并存，统一走通配分发器。
	ebooks := datumGroup.Group("/ebook")
	ebooks.GET("/*rest", s.datumEbookGet)
}

// datumEbookGet dispatches the read-only ebook routes: /list（后续可扩详情）。
func (s *Store) datumEbookGet(c *gin.Context) {
	rest := strings.Trim(c.Param("rest"), "/")
	switch {
	case rest == "list":
		s.datumEbookList(c)
	default:
		fail(c, http.StatusNotFound, "接口不存在")
	}
}

// datumEbookList serves the paged approved ebook list. Anonymous browsing
// matches /datum/file/list 语义（只出已过审内容）；行结构同款。
func (s *Store) datumEbookList(c *gin.Context) {
	page, size := datumPageParams(c)
	result, err := datum.NewFileRepo(s.conn).List(datum.FileFilter{
		Page:         page,
		PageSize:     size,
		FileType:     datumEbookFileType,
		OnlyApproved: true,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "电子书列表查询失败")
		return
	}
	items, _ := result.Items.([]datum.File)
	rows := make([]gin.H, 0, len(items))
	for _, file := range items {
		rows = append(rows, datumFilePayload(file))
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"msg":     "ok",
		"rows":    rows,
		"total":   result.Total,
	})
}
