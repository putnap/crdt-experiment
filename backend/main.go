package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"crdt-api/crdt"
	"crdt-api/docmanager"
	"crdt-api/models"
)

//go:embed migrations/*.sql
var migrations embed.FS

var dm = docmanager.NewDocManager()

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// In production, use a stricter policy
		return true
	},
}

func main() {
	if err := models.InitDB(migrations); err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/docs", createDocHandler).Methods("POST")
	r.HandleFunc("/ws/{docID}", wsHandler).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

type createDocRequest struct {
	DocID   string `json:"docId"`
	Title   string `json:"title"`
	OwnerID string `json:"ownerId"` // For RBAC
}

func createDocHandler(w http.ResponseWriter, r *http.Request) {
	var req createDocRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.DocID == "" {
		http.Error(w, "docId is required", http.StatusBadRequest)
		return
	}
	// Insert doc metadata into DB
	_, err := models.DB().Exec(`
		INSERT INTO documents (doc_id, title, owner_id) 
		VALUES ($1, $2, $3)
	`, req.DocID, req.Title, req.OwnerID)
	if err != nil {
		http.Error(w, "failed to create document", http.StatusInternalServerError)
		return
	}

	// Make sure doc session is created
	if _, err := dm.EnsureSession(req.DocID); err != nil {
		http.Error(w, "failed to init doc session", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"docId":"%s"}`, req.DocID)))
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	docID := vars["docID"]

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		userID = "anon-" + r.RemoteAddr
	}
	userColor := r.URL.Query().Get("color")
	if userColor == "" {
		userColor = "#" + randomColor() // or random pick
	}

	ds, err := dm.EnsureSession(docID)
	if err != nil {
		http.Error(w, "failed to load doc session", http.StatusInternalServerError)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	ds.AddConnection(conn)
	defer ds.RemoveConnection(conn)

	// Send "init" message: current doc text + presence
	initMsg := map[string]interface{}{
		"type":     "init",
		"docId":    docID,
		"text":     ds.CRDT.GetText(),
		"presence": ds.Presence,
	}
	if err := conn.WriteJSON(initMsg); err != nil {
		log.Println("Write init msg error:", err)
		return
	}

	// Listen for incoming ops
	for {
		var op crdt.Operation
		if err := conn.ReadJSON(&op); err != nil {
			log.Printf("WebSocket read error for doc %s: %v\n", docID, err)
			return
		}

		// Force docID, userColor, source from the session context
		op.DocID = docID
		op.Source = userID
		if op.Type == crdt.OpCursor {
			op.UserColor = userColor
		}

		ds.ApplyAndBroadcast(op, conn)
	}
}

func randomColor() string {
	// A quick hack to generate a random color
	return fmt.Sprintf("%06x", 0xffffff&(uint32)(newRandomInt()))
}

func newRandomInt() int64 {
	// You can do something more robust or use crypt/rand
	return 123456 // placeholder
}
