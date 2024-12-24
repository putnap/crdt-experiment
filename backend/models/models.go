package models

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"

	"crdt-api/crdt"
)

var db *sql.DB

func DB() *sql.DB {
	return db
}

func InitDB(migrations fs.FS) error {
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASS")
	name := os.Getenv("DB_NAME")

	connStr := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, pass, host, name)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migrate driver: %w", err)
	}

	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create source: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate new: %s", err)
	}

	if err := migrator.Up(); err != nil {
		switch err {
		case migrate.ErrNoChange:
			log.Println("Migration: There were no changes to apply.")
		default:
			return fmt.Errorf("failed to migrate database: %w", err)
		}
	} else {
		log.Println("Migration: Applied successfully.")
	}

	return db.Ping()
}

// InsertOperation adds an operation to doc_operations table
func InsertOperation(op crdt.Operation) error {
	_, err := db.Exec(`
		INSERT INTO doc_operations (doc_id, operation_id, op_type, position, value, timestamp, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, op.DocID, op.OperationID, op.Type, op.Position, op.Value, op.Timestamp, op.Source)
	return err
}

// InsertSnapshot saves a snapshot of the doc
func InsertSnapshot(docID string, revision int64, content string) error {
	_, err := db.Exec(`
		INSERT INTO doc_snapshots (doc_id, revision, content)
		VALUES ($1, $2, $3)
	`, docID, revision, content)
	return err
}

var ErrNotFound = errors.New("not found")

type SnapshotRecord struct {
	DocID    string
	Revision int64
	Content  string
}

func GetLatestSnapshot(docID string) (*SnapshotRecord, error) {
	row := db.QueryRow(`
		SELECT doc_id, revision, content
		FROM doc_snapshots
		WHERE doc_id = $1
		ORDER BY revision DESC
		LIMIT 1
	`, docID)

	var rec SnapshotRecord
	if err := row.Scan(&rec.DocID, &rec.Revision, &rec.Content); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

func (sr *SnapshotRecord) ToCRDT() *crdt.CRDT {
	c := crdt.NewCRDT()
	// Insert entire content in one go
	// Or chunk it, but here we assume a single multi-char insert
	op := crdt.Operation{
		Type:      crdt.OpInsert,
		DocID:     sr.DocID,
		Position:  0,
		Value:     sr.Content,
		Source:    "snapshot",
		Timestamp: sr.Revision,
	}
	c.ApplyOperation(op)
	return c
}

// Operations table record
type OperationRecord struct {
	DocID       string
	OperationID string
	OpType      string
	Position    int
	Value       string
	Timestamp   int64
	Source      string
}

func (o *OperationRecord) ToCRDTOperation() crdt.Operation {
	return crdt.Operation{
		Type:        crdt.OperationType(o.OpType),
		DocID:       o.DocID,
		Position:    o.Position,
		Value:       o.Value,
		OperationID: o.OperationID,
		Timestamp:   o.Timestamp,
		Source:      o.Source,
	}
}

func GetOperationsSince(docID string, minRevision int64) ([]OperationRecord, error) {
	rows, err := db.Query(`
		SELECT doc_id, operation_id, op_type, position, value, timestamp, source
		FROM doc_operations
		WHERE doc_id = $1 AND timestamp > $2
		ORDER BY timestamp ASC
	`, docID, minRevision)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ops []OperationRecord
	for rows.Next() {
		var or OperationRecord
		if err := rows.Scan(
			&or.DocID,
			&or.OperationID,
			&or.OpType,
			&or.Position,
			&or.Value,
			&or.Timestamp,
			&or.Source,
		); err != nil {
			return nil, err
		}
		ops = append(ops, or)
	}
	return ops, nil
}
