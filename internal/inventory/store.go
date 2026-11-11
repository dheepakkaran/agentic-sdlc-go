// Package inventory is a small network device inventory: the service the
// AI-SDLC agents work on. It is deliberately plain so agent changes are easy
// to review.
package inventory

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// Device is one network element (router, switch, OLT) in the inventory.
type Device struct {
	ID        string    `json:"id"`
	Hostname  string    `json:"hostname"`
	Role      string    `json:"role"` // e.g. "router", "switch", "olt"
	Site      string    `json:"site"`
	MgmtIP    string    `json:"mgmt_ip"`
	Status    string    `json:"status"` // "up" or "down"
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	ErrNotFound = errors.New("device not found")
	ErrExists   = errors.New("device already exists")
	ErrInvalid  = errors.New("invalid device")
)

// Store is an in-memory, concurrency-safe device store.
type Store struct {
	mu      sync.RWMutex
	devices map[string]Device
	now     func() time.Time
}

func NewStore() *Store {
	return &Store{devices: map[string]Device{}, now: time.Now}
}

func validate(d Device) error {
	if d.ID == "" || d.Hostname == "" {
		return ErrInvalid
	}
	if d.Status != "" && d.Status != "up" && d.Status != "down" {
		return ErrInvalid
	}
	return nil
}

func (s *Store) Create(d Device) (Device, error) {
	if err := validate(d); err != nil {
		return Device{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.devices[d.ID]; ok {
		return Device{}, ErrExists
	}
	if d.Status == "" {
		d.Status = "up"
	}
	d.UpdatedAt = s.now().UTC()
	s.devices[d.ID] = d
	return d, nil
}

func (s *Store) Get(id string) (Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[id]
	if !ok {
		return Device{}, ErrNotFound
	}
	return d, nil
}

// List returns all devices sorted by ID.
func (s *Store) List() []Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Device, 0, len(s.devices))
	for _, d := range s.devices {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.devices[id]; !ok {
		return ErrNotFound
	}
	delete(s.devices, id)
	return nil
}

// SetStatus sets only the status and updated_at of an existing device.
func (s *Store) SetStatus(id, status string) (Device, error) {
	if status != "up" && status != "down" {
		return Device{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[id]
	if !ok {
		return Device{}, ErrNotFound
	}
	d.Status = status
	d.UpdatedAt = s.now().UTC()
	s.devices[id] = d
	return d, nil
}
