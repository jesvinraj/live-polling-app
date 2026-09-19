package middleware

import (
	"net/http"
	"strings"

	"live-polling-tool/services"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserIDKey   = "userID"
	CtxEmailKey    = "userEmail"
	CtxUsernameKey = "userUsername"
)

func AuthMiddleware(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header is required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization format must be Bearer <token>"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set(CtxUserIDKey, claims.UserID)
		c.Set(CtxEmailKey, claims.Email)
		c.Set(CtxUsernameKey, claims.Username)
		c.Next()
	}
}
