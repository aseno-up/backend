package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/auth/model"
	user_model "github.com/aseno/backend/pkg/user/model"
)

// TokenContext holds the data needed to generate JWT tokens.
// This decouples token generation from the User struct, allowing
// clinic_id and role to come from clinic_members instead of users table.
type TokenContext struct {
	UserID         uuid.UUID
	Email          string
	ClinicID       uuid.UUID
	Role           user_model.UserRole
	ProfessionalID *uuid.UUID
}

// JWTService defines the interface for JWT operations
type JWTService interface {
	GenerateTokenPair(tc *TokenContext) (*model.TokenPair, error)
	GenerateAccessToken(tc *TokenContext) (string, time.Time, error)
	GenerateRefreshToken(tc *TokenContext) (string, error)
	GenerateSSEToken(userID, clinicID uuid.UUID) (string, error)
	ValidateAccessToken(tokenString string) (*model.JWTClaims, error)
	ValidateRefreshToken(tokenString string) (*model.JWTClaims, error)
	ValidateSSEToken(tokenString string) (*model.JWTClaims, error)
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
		issuer:          "aseno-clinic",
	}
}

// GenerateTokenPair generates both access and refresh tokens
func (s *jwtService) GenerateTokenPair(tc *TokenContext) (*model.TokenPair, error) {
	accessToken, expiresAt, err := s.GenerateAccessToken(tc)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.GenerateRefreshToken(tc)
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

// GenerateAccessToken generates an access token
func (s *jwtService) GenerateAccessToken(tc *TokenContext) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTokenTTL)

	claims := &model.JWTClaims{
		UserID:         tc.UserID,
		ClinicID:       tc.ClinicID,
		Email:          tc.Email,
		Role:           tc.Role,
		ProfessionalID: tc.ProfessionalID,
		Type:           model.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   tc.UserID.String(),
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

// GenerateRefreshToken generates a refresh token
func (s *jwtService) GenerateRefreshToken(tc *TokenContext) (string, error) {
	expiresAt := time.Now().Add(s.refreshTokenTTL)

	claims := &model.JWTClaims{
		UserID:         tc.UserID,
		ClinicID:       tc.ClinicID,
		Email:          tc.Email,
		Role:           tc.Role,
		ProfessionalID: tc.ProfessionalID,
		Type:           model.TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   tc.UserID.String(),
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

// GenerateSSEToken generates a short-lived token for SSE connections (30 seconds)
func (s *jwtService) GenerateSSEToken(userID, clinicID uuid.UUID) (string, error) {
	expiresAt := time.Now().Add(30 * time.Second)

	claims := &model.JWTClaims{
		UserID:   userID,
		ClinicID: clinicID,
		Type:     model.TokenTypeSSE,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   userID.String(),
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

// ValidateSSEToken validates an SSE token and returns the claims
func (s *jwtService) ValidateSSEToken(tokenString string) (*model.JWTClaims, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.Type != model.TokenTypeSSE {
		return nil, errors.ErrTokenInvalidInstance.WithMessage("Invalid token type")
	}

	return claims, nil
}
