package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Required methods to implement jwt.Claims interface
func (uc *UserClaims) GetIssuer() (string, error) {
	return uc.RegisteredClaims.Issuer, nil
}

func (uc *UserClaims) GetSubject() (string, error) {
	return uc.RegisteredClaims.Subject, nil
}

func (uc *UserClaims) GetAudience() (jwt.ClaimStrings, error) {
	return uc.RegisteredClaims.Audience, nil
}

func (uc *UserClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return uc.RegisteredClaims.ExpiresAt, nil
}

func (uc *UserClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return uc.RegisteredClaims.IssuedAt, nil
}

func (uc *UserClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return uc.RegisteredClaims.NotBefore, nil
}

func (uc *UserClaims) GetID() (string, error) {
	return uc.RegisteredClaims.ID, nil
}

func (uc *UserClaims) Valid() error {
	if uc.ExpiresAt != nil && !uc.ExpiresAt.Time.After(time.Now()) {
		return fmt.Errorf("token has expired")
	}
	if uc.NotBefore != nil && uc.NotBefore.Time.After(time.Now()) {
		return fmt.Errorf("token is not yet valid")
	}
	return nil
}

func ValidateToken(tokenString string) (*UserClaims, error) {
	secretKey := os.Getenv("JWT_SECRET_KEY")
	if secretKey == "" {
		return nil, fmt.Errorf("JWT_SECRET_KEY not set")
	}

	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	expirationTime, err := claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get expiration time: %w", err)
	}

	if !expirationTime.Time.After(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

func GenerateToken(userID, email, name, role string) (string, error) {
	secretKey := os.Getenv("JWT_SECRET_KEY")
	if secretKey == "" {
		return "", fmt.Errorf("JWT_SECRET_KEY not set")
	}

	claims := &UserClaims{
		UserID: userID,
		Email:  email,
		Name:   name,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "interview-system",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func RefreshToken(oldToken string) (string, error) {
	claims, err := ValidateToken(oldToken)
	if err != nil {
		return "", err
	}

	return GenerateToken(claims.UserID, claims.Email, claims.Name, claims.Role)
}
