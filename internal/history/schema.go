package history

const DropSchemaSQL = `
DROP VIEW IF EXISTS symbol_history;
DROP VIEW IF EXISTS snapshot_refs;
DROP VIEW IF EXISTS snapshot_symbols;
DROP VIEW IF EXISTS snapshot_files;
DROP VIEW IF EXISTS snapshot_packages;
DROP TABLE IF EXISTS commit_refs;
DROP TABLE IF EXISTS commit_symbols;
DROP TABLE IF EXISTS commit_files;
DROP TABLE IF EXISTS commit_packages;
DROP TABLE IF EXISTS schema_info;
DROP TABLE IF EXISTS ref_versions;
DROP TABLE IF EXISTS symbols;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS packages;
DROP TABLE IF EXISTS file_contents;
DROP TABLE IF EXISTS commits;
`

const CreateSchemaSQL = `
-- ----------------------------------------
-- Core entity tables (stored once)
-- ----------------------------------------

CREATE TABLE commits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hash TEXT NOT NULL UNIQUE,
    short_hash TEXT NOT NULL,
    message TEXT NOT NULL,
    author_name TEXT NOT NULL,
    author_email TEXT NOT NULL,
    author_time INTEGER NOT NULL,
    parent_hashes TEXT NOT NULL DEFAULT '[]',
    tree_hash TEXT NOT NULL DEFAULT '',
    indexed_at INTEGER NOT NULL DEFAULT 0,
    sequence INTEGER NOT NULL DEFAULT 0,
    branch TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX idx_commits_hash ON commits(hash);
CREATE INDEX idx_commits_author_time ON commits(author_time);
CREATE INDEX idx_commits_sequence ON commits(sequence);

-- Packages: one row per unique package across all commits.
CREATE TABLE packages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    stable_id TEXT NOT NULL,          -- "pkg:github.com/.../name"
    import_path TEXT NOT NULL,
    name TEXT NOT NULL,
    doc TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT 'go',
    UNIQUE(stable_id)
);

-- Files: one row per unique (stable_id, sha256) version.
CREATE TABLE files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    stable_id TEXT NOT NULL,          -- "file:path/to/file.go"
    path TEXT NOT NULL,
    package_id INTEGER NOT NULL REFERENCES packages(id),
    size INTEGER NOT NULL DEFAULT 0,
    line_count INTEGER NOT NULL DEFAULT 0,
    sha256 TEXT NOT NULL,
    language TEXT NOT NULL DEFAULT 'go',
    build_tags_json TEXT NOT NULL DEFAULT '[]',
    UNIQUE(stable_id, sha256)
);
CREATE INDEX idx_files_sha ON files(sha256);

-- Symbols: one row per unique (stable_id, body_hash) version.
CREATE TABLE symbols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    stable_id TEXT NOT NULL,          -- "sym:.../pkg.func.Name"
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    package_id INTEGER NOT NULL REFERENCES packages(id),
    file_id INTEGER NOT NULL REFERENCES files(id),
    start_line INTEGER NOT NULL DEFAULT 0,
    start_col INTEGER NOT NULL DEFAULT 0,
    end_line INTEGER NOT NULL DEFAULT 0,
    end_col INTEGER NOT NULL DEFAULT 0,
    start_offset INTEGER NOT NULL DEFAULT 0,
    end_offset INTEGER NOT NULL DEFAULT 0,
    doc TEXT NOT NULL DEFAULT '',
    signature TEXT NOT NULL DEFAULT '',
    receiver_type TEXT NOT NULL DEFAULT '',
    receiver_pointer INTEGER NOT NULL DEFAULT 0,
    exported INTEGER NOT NULL DEFAULT 0,
    language TEXT NOT NULL DEFAULT 'go',
    type_params_json TEXT NOT NULL DEFAULT '[]',
    tags_json TEXT NOT NULL DEFAULT '[]',
    body_hash TEXT NOT NULL DEFAULT '',
    UNIQUE(stable_id, body_hash)
);
CREATE INDEX idx_symbols_name ON symbols(name);
CREATE INDEX idx_symbols_kind ON symbols(kind);
CREATE INDEX idx_symbols_body ON symbols(body_hash);

-- Ref versions: one row per unique (from_symbol, to_stable_id, kind, file) combo.
-- Multiple locations for the same combo are stored as a JSON array.
CREATE TABLE ref_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_symbol_id INTEGER NOT NULL REFERENCES symbols(id),
    to_stable_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    file_id INTEGER NOT NULL REFERENCES files(id),
    locations_json TEXT NOT NULL DEFAULT '[]',
    UNIQUE(from_symbol_id, to_stable_id, kind, file_id)
);
CREATE INDEX idx_ref_from ON ref_versions(from_symbol_id);
CREATE INDEX idx_ref_to ON ref_versions(to_stable_id);

-- File contents: already deduplicated by hash.
CREATE TABLE file_contents (
    content_hash TEXT PRIMARY KEY,
    content BLOB NOT NULL
);

CREATE TABLE schema_info (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT INTO schema_info(key, value) VALUES
    ('history_schema_version', '2'),
    ('review_schema_version', '2');

-- ----------------------------------------
-- Commit mapping tables (which version is in which commit)
-- ----------------------------------------

CREATE TABLE commit_packages (
    commit_id INTEGER NOT NULL REFERENCES commits(id),
    package_id INTEGER NOT NULL REFERENCES packages(id),
    PRIMARY KEY (commit_id, package_id)
) WITHOUT ROWID;

CREATE TABLE commit_files (
    commit_id INTEGER NOT NULL REFERENCES commits(id),
    file_id INTEGER NOT NULL REFERENCES files(id),
    PRIMARY KEY (commit_id, file_id)
) WITHOUT ROWID;

CREATE TABLE commit_symbols (
    commit_id INTEGER NOT NULL REFERENCES commits(id),
    symbol_id INTEGER NOT NULL REFERENCES symbols(id),
    PRIMARY KEY (commit_id, symbol_id)
) WITHOUT ROWID;

CREATE TABLE commit_refs (
    commit_id INTEGER NOT NULL REFERENCES commits(id),
    ref_version_id INTEGER NOT NULL REFERENCES ref_versions(id),
    PRIMARY KEY (commit_id, ref_version_id)
) WITHOUT ROWID;
`

