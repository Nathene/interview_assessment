// filepath: /Users/nathanhensby/Documents/dev/app/interview_assessment/auth/middleware.go
package auth

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// AuthMiddleware validates JWT tokens and adds user claims to context
func AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := c.Request().Header.Get("Authorization")
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "No authorization token provided",
				})
			}

			tokenString := strings.TrimPrefix(token, "Bearer ")
			claims, err := ValidateToken(tokenString)
			if err != nil {
				return echo.ErrUnauthorized
			}

			c.Set("user", claims)
			return next(c)
		}
	}
}

// RequireRole middleware checks if the user has the required role
func RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get("user").(*UserClaims)
			if !ok {
				return echo.ErrUnauthorized
			}

			for _, role := range roles {
				if user.Role == role {
					return next(c)
				}
			}

			return echo.ErrForbidden
		}
	}
}
