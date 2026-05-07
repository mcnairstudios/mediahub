package source

import "encoding/json"

// FieldType identifies the UI input type for a config field.
type FieldType string

const (
	FieldText     FieldType = "text"
	FieldPassword FieldType = "password"
	FieldURL      FieldType = "url"
	FieldNumber   FieldType = "number"
	FieldBool     FieldType = "bool"
	FieldSelect   FieldType = "select"
	FieldHidden   FieldType = "hidden"
	FieldCustom   FieldType = "custom"
)

// ConfigField describes a single configuration field for a source plugin.
type ConfigField struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	Type        FieldType       `json:"type"`
	Required    bool            `json:"required,omitempty"`
	Default     json.RawMessage `json:"default,omitempty"`
	Placeholder string          `json:"placeholder,omitempty"`
	HelpText    string          `json:"help_text,omitempty"`
	Options     []Option        `json:"options,omitempty"`
	Component   string          `json:"component,omitempty"`
}

// Option is a value/label pair for select fields.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ViewHint provides UI hints for how a plugin's data should be displayed.
type ViewHint struct {
	Layout     string       `json:"layout,omitempty"`
	GroupBy    string       `json:"group_by,omitempty"`
	Searchable bool         `json:"searchable,omitempty"`
	Sortable   bool         `json:"sortable,omitempty"`
	Columns    []ViewColumn `json:"columns,omitempty"`
}

// ViewColumn describes a single column in a tabular plugin view.
type ViewColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Interaction describes a user-triggerable action that a plugin supports.
type Interaction struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	TargetField string `json:"target_field,omitempty"`
}

// PluginDescriptor declares metadata about a source plugin.
type PluginDescriptor struct {
	Type         SourceType    `json:"type"`
	Label        string        `json:"label"`
	ShortLabel   string        `json:"short_label"`
	Color        string        `json:"color"`
	Icon         string        `json:"icon,omitempty"`
	Version      string        `json:"version"`
	Description  string        `json:"description,omitempty"`
	ConfigFields []ConfigField `json:"config_fields"`
	View         *ViewHint     `json:"view,omitempty"`
	Interactions []Interaction `json:"interactions,omitempty"`
}

// PluginRegistration bundles everything needed to register a source plugin.
type PluginRegistration struct {
	Descriptor   PluginDescriptor
	Factory      Factory
	CustomRoutes []CustomRoute
	FrontendJS   []byte
}

// CustomRoute defines an additional API endpoint provided by a plugin.
// Handler must be an http.HandlerFunc — use any to avoid importing net/http
// in the source package. The API layer type-asserts it.
type CustomRoute struct {
	Method  string
	Pattern string // relative path, e.g. "places" -> /api/sources/{type}/places
	Handler any
}