const CreateViewsSQL = `
-- Compatibility views that recreate the old snapshot_* table shapes.
-- The React browser queries these views directly; the underlying
-- normalized tables are an implementation detail.

CREATE VIEW snapshot_packages AS
SELECT
    c.hash AS commit_hash,
    p.stable_id AS id,
    p.import_path,
    p.name,
    p.doc,
    p.language
FROM commit_packages cp
JOIN commits c ON c.id = cp.commit_id
JOIN packages p ON p.id = cp.package_id;

CREATE VIEW snapshot_files AS
SELECT
    c.hash AS commit_hash,
    f.stable_id AS id,
    f.path,
    p.stable_id AS package_id,
    f.size,
    f.line_count,
    f.sha256,
    f.language,
    f.build_tags_json
FROM commit_files cf
JOIN commits c ON c.id = cf.commit_id
JOIN files f ON f.id = cf.file_id
JOIN packages p ON p.id = f.package_id;

CREATE VIEW snapshot_symbols AS
SELECT
    c.hash AS commit_hash,
    s.stable_id AS id,
    s.kind,
    s.name,
    p.stable_id AS package_id,
    f.stable_id AS file_id,
    s.start_line, s.start_col, s.end_line, s.end_col,
    s.start_offset, s.end_offset,
    s.doc, s.signature,
    s.receiver_type, s.receiver_pointer,
    s.exported, s.language,
    s.type_params_json, s.tags_json,
    s.body_hash
FROM commit_symbols cs
JOIN commits c ON c.id = cs.commit_id
JOIN symbols s ON s.id = cs.symbol_id
JOIN packages p ON p.id = s.package_id
JOIN files f ON f.id = s.file_id;

-- snapshot_refs expands ref_versions.locations_json into one row per location.
-- We use a subquery with row_number to assign sequential IDs.
CREATE VIEW snapshot_refs AS
SELECT
    c.hash AS commit_hash,
    row_number() OVER (PARTITION BY c.id ORDER BY rv.id, j.key) AS id,
    s.stable_id AS from_symbol_id,
    rv.to_stable_id AS to_symbol_id,
    rv.kind,
    f.stable_id AS file_id,
    json_extract(j.value, '$.start_line') AS start_line,
    json_extract(j.value, '$.start_col') AS start_col,
    json_extract(j.value, '$.end_line') AS end_line,
    json_extract(j.value, '$.end_col') AS end_col,
    json_extract(j.value, '$.start_offset') AS start_offset,
    json_extract(j.value, '$.end_offset') AS end_offset
FROM commit_refs cr
JOIN commits c ON c.id = cr.commit_id
JOIN ref_versions rv ON rv.id = cr.ref_version_id
JOIN symbols s ON s.id = rv.from_symbol_id
JOIN files f ON f.id = rv.file_id,
    json_each(rv.locations_json) AS j;

CREATE VIEW symbol_history AS
SELECT
    s.stable_id AS symbol_id,
    s.name,
    s.kind,
    p.stable_id AS package_id,
    c.hash AS commit_hash,
    c.short_hash,
    c.message AS commit_message,
    c.author_time,
    s.body_hash,
    s.start_line,
    s.end_line,
    s.signature,
    f.stable_id AS file_id
FROM commit_symbols cs
JOIN commits c ON c.id = cs.commit_id
JOIN symbols s ON s.id = cs.symbol_id
JOIN packages p ON p.id = s.package_id
JOIN files f ON f.id = s.file_id;
`
