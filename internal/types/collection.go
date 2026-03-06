package types

import (
	"time"

	"github.com/glitchedgitz/pocketbase/models/schema"
)

// Create types.Collection based on below information
//
// Optional
// id
// String	15 characters string to store as collection ID.
// If not set, it will be auto generated.
// Required
// name
// String	Unique collection name (used as a table name for the records table).
// Required
// type
// String	The type of the collection - base (default), auth or view.
// Req|Opt
// schema
// Array
// List with the collection fields.
// This field is required for base collections.
// This field is optional for auth collections.
// This field is optional and autopopulated for view collections based on the options.query.
// For more info about the supported fields and their options, you could check the pocketbase/models/schema Go sub-package definitions.
// Optional
// system
// Boolean	Marks the collection as "system", aka. cannot be renamed or deleted.
// Optional
// listRule
// null|String	API List action rule.
// Check Rules/Filters syntax guide for more details.
// Optional
// viewRule
// null|String	API View action rule.
// Check Rules/Filters syntax guide for more details.
// Optional
// createRule
// null|String
// API Create action rule.
// Check Rules/Filters syntax guide for more details.
// This rule must be null for view collections.
// Optional
// updateRule
// null|String
// API Update action rule.
// Check Rules/Filters syntax guide for more details.
// This rule must be null for view collections.
// Optional
// deleteRule
// null|String
// API Delete action rule.
// Check Rules/Filters syntax guide for more details.
// This rule must be null for view collections.
// options (view)
// ├─
// Required
// query
// null|String	The SQL SELECT statement that will be used to create the underlying view of the collection.
// options (auth)
// ├─
// Optional
// manageRule
// null|String	API rule that gives admin-like permissions to allow fully managing the auth record(s), eg. changing the password without requiring to enter the old one, directly updating the verified state or email, etc. This rule is executed in addition to the createRule and updateRule.
// ├─
// Optional
// allowOAuth2Auth
// Boolean	Whether to allow OAuth2 sign-in/sign-up for the auth collection.
// ├─
// Optional
// allowUsernameAuth
// Boolean	Whether to allow username + password authentication for the auth collection.
// ├─
// Optional
// allowEmailAuth
// Boolean	Whether to allow email + password authentication for the auth collection.
// ├─
// Optional
// requireEmail
// Boolean	Whether to always require email address when creating or updating auth records.
// ├─
// Optional
// exceptEmailDomains
// Array<String>	Whether to allow sign-ups only with the email domains not listed in the specified list.
// ├─
// Optional
// onlyEmailDomains
// Array<String>	Whether to allow sign-ups only with the email domains listed in the specified list.
// └─
// Optional
// minPasswordLength
// Boolean	The minimum required password length for new auth records.

// Collection is a struct that holds information about a collection
type Collection struct {
	ID             string               `json:"id,omitempty"`
	Name           string               `json:"name"`
	Type           string               `json:"type"` // Can be base/auth
	Schema         []schema.SchemaField `json:"schema,omitempty"`
	System         bool                 `json:"system,omitempty"`
	ListRule       string               `json:"listRule,omitempty"`
	ViewRule       string               `json:"viewRule,omitempty"`
	CreateRule     string               `json:"createRule,omitempty"`
	UpdateRule     string               `json:"updateRule,omitempty"`
	DeleteRule     string               `json:"deleteRule,omitempty"`
	ManageRule     string               `json:"manageRule,omitempty"`
	AllowOAuth2    bool                 `json:"allowOAuth2Auth,omitempty"`
	AllowUsername  bool                 `json:"allowUsernameAuth,omitempty"`
	AllowEmail     bool                 `json:"allowEmailAuth,omitempty"`
	RequireEmail   bool                 `json:"requireEmail,omitempty"`
	ExceptEmail    []string             `json:"exceptEmailDomains,omitempty"`
	OnlyEmail      []string             `json:"onlyEmailDomains,omitempty"`
	MinPasswordLen int                  `json:"minPasswordLength,omitempty"`
	CreatedAt      time.Time            `json:"createdAt,omitempty"`
	UpdatedAt      time.Time            `json:"updatedAt,omitempty"`
}
