package kadmin

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"unicode/utf8"

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

func validDatumUTF8(reader io.Reader) (bool, error) {
	buffer := make([]byte, (64<<10)+3)
	carry := 0
	for {
		n, err := reader.Read(buffer[carry : carry+(64<<10)])
		size := carry + n
		matched := false
		for tail := 0; tail <= 3 && tail <= size; tail++ {
			if utf8.Valid(buffer[:size-tail]) && (tail == 0 || !utf8.FullRune(buffer[size-tail:size])) {
				copy(buffer[:tail], buffer[size-tail:size])
				carry = tail
				matched = true
				break
			}
		}
		if !matched {
			return false, nil
		}
		if err == io.EOF {
			return carry == 0, nil
		}
		if err != nil {
			return false, err
		}
	}
}
