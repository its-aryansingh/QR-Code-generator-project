package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/models"
)

// Mock QR repository for testing
type mockQRRepo struct {
	records map[uuid.UUID]*models.QRRecord
}

func newMockQRRepo() *mockQRRepo {
	return &mockQRRepo{
		records: make(map[uuid.UUID]*models.QRRecord),
	}
}

func (m *mockQRRepo) Create(record *models.QRRecord) error {
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	record.CreatedAt = time.Now()
	m.records[record.ID] = record
	return nil
}

func (m *mockQRRepo) FindByUserID(userID uuid.UUID, limit, offset int) ([]models.QRRecord, error) {
	var result []models.QRRecord
	for _, record := range m.records {
		if record.UserID == userID {
			result = append(result, *record)
		}
	}
	// Apply pagination
	if offset >= len(result) {
		return []models.QRRecord{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (m *mockQRRepo) FindByID(id uuid.UUID) (*models.QRRecord, error) {
	if record, ok := m.records[id]; ok {
		return record, nil
	}
	return nil, ErrQRNotFound
}

func (m *mockQRRepo) CountByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	for _, record := range m.records {
		if record.UserID == userID {
			count++
		}
	}
	return count, nil
}

func TestQRService_Generate(t *testing.T) {
	repo := newMockQRRepo()
	service := NewQRService(repo)
	userID := uuid.New()

	tests := []struct {
		name    string
		userID  uuid.UUID
		content string
		qrType  string
		size    int
		wantErr error
	}{
		{
			name:    "successful generation with default size",
			userID:  userID,
			content: "https://example.com",
			qrType:  "url",
			size:    256,
			wantErr: nil,
		},
		{
			name:    "successful generation with custom size",
			userID:  userID,
			content: "Hello World",
			qrType:  "text",
			size:    512,
			wantErr: nil,
		},
		{
			name:    "empty content",
			userID:  userID,
			content: "",
			qrType:  "url",
			size:    256,
			wantErr: ErrInvalidContent,
		},
		{
			name:    "size too small",
			userID:  userID,
			content: "test",
			qrType:  "url",
			size:    32,
			wantErr: ErrInvalidSize,
		},
		{
			name:    "size too large",
			userID:  userID,
			content: "test",
			qrType:  "url",
			size:    4096,
			wantErr: ErrInvalidSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.Generate(tt.userID, tt.content, tt.qrType, tt.size)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Generate() unexpected error = %v", err)
				return
			}

			if result.Record == nil {
				t.Error("Generate() record should not be nil")
				return
			}

			if result.Record.Content != tt.content {
				t.Errorf("Generate() content = %v, want %v", result.Record.Content, tt.content)
			}

			if result.Record.Size != tt.size {
				t.Errorf("Generate() size = %v, want %v", result.Record.Size, tt.size)
			}

			if result.QRBase64 == "" {
				t.Error("Generate() QRBase64 should not be empty")
			}

			// Check that QR is a valid base64 data URI
			expectedPrefix := "data:image/png;base64,"
			if len(result.QRBase64) < len(expectedPrefix) || result.QRBase64[:len(expectedPrefix)] != expectedPrefix {
				t.Errorf("Generate() QRBase64 should start with %s", expectedPrefix)
			}
		})
	}
}

func TestQRService_GetHistory(t *testing.T) {
	repo := newMockQRRepo()
	service := NewQRService(repo)
	userID := uuid.New()
	otherUserID := uuid.New()

	// Create some records for the user
	for i := 0; i < 5; i++ {
		_, _ = service.Generate(userID, "https://example.com/"+string(rune('a'+i)), "url", 256)
	}
	// Create records for another user
	_, _ = service.Generate(otherUserID, "https://other.com", "url", 256)

	tests := []struct {
		name          string
		userID        uuid.UUID
		page          int
		pageSize      int
		wantTotal     int64
		wantPageCount int
	}{
		{
			name:          "get all records",
			userID:        userID,
			page:          1,
			pageSize:      10,
			wantTotal:     5,
			wantPageCount: 5,
		},
		{
			name:          "pagination",
			userID:        userID,
			page:          1,
			pageSize:      2,
			wantTotal:     5,
			wantPageCount: 2,
		},
		{
			name:          "empty result for other user",
			userID:        uuid.New(),
			page:          1,
			pageSize:      10,
			wantTotal:     0,
			wantPageCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetHistory(tt.userID, tt.page, tt.pageSize)

			if err != nil {
				t.Errorf("GetHistory() unexpected error = %v", err)
				return
			}

			if result.Total != tt.wantTotal {
				t.Errorf("GetHistory() total = %v, want %v", result.Total, tt.wantTotal)
			}

			if len(result.Records) != tt.wantPageCount {
				t.Errorf("GetHistory() records count = %v, want %v", len(result.Records), tt.wantPageCount)
			}
		})
	}
}

func TestQRService_GetByID(t *testing.T) {
	repo := newMockQRRepo()
	service := NewQRService(repo)
	userID := uuid.New()
	otherUserID := uuid.New()

	// Create a record
	result, _ := service.Generate(userID, "https://example.com", "url", 256)
	recordID := result.Record.ID

	tests := []struct {
		name    string
		id      uuid.UUID
		userID  uuid.UUID
		wantErr error
	}{
		{
			name:    "get existing record",
			id:      recordID,
			userID:  userID,
			wantErr: nil,
		},
		{
			name:    "non-existent record",
			id:      uuid.New(),
			userID:  userID,
			wantErr: ErrQRNotFound,
		},
		{
			name:    "record owned by other user",
			id:      recordID,
			userID:  otherUserID,
			wantErr: ErrQRNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := service.GetByID(tt.id, tt.userID)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("GetByID() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("GetByID() unexpected error = %v", err)
				return
			}

			if record.ID != tt.id {
				t.Errorf("GetByID() id = %v, want %v", record.ID, tt.id)
			}
		})
	}
}
