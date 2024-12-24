CREATE TABLE documents (
  doc_id    TEXT PRIMARY KEY,
  title     TEXT,
  owner_id  TEXT,
  created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE doc_snapshots (
  doc_id      TEXT,
  revision    BIGINT,
  content     TEXT,   -- full text snapshot
  created_at  TIMESTAMP DEFAULT now(),
  PRIMARY KEY(doc_id, revision)
);

CREATE TABLE doc_operations (
  doc_id      TEXT,
  operation_id TEXT PRIMARY KEY,
  op_type     TEXT,
  position    INT,
  value       TEXT,
  timestamp   BIGINT,
  source      TEXT, -- userID
  created_at  TIMESTAMP DEFAULT now()
);
