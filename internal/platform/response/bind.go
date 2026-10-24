package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	apperr "gptgo/internal/platform/errors"
)

func BindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			return apperr.TooLarge("请求体过大")
		}
		return apperr.Invalid("参数格式错误")
	}
	return nil
}
