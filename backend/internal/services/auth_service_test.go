package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/models"
)

// Mock user repository for testing
type mockUserRepo struct {
	users map[string]*models.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[string]*models.User),
	}
}

func (m *mockUserRepo) Create(user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	m.users[user.Email] = user
	return nil
}

func (m *mockUserRepo) FindByEmail(email string) (*models.User, error) {
	if user, ok := m.users[email]; ok {
		return user, nil
	}
	return nil, ErrUserNotFound
}

func (m *mockUserRepo) FindByID(id uuid.UUID) (*models.User, error) {
	for _, user := range m.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, ErrUserNotFound
}

func (m *mockUserRepo) FindByAPIKey(apiKey string) (*models.User, error) {
	for _, user := range m.users {
		if user.APIKey == apiKey {
			return user, nil
		}
	}
	return nil, ErrUserNotFound
}

func (m *mockUserRepo) Update(user *models.User) error {
	if user.Email != "" {
		m.users[user.Email] = user
	}
	return nil
}

func TestAuthService_Register(t *testing.T) {
	repo := newMockUserRepo()
	service := NewAuthService(repo, "test-secret", 15*time.Minute, 7*24*time.Hour)

	tests := []struct {
		name      string
		email     string
		password  string
		wantErr   error
		setupFunc func()
	}{
		{
			name:     "successful registration",
			email:    "test@example.com",
			password: "password123",
			wantErr:  nil,
		},
		{
			name:     "duplicate email",
			email:    "duplicate@example.com",
			password: "password123",
			wantErr:  ErrEmailExists,
			setupFunc: func() {
				repo.users["duplicate@example.com"] = &models.User{
					ID:    uuid.New(),
					Email: "duplicate@example.com",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			user, err := service.Register(tt.email, tt.password)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Register() unexpected error = %v", err)
				return
			}

			if user.Email != tt.email {
				t.Errorf("Register() email = %v, want %v", user.Email, tt.email)
			}

			if user.PasswordHash == "" {
				t.Error("Register() password hash should not be empty")
			}

			if user.PasswordHash == tt.password {
				t.Error("Register() password should be hashed, not plain text")
			}
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	repo := newMockUserRepo()
	service := NewAuthService(repo, "test-secret", 15*time.Minute, 7*24*time.Hour)

	// Register a user first
	_, err := service.Register("login@example.com", "correctpassword")
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{
			name:     "successful login",
			email:    "login@example.com",
			password: "correctpassword",
			wantErr:  false,
		},
		{
			name:     "wrong password",
			email:    "login@example.com",
			password: "wrongpassword",
			wantErr:  true,
		},
		{
			name:     "non-existent user",
			email:    "nonexistent@example.com",
			password: "password",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := service.Login(tt.email, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Error("Login() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Login() unexpected error = %v", err)
				return
			}

			if tokens.AccessToken == "" {
				t.Error("Login() access token should not be empty")
			}

			if tokens.RefreshToken == "" {
				t.Error("Login() refresh token should not be empty")
			}

			if tokens.ExpiresIn <= 0 {
				t.Error("Login() expires_in should be positive")
			}
		})
	}
}

func TestAuthService_ValidateToken(t *testing.T) {
	repo := newMockUserRepo()
	service := NewAuthService(repo, "test-secret", 15*time.Minute, 7*24*time.Hour)

	// Register and login
	_, _ = service.Register("validate@example.com", "password123")
	tokens, _ := service.Login("validate@example.com", "password123")

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token",
			token:   tokens.AccessToken,
			wantErr: false,
		},
		{
			name:    "invalid token",
			token:   "invalid.token.here",
			wantErr: true,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := service.ValidateToken(tt.token)

			if tt.wantErr {
				if err == nil && token.Valid {
					t.Error("ValidateToken() expected invalid token")
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateToken() unexpected error = %v", err)
			}

			if !token.Valid {
				t.Error("ValidateToken() token should be valid")
			}
		})
	}
}
