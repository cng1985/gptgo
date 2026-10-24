package account

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"gptgo/internal/platform/response"
)

type Controller struct {
	resource *Resource
	ttl      time.Duration
}

func NewController(resource *Resource, ttl time.Duration) *Controller {
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &Controller{resource: resource, ttl: ttl}
}

func (c *Controller) Register(api *gin.RouterGroup) {
	api.POST("/account/register", c.register)
	api.POST("/account/login", c.login)
	api.POST("/account/logout", c.logout)
	api.POST("/account/one", c.one)
	api.POST("/account/update", c.update)
}

func (c *Controller) register(ctx *gin.Context) {
	var request AuthRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, token, err := c.resource.Register(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	c.writeSession(ctx, token)
	response.SuccessResponse(ctx, response.OK(result))
}

func (c *Controller) login(ctx *gin.Context) {
	var request AuthRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, token, err := c.resource.Login(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	c.writeSession(ctx, token)
	response.SuccessResponse(ctx, response.OK(result))
}

func (c *Controller) logout(ctx *gin.Context) {
	token, _ := ctx.Cookie(SessionCookie)
	if err := c.resource.Logout(ctx.Request.Context(), token); err != nil {
		response.HandleError(ctx, err)
		return
	}
	c.clearSession(ctx)
	response.SuccessResponse(ctx, response.OK(LogoutResponse{OK: true}))
}

func (c *Controller) writeSession(ctx *gin.Context, token string) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(SessionCookie, token, int(c.ttl.Seconds()), "/", "", false, true)
}

func (c *Controller) clearSession(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(SessionCookie, "", -1, "/", "", false, true)
}

func (c *Controller) one(ctx *gin.Context) {
	result, err := c.resource.One(ctx.Request.Context(), &AccountRequest{})
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) update(ctx *gin.Context) {
	var request AccountRequest
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
