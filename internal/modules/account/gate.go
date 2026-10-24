package account

import (
	"github.com/gin-gonic/gin"

	"gptgo/internal/platform/domain"
)

type Gate struct {
	enabled bool
	repo    *Repository
}

func NewGate(enabled bool, repo *Repository) *Gate {
	return &Gate{enabled: enabled, repo: repo}
}

func (g *Gate) Resolve(c *gin.Context) (domain.Scope, bool) {
	if !g.enabled {
		return domain.Scope{ActorID: 1}, true
	}
	token, err := c.Cookie(SessionCookie)
	if err != nil || token == "" {
		return domain.Scope{}, false
	}
	accountID, err := g.repo.ActiveAccountID(c.Request.Context(), token)
	if err != nil || accountID <= 0 {
		return domain.Scope{}, false
	}
	return domain.Scope{ActorID: accountID}, true
}
