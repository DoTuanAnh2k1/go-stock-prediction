package token

import (
	"errors"
	"go-stock-prediction/pkg/logger"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Todo: Get from file or .env
var secretKey = []byte("le-chi-phat-aka-phat-lc")

func CreateToken(username string, permission string) (string, error) {
	// Create a new JWT token with claims
	claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,                               // Subject (user identifier)
		"aud": permission,                             // Audience (user role)
		"exp": time.Now().Add(1 * time.Minute).Unix(), // Expiration time
		"iat": time.Now().Unix(),                      // Issued at
	})

	// Print information about the created token
	logger.Logger.Infof("Token claims added: %+v\n", claims)

	tokenString, err := claims.SignedString(secretKey)
	if err != nil {
		logger.Logger.Error("Cannot create json web token: ", err)
		return "", err
	}

	// Don't add prefix here, let the client handle it
	return tokenString, nil
}

// ParseToken parses the JWT token and returns the username and roles
func ParseToken(tokenString string) (string, string, error) {
	// Remove the "Basic " prefix if it exists (for backward compatibility)
	tokenString = strings.TrimPrefix(tokenString, "Basic ")

	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			logger.Logger.Error("unexpected signing method")
			return nil, errors.New("unexpected signing method")
		}
		return secretKey, nil
	})

	if err != nil {
		logger.Logger.Error("Error parsing token: ", err)
		return "", "", err
	}

	// Extract claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Retrieve the username and roles from the claims
		username, ok1 := claims["sub"].(string)
		roles, ok2 := claims["aud"].(string)

		if !ok1 || !ok2 {
			return "", "", errors.New("invalid token claims")
		}

		return username, roles, nil
	}

	return "", "", errors.New("invalid token")
}
