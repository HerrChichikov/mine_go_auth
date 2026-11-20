package services

import (
	"errors"
	"fmt"
	"time"

	"alpha/internal/interfaces"

	"github.com/golang-jwt/jwt/v5"
)

type tokenService struct {
	secretKey []byte
}

func NewTokenService(secretKey string) interfaces.TokenService {
	return &tokenService{
		secretKey: []byte(secretKey),
	}
}

func (ts *tokenService) GenerateRegistrationToken(invitationID string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  invitationID,
		"type": "registration",
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(ts.secretKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenString, nil
}

func (ts *tokenService) ParseRegistrationToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return ts.secretKey, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token")
	}

	// Проверяем тип токена
	if claims["type"] != "registration" {
		return "", errors.New("invalid token type")
	}

	invitationID, ok := claims["sub"].(string)
	if !ok {
		return "", errors.New("invalid sub claim")
	}

	return invitationID, nil
}
