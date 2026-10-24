package toolkit

import (
	"github.com/gin-gonic/gin"

	"gptgo/internal/platform/response"
)

type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

func (c *Controller) Register(api *gin.RouterGroup) {
	api.POST("/tool/search", c.searchBuiltin)
	api.POST("/tool/update", c.updateBuiltin)
	api.POST("/httptool/create", c.createHTTP)
	api.POST("/httptool/update", c.updateHTTP)
	api.POST("/httptool/one", c.oneHTTP)
	api.POST("/httptool/search", c.searchHTTP)
	api.POST("/httptool/delete", c.deleteHTTP)
	api.POST("/mcp/create", c.createMCP)
	api.POST("/mcp/update", c.updateMCP)
	api.POST("/mcp/one", c.oneMCP)
	api.POST("/mcp/search", c.searchMCP)
	api.POST("/mcp/delete", c.deleteMCP)
	api.POST("/mcp/tools", c.toolsMCP)
}

func (c *Controller) searchBuiltin(ctx *gin.Context) {
	var search ToolSearch
	if err := bindOptional(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.SearchBuiltin(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func (c *Controller) updateBuiltin(ctx *gin.Context) {
	var request BuiltinUpdate
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.UpdateBuiltin(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) createHTTP(ctx *gin.Context) {
	var request HTTPToolRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.CreateHTTP(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) updateHTTP(ctx *gin.Context) {
	var request HTTPToolRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.UpdateHTTP(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) oneHTTP(ctx *gin.Context) {
	var request HTTPToolRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.OneHTTP(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) searchHTTP(ctx *gin.Context) {
	var search HTTPToolSearch
	if err := bindOptional(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.SearchHTTP(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func (c *Controller) deleteHTTP(ctx *gin.Context) {
	var request HTTPToolRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.DeleteHTTP(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) createMCP(ctx *gin.Context) {
	var request MCPRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.CreateMCP(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) updateMCP(ctx *gin.Context) {
	var request MCPRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.UpdateMCP(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) oneMCP(ctx *gin.Context) {
	var request MCPRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.OneMCP(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) searchMCP(ctx *gin.Context) {
	var search MCPSearch
	if err := bindOptional(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.SearchMCP(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func (c *Controller) deleteMCP(ctx *gin.Context) {
	var request MCPRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.DeleteMCP(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) toolsMCP(ctx *gin.Context) {
	var request MCPRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.RemoteTools(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func bindOptional(ctx *gin.Context, dest any) error {
	if ctx.Request.ContentLength == 0 {
		return nil
	}
	return response.BindJSON(ctx, dest)
}
