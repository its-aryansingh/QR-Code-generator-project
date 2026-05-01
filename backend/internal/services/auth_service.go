package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailExists        = errors.New("email already registered")
	ErrUserNotFound       = errors.New("user not found")
)

type AuthService interface {
	Register(email, password string) (*models.User, error)
	Login(email, password string) (*TokenPair, error)
	GoogleLogin(email, name, avatarURL string) (*TokenPair, error)
	RefreshToken(refreshToken string) (*TokenPair, error)
	ValidateToken(tokenString string) (*jwt.Token, error)
	UpdateProfile(userID uuid.UUID, name, password string) error
	GetProfile(userID uuid.UUID) (*models.User, error)
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type authService struct {
	userRepo      repository.UserRepository
	jwtSecret     string
	jwtExpiry     time.Duration
	refreshExpiry time.Duration
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string, jwtExpiry, refreshExpiry time.Duration) AuthService {
	return &authService{
		userRepo:      userRepo,
		jwtSecret:     jwtSecret,
		jwtExpiry:     jwtExpiry,
		refreshExpiry: refreshExpiry,
	}
}

func (s *authService) Register(email, password string) (*models.User, error) {
	// Check if email already exists
	existingUser, _ := s.userRepo.FindByEmail(email)
	if existingUser != nil {
		return nil, ErrEmailExists
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
	}

	// Generate API key
	if err := user.GenerateAPIKey(); err != nil {
		return nil, err
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *authService) Login(email, password string) (*TokenPair, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate tokens
	return s.generateTokenPair(user.ID)
}

func (s *authService) GoogleLogin(email, name, avatarURL string) (*TokenPair, error) {
	// Try to find existing user
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		// User doesn't exist — create one with a random password hash
		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			return nil, err
		}
		randomPass := hex.EncodeToString(randomBytes)
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPass), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}

		user = &models.User{
			Email:        email,
			PasswordHash: string(hashedPassword),
			Name:         name,
			AvatarURL:    avatarURL,
		}

		// Generate API key
		if err := user.GenerateAPIKey(); err != nil {
			return nil, err
		}

		if err := s.userRepo.Create(user); err != nil {
			return nil, err
		}
	} else {
		// Update profile info from Google if changed
		updated := false
		if name != "" && user.Name != name {
			user.Name = name
			updated = true
		}
		if avatarURL != "" && user.AvatarURL != avatarURL {
			user.AvatarURL = avatarURL
			updated = true
		}
		if updated {
			_ = s.userRepo.Update(user)
		}
	}

	return s.generateTokenPair(user.ID)
}

func (s *authService) UpdateProfile(userID uuid.UUID, name, password string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	if name != "" {
		user.Name = name
	}

	if password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.PasswordHash = string(hashedPassword)
	}

	return s.userRepo.Update(user)
}

func (s *authService) GetProfile(userID uuid.UUID) (*models.User, error) {
	return s.userRepo.FindByID(userID)
}

func (s *authService) RefreshToken(refreshToken string) (*TokenPair, error) {
	token, err := s.ValidateToken(refreshToken)
	if err != nil || !token.Valid {
		return nil, ErrInvalidCredentials
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidCredentials
	}

	// Check if it's a refresh token
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, ErrInvalidCredentials
	}

	userIDStr, ok := claims["sub"].(string)
	if !ok {
		return nil, ErrInvalidCredentials
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Verify user still exists
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return nil, ErrUserNotFound
	}

	return s.generateTokenPair(userID)
}

func (s *authService) ValidateToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.jwtSecret), nil
	})
}

func (s *authService) generateTokenPair(userID uuid.UUID) (*TokenPair, error) {
	now := time.Now()

	// Access token
	accessClaims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "access",
		"iat":  now.Unix(),
		"exp":  now.Add(s.jwtExpiry).Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	// Refresh token
	refreshClaims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "refresh",
		"iat":  now.Unix(),
		"exp":  now.Add(s.refreshExpiry).Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int64(s.jwtExpiry.Seconds()),
	}, nil
}
