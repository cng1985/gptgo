package routing

import "github.com/gin-gonic/gin"

type Registrar interface {
	Register(api *gin.RouterGroup)
}
