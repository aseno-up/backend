package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/auth/model"
	user_model "github.com/saas/backend/pkg/user/model"
)

// JWTService defines the interface for JWT operations
type JWTService interface {
	GenerateTokenPair(user *user_model.User) (*model.TokenPair, error)
	GenerateAccessToken(user *user_model.User) (string, time.Time, error)
	GenerateRefreshToken(user *user_model.User) (string, error)
	ValidateAccessToken(tokenString string) (*model.JWTClaims, error)
	ValidateRefreshToken(tokenString string) (*model.JWTClaims, error)
}

// jwtService implements JWTService
type jwtService struct {
	secret          string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	issuer          string
}

// NewJWTService creates a new JWT service
func NewJWTService(secret string, accessTokenTTLMinutes, refreshTokenTTLDays int) JWTService {
	return &jwtService{
		secret:          secret,
		accessTokenTTL:  time.Duration(accessTokenTTLMinutes) * time.Minute,
		refreshTokenTTL: time.Duration(refreshTokenTTLDays) * 24 * time.Hour,
		issuer:          "saas-clinic",
	}
}

// GenerateTokenPair generates both access and refresh tokens
func (s *jwtService) GenerateTokenPair(user *user_model.User) (*model.TokenPair, error) {
	accessToken, expiresAt, err := s.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	return &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		ExpiresIn:    int(s.accessTokenTTL.Seconds()),
	}, nil
}

// GenerateAccessToken generates an access token for the user
func (s *jwtService) GenerateAccessToken(user *user_model.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTokenTTL)

	claims := &model.JWTClaims{
		UserID:   user.ID,
		ClinicID: user.ClinicID,
		Email:    user.Email,
		Role:     user.Role,
		Type:     model.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   user.ID.String(),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", time.Time{}, errors.ErrInternalInstance.WithError(err)
	}

	return tokenString, expiresAt, nil
}

// GenerateRefreshToken generates a refresh token for the user
func (s *jwtService) GenerateRefreshToken(user *user_model.User) (string, error) {
	expiresAt := time.Now().Add(s.refreshTokenTTL)

	claims := &model.JWTClaims{
		UserID:   user.ID,
		ClinicID: user.ClinicID,
		Email:    user.Email,
		Role:     user.Role,
		Type:     model.TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   user.ID.String(),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", errors.ErrInternalInstance.WithError(err)
	}

	return tokenString, nil
}

// ValidateAccessToken validates an access token and returns the claims
func (s *jwtService) ValidateAccessToken(tokenString string) (*model.JWTClaims, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.Type != model.TokenTypeAccess {
		return nil, errors.ErrTokenInvalidInstance.WithMessage("Invalid token type")
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns the claims
func (s *jwtService) ValidateRefreshToken(tokenString string) (*model.JWTClaims, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.Type != model.TokenTypeRefresh {
		return nil, errors.ErrTokenInvalidInstance.WithMessage("Invalid token type")
	}

	return claims, nil
}

// parseToken parses and validates a JWT token
func (s *jwtService) parseToken(tokenString string) (*model.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &model.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secret), nil
	})

	if err != nil {
		if err == jwt.ErrTokenExpired {
			return nil, errors.ErrTokenExpiredInstance
		}
		return nil, errors.ErrTokenInvalidInstance.WithError(err)
	}

	claims, ok := token.Claims.(*model.JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.ErrTokenInvalidInstance
	}

	return claims, nil
}
