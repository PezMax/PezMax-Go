package kadmin

import (
	"errors"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
)

const datumMultipartMemory = 8 << 20
const datumMultipartOverhead = 1 << 20

func datumMultipartFile(c *gin.Context, maxFileBytes int64, missingMessage string) (*multipart.FileHeader, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileBytes+datumMultipartOverhead)
	if err := c.Request.ParseMultipartForm(datumMultipartMemory); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			fail(c, http.StatusRequestEntityTooLarge, "上传请求过大")
		} else {
			fail(c, http.StatusBadRequest, missingMessage)
		}
		return nil, false
	}
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, missingMessage)
		return nil, false
	}
	return file, true
}

func cleanupDatumMultipart(c *gin.Context) {
	if c.Request.MultipartForm != nil {
		_ = c.Request.MultipartForm.RemoveAll()
	}
}
