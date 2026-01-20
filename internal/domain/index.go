package domain

// Index represents the index.yaml file structure
type Index struct {
	Version   string        `json:"version" yaml:"version"`
	Commit    string        `json:"commit" yaml:"commit"`
	UpdatedAt string        `json:"updated_at" yaml:"updated_at"`
	Servers   []IndexEntry  `json:"servers" yaml:"servers"`
}

// IndexEntry represents a single server entry in the index
type IndexEntry struct {
	Name        string            `json:"name" yaml:"name"`
	Path        string            `json:"path" yaml:"path"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string            `json:"version,omitempty" yaml:"version,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}
