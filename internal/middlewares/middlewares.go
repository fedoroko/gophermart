package middlewares

import (
	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/storage"
	"github.com/gin-gonic/gin"
	"net/http"
)

func AuthBasic(db storage.Repo, logger *config.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if len(token) == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		s, err := db.SessionCheck(c.Request.Context(), token)
		if err != nil {
			logger.Error().Err(err).Send()
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set("session", s)
		c.Next()
	}
}

func AuthWithBalance(db storage.Repo, logger *config.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if len(token) == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		s, err := db.SessionBalanceCheck(c.Request.Context(), token)
		if err != nil {
			logger.Error().Err(err).Send()
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set("session", s)
		c.Next()
	}
}

func InstanceID(id int) gin.HandlerFunc {
	return func(context *gin.Context) {

	}
}

func RateLimit() gin.HandlerFunc {
	return func(context *gin.Context) {

	}
}
