package skill

import (
	"github.com/gin-gonic/gin"

	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

func (c *Controller) Register(api *gin.RouterGroup) {
	api.POST("/skill/search", c.search)
	api.POST("/skill/one", c.one)
	api.POST("/skill/update", c.update)
	api.POST("/skill/delete", c.delete)
	api.POST("/skill/upload", c.upload)
	api.POST("/skill/hub/search", c.searchHub)
	api.POST("/skill/hub/install", c.installHub)
}

func (c *Controller) search(ctx *gin.Context) {
	var search SkillSearch
	if err := bindOptional(ctx, &search); err != nil {
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

func (c *Controller) one(ctx *gin.Context) {
	var request SkillRequest
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

func (c *Controller) update(ctx *gin.Context) {
	var request SkillRequest
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

func (c *Controller) delete(ctx *gin.Context) {
	var request SkillRequest
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

func (c *Controller) upload(ctx *gin.Context) {
	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		response.HandleError(ctx, apperr.Invalid("请上传 .zip 或 SKILL.md"))
		return
	}
	defer file.Close()
	name := ""
	if header != nil {
		name = header.Filename
	}
	result, err := c.resource.Upload(ctx.Request.Context(), name, file)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) searchHub(ctx *gin.Context) {
	var search HubSearch
	if err := bindOptional(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.SearchHub(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func (c *Controller) installHub(ctx *gin.Context) {
	var request HubInstall
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.InstallHub(ctx.Request.Context(), &request)
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
