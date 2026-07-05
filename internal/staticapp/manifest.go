package staticapp

// Manifest describes an exported live-server bundle. It is intentionally small:
// the SQLite schema is the runtime data contract, while the manifest tells the
// server where the database lives and which coarse features are packaged.
type Manifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	Kind          string          `json:"kind"`
	GeneratedAt   string          `json:"generatedAt"`
	DB            DBManifest      `json:"db"`
	Features      FeatureManifest `json:"features"`
	Repo          RepoManifest    `json:"repo"`
	Commits       CommitManifest  `json:"commits"`
	Runtime       RuntimeManifest `json:"runtime"`
}

type DBManifest struct {
	Path                 string            `json:"path"`
	SizeBytes            int64             `json:"sizeBytes"`
	SchemaVersion        int               `json:"schemaVersion"`
	HistorySchemaVersion string            `json:"historySchemaVersion,omitempty"`
	ReviewSchemaVersion  string            `json:"reviewSchemaVersion,omitempty"`
	SchemaVersions       map[string]string `json:"schemaVersions,omitempty"`
}

type FeatureManifest struct {
	CodebaseBrowser bool `json:"codebaseBrowser"`
	ReviewDocs      bool `json:"reviewDocs"`
	LLMDatabase     bool `json:"llmDatabase"`
}

type RepoManifest struct {
	RootLabel string `json:"rootLabel"`
}

type CommitManifest struct {
	Count  int    `json:"count"`
	Oldest string `json:"oldest"`
	Newest string `json:"newest"`
}

type RuntimeManifest struct {
	QueryEngine              string `json:"queryEngine"`
	RequiresStaticHTTPServer bool   `json:"requiresStaticHttpServer"`
	HasGoRuntimeServer       bool   `json:"hasGoRuntimeServer"`
}
