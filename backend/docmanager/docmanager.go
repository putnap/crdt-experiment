package docmanager

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"crdt-api/crdt"
	"crdt-api/models"
)

type PresenceInfo struct {
	UserID    string
	UserColor string
	CursorPos int
}

type DocumentSession struct {
	DocID       string
	CRDT        *crdt.CRDT
	Connections map[*websocket.Conn]bool
	Presence    map[string]*PresenceInfo // userID -> presence
	mu          sync.RWMutex
}

func NewDocumentSession(docID string) *DocumentSession {
	return &DocumentSession{
		DocID:       docID,
		CRDT:        crdt.NewCRDT(),
		Connections: make(map[*websocket.Conn]bool),
		Presence:    make(map[string]*PresenceInfo),
	}
}

func (ds *DocumentSession) AddConnection(conn *websocket.Conn) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.Connections[conn] = true
}

func (ds *DocumentSession) RemoveConnection(conn *websocket.Conn) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.Connections, conn)
}

// BroadcastOperation sends the operation to all clients *except* sender
func (ds *DocumentSession) BroadcastOperation(operation crdt.Operation, sender *websocket.Conn) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	for conn := range ds.Connections {
		if conn == sender {
			continue
		}
		if err := conn.WriteJSON(operation); err != nil {
			log.Println("Error broadcasting to client:", err)
		}
	}
}

// ApplyAndBroadcast applies the op to CRDT, stores it in DB, then broadcasts
func (ds *DocumentSession) ApplyAndBroadcast(op crdt.Operation, sender *websocket.Conn) {
	// 1. Apply to CRDT
	ds.CRDT.ApplyOperation(op)

	// 2. Update presence if it’s a cursor operation
	if op.Type == crdt.OpCursor {
		ds.mu.Lock()
		ds.Presence[op.Source] = &PresenceInfo{
			UserID:    op.Source,
			UserColor: op.UserColor,
			CursorPos: op.CursorPos,
		}
		ds.mu.Unlock()
	}

	// 3. Persist operation in DB
	go func() {
		if err := models.InsertOperation(op); err != nil {
			log.Println("Failed to store operation:", err)
		}
	}()

	// 4. Broadcast to others
	ds.BroadcastOperation(op, sender)
}

// DocManager manages multiple DocumentSessions
type DocManager struct {
	mu   sync.RWMutex
	docs map[string]*DocumentSession
}

func NewDocManager() *DocManager {
	return &DocManager{
		docs: make(map[string]*DocumentSession),
	}
}

// EnsureSession loads from DB if not present in memory
func (dm *DocManager) EnsureSession(docID string) (*DocumentSession, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if ds, exists := dm.docs[docID]; exists {
		return ds, nil
	}

	// If not in memory, create a new session
	ds := NewDocumentSession(docID)
	// Load from DB: 1) find latest snapshot, 2) replay ops
	err := loadDocumentFromDB(ds)
	if err != nil {
		return nil, err
	}
	dm.docs[docID] = ds
	return ds, nil
}

func loadDocumentFromDB(ds *DocumentSession) error {
	// 1. Query latest snapshot from doc_snapshots
	snapshot, err := models.GetLatestSnapshot(ds.DocID)
	if err != nil {
		if err == models.ErrNotFound {
			// no snapshot yet, that’s okay
			return nil
		}
		return err
	}
	if snapshot != nil {
		// Rebuild CRDT from snapshot
		ds.CRDT = snapshot.ToCRDT()
	}

	// 2. Query operations after snapshot revision
	ops, err := models.GetOperationsSince(ds.DocID, snapshot.Revision)
	if err != nil {
		return err
	}
	for _, op := range ops {
		ds.CRDT.ApplyOperation(op.ToCRDTOperation())
	}

	return nil
}

// Periodic snapshot (for large docs or set intervals)
func (dm *DocManager) TakeSnapshot(docID string) error {
	dm.mu.RLock()
	ds, ok := dm.docs[docID]
	dm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("doc session not found for %s", docID)
	}

	text := ds.CRDT.GetText()
	revision := time.Now().UnixNano()

	// store snapshot
	return models.InsertSnapshot(ds.DocID, revision, text)
}
