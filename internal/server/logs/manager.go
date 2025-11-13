package logs

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Entry represents an application log entry intended for UI consumption.
type Entry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Source    string                 `json:"source"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Manager keeps a bounded in-memory history of log entries and notifies subscribers.
type Manager struct {
	mu          sync.RWMutex
	maxEntries  int
	entries     []Entry
	subscribers map[chan Entry]struct{}
	subscribeMu sync.Mutex
}

// NewManager creates a new log manager with the provided maximum in-memory history.
func NewManager(maxEntries int) *Manager {
	if maxEntries <= 0 {
		maxEntries = 500
	}
	return &Manager{
		maxEntries:  maxEntries,
		entries:     make([]Entry, 0, maxEntries),
		subscribers: make(map[chan Entry]struct{}),
	}
}

// Add records a new entry and broadcasts it to subscribers.
func (m *Manager) Add(entry Entry) Entry {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	m.entries = append(m.entries, entry)
	if len(m.entries) > m.maxEntries {
		trim := len(m.entries) - m.maxEntries
		m.entries = m.entries[trim:]
	}

	m.broadcast(entry)
	return entry
}

// List returns up to limit entries occurring after the provided ID (exclusive).
func (m *Manager) List(afterID string, limit int) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > m.maxEntries {
		limit = m.maxEntries
	}

	startIdx := 0
	if afterID != "" {
		for i := len(m.entries) - 1; i >= 0; i-- {
			if m.entries[i].ID == afterID {
				startIdx = i + 1
				break
			}
		}
	}

	if startIdx >= len(m.entries) {
		return []Entry{}
	}

	endIdx := startIdx + limit
	if endIdx > len(m.entries) {
		endIdx = len(m.entries)
	}

	out := make([]Entry, endIdx-startIdx)
	copy(out, m.entries[startIdx:endIdx])
	return out
}

// Subscribe returns a channel that receives live log entries and an unsubscribe function.
func (m *Manager) Subscribe() (chan Entry, func()) {
	ch := make(chan Entry, 100)
	m.subscribeMu.Lock()
	m.subscribers[ch] = struct{}{}
	m.subscribeMu.Unlock()

	unsub := func() {
		m.subscribeMu.Lock()
		if _, ok := m.subscribers[ch]; ok {
			delete(m.subscribers, ch)
			close(ch)
		}
		m.subscribeMu.Unlock()
	}
	return ch, unsub
}

func (m *Manager) broadcast(entry Entry) {
	m.subscribeMu.Lock()
	defer m.subscribeMu.Unlock()
	for ch := range m.subscribers {
		select {
		case ch <- entry:
		default:
			// drop if subscriber is slow
		}
	}
}
