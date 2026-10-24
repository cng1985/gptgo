package response

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	apperr "gptgo/internal/platform/errors"
)

func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	var max *http.MaxBytesError
	if errors.As(err, &max) {
		c.JSON(200, Response[any]{Code: 413, Msg: "请求体过大"})
		return
	}
	ae, ok := apperr.As(err)
	if !ok {
		slog.Default().Error("unclassified error", "err", err.Error(), "path", c.FullPath())
		c.JSON(200, Response[any]{Code: 500, Msg: "服务内部错误"})
		return
	}
	if ae.AppCode >= 500 {
		cause := ""
		if ae.Cause != nil {
			cause = ae.Cause.Error()
		}
		slog.Default().Error("request failed", "code", ae.AppCode, "category", string(ae.Cat), "err", cause, "path", c.FullPath())
	}
	c.JSON(200, Response[any]{Code: ae.AppCode, Msg: ae.Msg})
}

func Fail(c *gin.Context, status int, code int, msg string) {
	c.JSON(status, Response[any]{Code: code, Msg: msg})
}
