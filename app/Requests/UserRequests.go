// Package requests holds the input payloads controllers bind to, along with
// their validation rules. Keeping them separate from models means the wire
// format can differ from the table shape.
package requests

import (
	"strings"

	"github.com/gochin/framework/pkg/router"
)

// CreateUser is the payload for creating a user.
type CreateUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Validate reports field-level problems as a single 422.
func (r CreateUser) Validate() error {
	fields := map[string]string{}

	if strings.TrimSpace(r.Name) == "" {
		fields["name"] = "name is required"
	}
	if err := validateEmail(r.Email); err != "" {
		fields["email"] = err
	}

	if len(fields) > 0 {
		return router.Unprocessablef("validation failed").WithFields(fields)
	}
	return nil
}

// UpdateUser is the payload for a partial user update. Pointer fields
// distinguish "absent" from "set to empty".
type UpdateUser struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

func (r UpdateUser) Validate() error {
	fields := map[string]string{}

	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		fields["name"] = "name cannot be empty"
	}
	if r.Email != nil {
		if err := validateEmail(*r.Email); err != "" {
			fields["email"] = err
		}
	}
	if r.Name == nil && r.Email == nil {
		return router.Unprocessablef("no fields to update")
	}

	if len(fields) > 0 {
		return router.Unprocessablef("validation failed").WithFields(fields)
	}
	return nil
}

func validateEmail(email string) string {
	email = strings.TrimSpace(email)
	switch {
	case email == "":
		return "email is required"
	case !strings.Contains(email, "@"), strings.HasPrefix(email, "@"), strings.HasSuffix(email, "@"):
		return "email must be a valid address"
	default:
		return ""
	}
}

// CreatePost is the payload for creating a post.
type CreatePost struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	Published bool   `json:"published"`
	UserID    int64  `json:"user_id"`
}

func (r CreatePost) Validate() error {
	fields := map[string]string{}

	if strings.TrimSpace(r.Title) == "" {
		fields["title"] = "title is required"
	}
	if r.UserID <= 0 {
		fields["user_id"] = "user_id is required"
	}

	if len(fields) > 0 {
		return router.Unprocessablef("validation failed").WithFields(fields)
	}
	return nil
}
