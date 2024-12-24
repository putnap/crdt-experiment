package crdt

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// OperationType enumerates possible changes
type OperationType string

const (
	OpInsert OperationType = "insert"
	OpDelete OperationType = "delete"
	OpCursor OperationType = "cursor" // for presence
)

// Operation represents a CRDT operation
// With multi-char "insert", value can contain multiple characters
type Operation struct {
	Type        OperationType `json:"type"`
	DocID       string        `json:"docId"`
	Position    int           `json:"position"`
	Value       string        `json:"value,omitempty"` // for insert
	OperationID string        `json:"operationId"`
	Source      string        `json:"source"`
	Timestamp   int64         `json:"timestamp"`
	CursorPos   int           `json:"cursorPos,omitempty"` // for OpCursor
	UserColor   string        `json:"userColor,omitempty"` // presence
}

// CRDT manages the text as a slice of runes/characters.
type CRDT struct {
	mu   sync.RWMutex
	text []rune
}

// NewCRDT constructs a new CRDT
func NewCRDT() *CRDT {
	return &CRDT{
		text: []rune{},
	}
}

// ApplyOperation applies an operation to the CRDT text
// This does not handle advanced tombstones or concurrency merges
// beyond single master concurrency, but is enough for a
// single leader approach or short-lived concurrency windows.
func (c *CRDT) ApplyOperation(op Operation) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch op.Type {
	case OpInsert:
		if op.Position < 0 {
			op.Position = 0
		}
		if op.Position > len(c.text) {
			op.Position = len(c.text)
		}
		insertRunes := []rune(op.Value)
		// Insert into the slice
		c.text = append(
			c.text[:op.Position],
			append(insertRunes, c.text[op.Position:]...)...,
		)

	case OpDelete:
		if op.Position < 0 || op.Position >= len(c.text) {
			return
		}
		deleteCount := len([]rune(op.Value))
		if op.Position+deleteCount > len(c.text) {
			deleteCount = len(c.text) - op.Position
		}
		c.text = append(c.text[:op.Position], c.text[op.Position+deleteCount:]...)
	case OpCursor:
		// No direct impact on text, so nothing to do here
		// but we keep for presence updates
	}
}

// GetText returns the CRDT text as a string
func (c *CRDT) GetText() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return string(c.text)
}

// Helper to create a new operation
func NewOperation(docID string, opType OperationType, position int, value string, source string) Operation {
	return Operation{
		Type:        opType,
		DocID:       docID,
		Position:    position,
		Value:       value,
		OperationID: uuid.NewString(),
		Timestamp:   time.Now().UnixNano(),
		Source:      source,
	}
}
