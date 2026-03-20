CREATE TABLE IF NOT EXISTS blobs (
    id TEXT NOT NULL,
    function_id TEXT,
    name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    content BLOB NOT NULL,
    is_public INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
    PRIMARY KEY (id, function_id)
);

CREATE INDEX IF NOT EXISTS idx_blobs_function_id ON blobs(function_id);