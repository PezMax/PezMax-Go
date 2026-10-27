package kadmin

import (
	"io"
	"net/http"
	"strconv"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/fileurl"
	"github.com/gin-gonic/gin"
)

const datumKnowledgeMaxBytes int64 = 16 << 20

var datumKnowledgeOpen = openDatumObject

func (s *Store) registerDatumKnowledgeRoutes(group *gin.RouterGroup) {
	knowledge := group.Group("/knowledge")
	knowledge.GET("/manifest", s.requireDatumAuth(), s.datumKnowledgeManifest)
	knowledge.GET("/source", s.requireDatumAuth(), s.datumKnowledgeSource)
	// As with existing previews, only currently approved public rows may be
	// previewed anonymously. No tokens in links; no download-history side effect.
	knowledge.GET("/content", s.datumKnowledgeContent)
}

func (s *Store) datumKnowledgeManifest(c *gin.Context) {
	rows, total, err := datum.KnowledgeManifest(s.conn)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "平台知识目录暂不可用")
		return
	}
	c.Header("Cache-Control", "no-store")
	success(c, gin.H{"sources": rows, "total": total, "partial": total > int64(len(rows))})
}

func (s *Store) knowledgeSource(c *gin.Context) (datum.KnowledgeSource, bool) {
	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "资源 ID 无效")
		return datum.KnowledgeSource{}, false
	}
	var source datum.KnowledgeSource
	found := false
	switch c.Query("kind") {
	case "file":
		var row datum.File
		row, found, err = datum.NewFileRepo(s.conn).FindByID(id)
		found = found && row.FileStatus == 1
		source = datum.KnowledgeSource{Kind: "file", ID: strconv.FormatInt(id, 10), Title: row.FileName,
			Format: row.FileFormat, Subject: row.FileSubject, School: row.FileSchool, Year: row.FileYear,
			URL: row.FileURL, Size: row.FileSize, Updated: row.UpdateTime}
	case "ebook":
		var row datum.Ebook
		row, found, err = datum.NewEbookRepo(s.conn).FindByID(id)
		found = found && row.EbookStatus == 1
		source = datum.KnowledgeSource{Kind: "ebook", ID: strconv.FormatInt(id, 10), Title: row.EbookName,
			Format: row.EbookFormat, Subject: row.EbookSubject, URL: row.EbookURL, Size: row.EbookSize, Updated: row.UpdateTime}
	default:
		fail(c, http.StatusBadRequest, "资源类型无效")
		return source, false
	}
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "平台资源查询失败")
		return source, false
	}
	if !found {
		fail(c, http.StatusNotFound, "资源已下架、删除或不可访问")
		return source, false
	}
	if source.Size < 1 || source.Size > datumKnowledgeMaxBytes ||
		(source.Format != "pdf" && source.Format != "epub" && source.Format != "txt" && source.Format != "md") {
		fail(c, http.StatusUnprocessableEntity, "资源暂不支持正文索引")
		return source, false
	}
	source = source.Versioned()
	if revision := c.Query("revision"); revision != "" && revision != source.Revision {
		fail(c, http.StatusConflict, "资源版本已变化，请重新同步")
		return source, false
	}
	c.Header("Cache-Control", "no-store")
	return source, true
}

func (s *Store) datumKnowledgeSource(c *gin.Context) {
	if source, ok := s.knowledgeSource(c); ok {
		success(c, source)
	}
}

func (s *Store) datumKnowledgeContent(c *gin.Context) {
	source, ok := s.knowledgeSource(c)
	if !ok {
		return
	}
	bucket, key, err := fileurl.Parse(source.URL)
	if err != nil {
		fail(c, http.StatusNotFound, "资源正文不可用")
		return
	}
	body, info, err := datumKnowledgeOpen(c.Request.Context(), bucket, key)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "资源正文暂不可用")
		return
	}
	defer body.Close()
	if info.Size > datumKnowledgeMaxBytes {
		fail(c, http.StatusRequestEntityTooLarge, "资源正文过大")
		return
	}
	data, err := io.ReadAll(io.LimitReader(body, datumKnowledgeMaxBytes+1))
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "资源正文读取失败")
		return
	}
	if int64(len(data)) > datumKnowledgeMaxBytes {
		fail(c, http.StatusRequestEntityTooLarge, "资源正文过大")
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "inline")
	c.Data(http.StatusOK, mimeByExt(source.Format), data)
}
