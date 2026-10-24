package chat

import (
	"github.com/gin-gonic/gin"

	"gptgo/internal/platform/response"
)

type Controller struct {
	resource *Resource
	models   Streamer
}

func NewController(resource *Resource, models Streamer) *Controller {
	return &Controller{resource: resource, models: models}
}

func (c *Controller) Register(api *gin.RouterGroup) {
	api.POST("/conversation/create", c.create)
	api.POST("/conversation/update", c.update)
	api.POST("/conversation/one", c.one)
	api.POST("/conversation/search", c.search)
	api.POST("/conversation/delete", c.delete)
	api.POST("/conversation/clear", c.clear)
	api.POST("/message/search", c.messages)
	api.POST("/chat/stream", c.stream)
	api.POST("/chat/optimize", c.optimize)
	api.POST("/ask", c.ask)
}

func (c *Controller) create(ctx *gin.Context) {
	request := ConversationRequest{}
	if ctx.Request.ContentLength > 0 {
		if err := response.BindJSON(ctx, &request); err != nil {
			response.HandleError(ctx, err)
			return
		}
	}
	result, err := c.resource.Create(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) update(ctx *gin.Context) {
	var request ConversationRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Update(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) one(ctx *gin.Context) {
	var request ConversationRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.One(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) search(ctx *gin.Context) {
	var search ConversationSearch
	if err := response.BindJSON(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Search(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func (c *Controller) delete(ctx *gin.Context) {
	var request ConversationRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Delete(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) clear(ctx *gin.Context) {
	var request ConversationClearRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Clear(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) messages(ctx *gin.Context) {
	var search MessageSearch
	if err := response.BindJSON(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.SearchMessages(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func (c *Controller) stream(ctx *gin.Context) {
	ServeStream(ctx, c.resource, c.models)
}

func (c *Controller) ask(ctx *gin.Context) {
	var request AskRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.AskOnce(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) optimize(ctx *gin.Context) {
	var request ChatOptimizeRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Optimize(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}
