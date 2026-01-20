package domain

import "time"

// ServerJSON represents an MCP server definition following the MCP schema
type ServerJSON struct {
	Schema      string      `json:"$schema" yaml:"$schema" validate:"required"`
	Name        string      `json:"name" yaml:"name" validate:"required,server_name"`
	Description string      `json:"description" yaml:"description" validate:"required,min=1,max=100"`
	Version     string      `json:"version" yaml:"version" validate:"required,semver"`
	Title       string      `json:"title,omitempty" yaml:"title,omitempty"`
	WebsiteURL  string      `json:"websiteUrl,omitempty" yaml:"websiteUrl,omitempty" validate:"omitempty,url"`
	Repository  *Repository `json:"repository,omitempty" yaml:"repository,omitempty"`
	Packages    []Package   `json:"packages,omitempty" yaml:"packages,omitempty"`
	Remotes     []Remote    `json:"remotes,omitempty" yaml:"remotes,omitempty"`
}

// Repository contains source repository information
type Repository struct {
	URL    string `json:"url" yaml:"url" validate:"required,url"`
	Source string `json:"source" yaml:"source" validate:"required,oneof=github gitlab bitbucket"`
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
}

// Package represents a distributable package of an MCP server
type Package struct {
	RegistryType         string                `json:"registryType" yaml:"registryType" validate:"required,oneof=npm pypi nuget oci mcpb"`
	RegistryBaseURL      string                `json:"registryBaseUrl,omitempty" yaml:"registryBaseUrl,omitempty" validate:"omitempty,url"`
	Identifier           string                `json:"identifier" yaml:"identifier" validate:"required"`
	Version              string                `json:"version,omitempty" yaml:"version,omitempty"`
	FileSHA256           string                `json:"fileSha256,omitempty" yaml:"fileSha256,omitempty"`
	RuntimeHint          string                `json:"runtimeHint,omitempty" yaml:"runtimeHint,omitempty" validate:"omitempty,oneof=npx uvx docker dnx"`
	Transport            Transport             `json:"transport" yaml:"transport" validate:"required"`
	EnvironmentVariables []EnvironmentVariable `json:"environmentVariables,omitempty" yaml:"environmentVariables,omitempty"`
	PackageArguments     []Argument            `json:"packageArguments,omitempty" yaml:"packageArguments,omitempty"`
	RuntimeArguments     []Argument            `json:"runtimeArguments,omitempty" yaml:"runtimeArguments,omitempty"`
}

// Remote represents a cloud-hosted MCP server endpoint
type Remote struct {
	Type    string          `json:"type" yaml:"type" validate:"required,oneof=sse streamable-http"`
	URL     string          `json:"url" yaml:"url" validate:"required,url"`
	Headers []KeyValueInput `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// Transport defines how to communicate with a package
type Transport struct {
	Type    string          `json:"type" yaml:"type" validate:"required,oneof=stdio sse streamable-http"`
	URL     string          `json:"url,omitempty" yaml:"url,omitempty"`
	Headers []KeyValueInput `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// EnvironmentVariable defines an environment variable for configuration
type EnvironmentVariable struct {
	Name        string   `json:"name" yaml:"name" validate:"required"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	IsRequired  bool     `json:"isRequired,omitempty" yaml:"isRequired,omitempty"`
	IsSecret    bool     `json:"isSecret,omitempty" yaml:"isSecret,omitempty"`
	Default     string   `json:"default,omitempty" yaml:"default,omitempty"`
	Choices     []string `json:"choices,omitempty" yaml:"choices,omitempty"`
}

// Argument represents a command-line argument
type Argument struct {
	Type        string   `json:"type" yaml:"type" validate:"required,oneof=positional named"`
	Name        string   `json:"name,omitempty" yaml:"name,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	IsRequired  bool     `json:"isRequired,omitempty" yaml:"isRequired,omitempty"`
	Default     string   `json:"default,omitempty" yaml:"default,omitempty"`
	Choices     []string `json:"choices,omitempty" yaml:"choices,omitempty"`
}

// KeyValueInput represents a configurable key-value pair
type KeyValueInput struct {
	Name        string   `json:"name" yaml:"name" validate:"required"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	IsRequired  bool     `json:"isRequired,omitempty" yaml:"isRequired,omitempty"`
	IsSecret    bool     `json:"isSecret,omitempty" yaml:"isSecret,omitempty"`
	Default     string   `json:"default,omitempty" yaml:"default,omitempty"`
	Choices     []string `json:"choices,omitempty" yaml:"choices,omitempty"`
}

// ServerMeta contains registry metadata about a server
type ServerMeta struct {
	PublisherProvided map[string]interface{} `json:"io.modelcontextprotocol.registry/publisher-provided,omitempty" yaml:"io.modelcontextprotocol.registry/publisher-provided,omitempty"`
	Official          *OfficialMeta          `json:"io.modelcontextprotocol.registry/official,omitempty" yaml:"io.modelcontextprotocol.registry/official,omitempty"`
}

// OfficialMeta contains official registry metadata
type OfficialMeta struct {
	Status      string    `json:"status" yaml:"status"`
	PublishedAt time.Time `json:"publishedAt" yaml:"publishedAt"`
	IsLatest    bool      `json:"isLatest" yaml:"isLatest"`
}
