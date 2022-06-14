package middlewares

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/storage"
)

// AuthBasic получает пользователя по токену из заголовка и передает в хендлер
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

// AuthWithBalance аналогично AuthBasic, но более затратный, т.к. запрашивает еще и балансы пользователя
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

// InstanceID проставляет инстанс в хедер, на случай дальнейшего масштабирования
func InstanceID(id int) gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Set("instance", fmt.Sprintf("%d", id))
	}
}

// RateLimit ограничитель запросов
func RateLimit() gin.HandlerFunc {
	return func(context *gin.Context) {
		// TO-DO
	}
}

// StatCollector счетчик посезений страниц
func StatCollector() gin.HandlerFunc {
	return func(context *gin.Context) {
		// TO-DO
	}
}
