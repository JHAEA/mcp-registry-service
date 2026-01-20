package domain

// ServerResponse wraps a server with metadata for API responses
type ServerResponse struct {
	Server ServerJSON  `json:"server"`
	Meta   *ServerMeta `json:"_meta,omitempty"`
}

// ServerListResponse represents a paginated list of servers
type ServerListResponse struct {
	Servers  []ServerResponse `json:"servers"`
	Metadata ListMetadata     `json:"metadata"`
}

// ListMetadata contains pagination metadata
type ListMetadata struct {
	NextCursor string `json:"nextCursor,omitempty"`
	Count      int    `json:"count"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status       string      `json:"status"`
	RepoURL      string      `json:"repo_url"`
	Branch       string      `json:"branch"`
	CommitSHA    string      `json:"commit_sha"`
	LastSyncAt   string      `json:"last_sync_at"`
	IndexStatus  string      `json:"index_status"`
	ServerCount  int         `json:"server_count"`
	CacheStats   *CacheStats `json:"cache_stats,omitempty"`
}

// CacheStats contains cache statistics
type CacheStats struct {
	Size     int     `json:"size"`
	Capacity int     `json:"capacity"`
	HitRate  float64 `json:"hit_rate"`
}

// PingResponse represents the ping response
type PingResponse struct {
	Pong bool `json:"pong"`
}

// VersionResponse represents the version info response
type VersionResponse struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`
}

// ErrorResponse represents an API error following Huma format
type ErrorResponse struct {
	Status int           `json:"status"`
	Title  string        `json:"title"`
	Detail string        `json:"detail,omitempty"`
	Errors []ErrorDetail `json:"errors,omitempty"`
}

// ErrorDetail provides detailed error information
type ErrorDetail struct {
	Message  string      `json:"message"`
	Location string      `json:"location,omitempty"`
	Value    interface{} `json:"value,omitempty"`
}

// NotImplementedResponse for write endpoints
type NotImplementedResponse struct {
	Status  int    `json:"status"`
	Title   string `json:"title"`
	Detail  string `json:"detail"`
	SeeAlso string `json:"see_also"`
}
