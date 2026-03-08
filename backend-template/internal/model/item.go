package model

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Item represents a simple data entity for demonstration purposes
type Item struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// ItemStore is a thread-safe in-memory store for items
type ItemStore struct {
	mu    sync.RWMutex
	items []Item
}

// NewItemStore creates a new ItemStore with sample data
func NewItemStore() *ItemStore {
	now := time.Now()
	return &ItemStore{
		items: []Item{
			{
				ID:          uuid.New().String(),
				TenantID:    "",
				Name:        "Sample Item 1",
				Description: "This is a sample item for demonstration",
				CreatedAt:   now,
			},
			{
				ID:          uuid.New().String(),
				TenantID:    "",
				Name:        "Sample Item 2",
				Description: "Another example item",
				CreatedAt:   now,
			},
			{
				ID:          uuid.New().String(),
				TenantID:    "",
				Name:        "Sample Item 3",
				Description: "A third sample item",
				CreatedAt:   now,
			},
		},
	}
}

// Add creates a new item and returns it
func (s *ItemStore) Add(name, description, tenantID string) Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := Item{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}

	s.items = append(s.items, item)
	return item
}

// GetAll returns all items
func (s *ItemStore) GetAll() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make([]Item, len(s.items))
	copy(result, s.items)
	return result
}

// GetByID finds an item by ID
func (s *ItemStore) GetByID(id string) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, item := range s.items {
		if item.ID == id {
			return item, true
		}
	}
	return Item{}, false
}

// GetByTenant returns all items for a specific tenant
func (s *ItemStore) GetByTenant(tenantID string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Item
	for _, item := range s.items {
		if item.TenantID == tenantID {
			result = append(result, item)
		}
	}
	return result
}
