package domain

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

// ServerNameRegex validates server names in reverse-DNS format
var ServerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$`)

// SemVerRegex validates semantic version strings
var SemVerRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// NewValidator creates a configured validator instance
func NewValidator() *validator.Validate {
	v := validator.New()

	// Register custom server name validation
	_ = v.RegisterValidation("server_name", func(fl validator.FieldLevel) bool {
		return ServerNameRegex.MatchString(fl.Field().String())
	})

	// Register custom semver validation
	_ = v.RegisterValidation("semver", func(fl validator.FieldLevel) bool {
		return SemVerRegex.MatchString(fl.Field().String())
	})

	return v
}

// ValidateServer validates a ServerJSON struct
func ValidateServer(server *ServerJSON) error {
	v := NewValidator()
	return v.Struct(server)
}
