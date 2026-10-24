package chatbi

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
	api.POST("/datasource/create", c.createSource)
	api.POST("/datasource/update", c.updateSource)
	api.POST("/datasource/one", c.oneSource)
	api.POST("/datasource/search", c.searchSource)
	api.POST("/datasource/delete", c.deleteSource)
	api.POST("/datasource/test", c.testSource)
	api.POST("/datasource/schema", c.schemaSource)
	api.POST("/chatbi/ask", c.ask)
	api.POST("/billing/one", c.billingOne)
	api.POST("/billing/search", c.billingSearch)
	api.POST("/billing/topup", c.topup)
	api.POST("/usage/summary", c.usageSummary)
	api.POST("/usage/search", c.usageSearch)
}

func (c *Controller) createSource(ctx *gin.Context) {
	var request DatasourceRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Create(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) updateSource(ctx *gin.Context) {
	var request DatasourceRequest
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

func (c *Controller) oneSource(ctx *gin.Context) {
	var request DatasourceRequest
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

func (c *Controller) searchSource(ctx *gin.Context) {
	var search DatasourceSearch
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

func (c *Controller) deleteSource(ctx *gin.Context) {
	var request DatasourceRequest
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

func (c *Controller) testSource(ctx *gin.Context) {
	var request DatasourceRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Test(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) schemaSource(ctx *gin.Context) {
	var request DatasourceRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Schema(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) ask(ctx *gin.Context) {
	var request AskRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Ask(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) billingOne(ctx *gin.Context) {
	result, err := c.resource.Wallet(ctx.Request.Context())
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) billingSearch(ctx *gin.Context) {
	var search BillingSearch
	if err := bindOptional(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.SearchBilling(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func (c *Controller) topup(ctx *gin.Context) {
	var request TopupRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Topup(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) usageSummary(ctx *gin.Context) {
	var request UsageSummaryRequest
	if err := bindOptional(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Summarize(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) usageSearch(ctx *gin.Context) {
	var search UsageSearch
	if err := bindOptional(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.SearchUsage(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func bindOptional(ctx *gin.Context, dst any) error {
	if ctx.Request.ContentLength == 0 {
		return nil
	}
	return response.BindJSON(ctx, dst)
}
