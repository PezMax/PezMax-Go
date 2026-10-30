package kadmin

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/textguard"
	"github.com/gin-gonic/gin"
)

// 用户可见短文本（文件名、用户名、标题/简介/备注）的敏感词写入拦截。
//
// 词表是纯文本文件，默认 data/sensitive_words.txt，可在“参数配置”里改路径或
// 用 KADMIN_SENSITIVE_WORDS_PATH 覆盖；内容变更后自动热加载，无需重启。
// 拦截在写入前完成：命中即 400，不落库、不占存储；返回文案不包含命中的词，
// 避免被用来试探词表，命中明细只写服务端日志。
//
// 词表缺失或读取失败时放行（可用性优先），并在日志里报一次错误。

const (
	sensitiveWordsPathEnv = "KADMIN_SENSITIVE_WORDS_PATH"

	sensitiveWordsEnabledKey  = "security.sensitive_words_enabled"
	sensitiveWordsPathKey     = "security.sensitive_words_path"
	defaultSensitiveWordsPath = "data/sensitive_words.txt"
)

// textGuard returns the store-wide guard, sharing one loaded dictionary and
// one file watch across all handlers.
func (s *Store) textGuard() *textguard.Guard {
	s.guardOnce.Do(func() {
		if s.sensitiveWords == nil {
			s.sensitiveWords = textguard.New()
		}
	})
	return s.sensitiveWords
}

// rejectSensitiveText writes a 400 and returns true when any field contains a
// prohibited word. Checks are skipped when the config switch is off, so an
// operator can stop blocking during an incident without a deploy.
func (s *Store) rejectSensitiveText(c *gin.Context, fields ...textguard.Field) bool {
	values, err := s.readSystemConfig()
	if err != nil {
		log.Printf("敏感词配置读取失败，本次跳过检查：%v", err)
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(values[sensitiveWordsEnabledKey]), "true") {
		return false
	}
	guard := s.textGuard()
	guard.SetPath(resolveSensitiveWordsPath(values[sensitiveWordsPathKey]))
	label, word, hit := guard.CheckFields(fields)
	if !hit {
		return false
	}
	actor := int64(0)
	if userID, ok := datumUserIDFrom(c); ok {
		actor = userID
	}
	log.Printf("敏感词拦截：路径=%s 字段=%s 命中=%q 用户=%d", c.Request.URL.Path, label, word, actor)
	fail(c, http.StatusBadRequest, fmt.Sprintf("%s包含违规内容，请修改后重试", label))
	return true
}

// resolveSensitiveWordsPath resolves the word-list file: the env override wins
// over the stored config, and relative paths resolve against the kadmin root
// (the same base as data/system_config.json).
func resolveSensitiveWordsPath(configured string) string {
	path := strings.TrimSpace(os.Getenv(sensitiveWordsPathEnv))
	if path == "" {
		path = strings.TrimSpace(configured)
	}
	if path == "" {
		path = defaultSensitiveWordsPath
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(systemConfigProjectRoot(), path))
}

// rejectSensitiveBookmark checks the free-text bookmark columns; URL and cover
// image are links, not prose, and are intentionally not scanned (a legitimate
// path could contain any Latin fragment).
func (s *Store) rejectSensitiveBookmark(c *gin.Context, req *datumBookmarkWriteRequest) bool {
	return s.rejectSensitiveText(c,
		textguard.Field{Label: "标题", Value: req.Title},
		textguard.Field{Label: "简介", Value: req.Description},
		textguard.Field{Label: "合集", Value: req.Collection},
		textguard.Field{Label: "科目", Value: req.Subject},
		textguard.Field{Label: "备注", Value: req.Remark},
	)
}
