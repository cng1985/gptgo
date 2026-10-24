package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"gptgo/internal/platform/response"
)

func RegisterStatic(engine *gin.Engine) error {
	sub, err := fs.Sub(Dist, "dist")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(sub))
	engine.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if strings.HasPrefix(requestPath, "/api") || strings.HasPrefix(requestPath, "/health") {
			response.Fail(c, http.StatusNotFound, 404, "接口不存在")
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		clean := path.Clean("/" + requestPath)
		name := strings.TrimPrefix(clean, "/")
		if name != "" && !strings.Contains(name, "..") {
			file, openErr := sub.Open(name)
			if openErr == nil {
				stat, statErr := file.Stat()
				file.Close()
				if statErr == nil && !stat.IsDir() {
					c.Request.URL.Path = clean
					if strings.HasPrefix(name, "assets/") {
						c.Header("Cache-Control", "public, max-age=31536000, immutable")
					}
					fileServer.ServeHTTP(c.Writer, c.Request)
					return
				}
			}
		}
		if name != "" && strings.Contains(path.Base(name), ".") {
			c.Status(http.StatusNotFound)
			return
		}
		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
	return nil
}
