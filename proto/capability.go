package proto

import (
	"errors"
	"fmt"
	"strings"
)

type Feature struct {
	Name        string            `json:"name"`                  // Unique key: "temperature", "led", etc.
	Description string            `json:"description,omitempty"` // Human-readable purpose
	Methods     FeatureMethods `json:"methods"`               // Routing topics (Generated topics will be name/method)
}

type DataType struct {
	Type        string    `json:"type"`
	Unit        string    `json:"unit,omitempty"`
	Description string    `json:"description,omitempty"`
	Optional    bool      `json:"optional,omitempty"`
	Range       []float64 `json:"range,omitempty"`
	Enum        []string  `json:"enum,omitemtpy"`
}

type Method struct {
	InputSchema  map[string]DataType `json:"input_schema,omitempty"`
	OutputSchema map[string]DataType `json:"output_schema,omitempty"`
	Description  string              `json:"description,omitempty"`
}

type FeatureMethods struct {
	Data    Method `json:"data,omitempty"`
	Status  Method `json:"status,omitempty"`
	Command Method `json:"command,omitempty"`
	Query   Method `json:"query,omitempty"`
}

func (c *Feature) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("feature name is required")
	}

	// Ensure at least one method is defined
	if !c.Methods.Data.IsDefined() &&
		!c.Methods.Status.IsDefined() &&
		!c.Methods.Command.IsDefined() &&
		!c.Methods.Query.IsDefined() {
		return fmt.Errorf("feature %q must define at least one method", c.Name)
	}

	// Validate each method individually
	if err := c.Methods.Data.Validate("data"); err != nil {
		return err
	}
	if err := c.Methods.Status.Validate("status"); err != nil {
		return err
	}
	if err := c.Methods.Command.Validate("command"); err != nil {
		return err
	}
	if err := c.Methods.Query.Validate("query"); err != nil {
		return err
	}

	return nil
}
func (m Method) IsDefined() bool {
	return len(m.InputSchema) > 0 || len(m.OutputSchema) > 0
}

func (m Method) Validate(name string) error {
	for field, schema := range m.InputSchema {
		if err := validateDataType(fmt.Sprintf("%s.input.%s", name, field), schema); err != nil {
			return err
		}
	}
	for field, schema := range m.OutputSchema {
		if err := validateDataType(fmt.Sprintf("%s.output.%s", name, field), schema); err != nil {
			return err
		}
	}
	return nil
}

var validDataTypes = map[string]bool{
	"number": true,
	"string": true,
	"bool":   true,
	"enum":   true,
	"object": true,
}

func validateDataType(path string, dt DataType) error {
	if _, ok := validDataTypes[dt.Type]; !ok {
		return fmt.Errorf("invalid data type %q at %s", dt.Type, path)
	}
	if dt.Type == "enum" && len(dt.Enum) == 0 {
		return fmt.Errorf("enum type at %s must define non-empty Enum", path)
	}
	if dt.Type == "number" && len(dt.Range) > 0 && len(dt.Range) != 2 {
		return fmt.Errorf("range at %s must have exactly two values (min, max)", path)
	}
	return nil
}
