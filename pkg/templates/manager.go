package templates

import (
	"bytes"
	"fmt"
	"path/filepath"
	"text/template"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// Renderer interface for template rendering (for dependency injection)
type Renderer interface {
	GetTemplate(name string) *template.Template
	ExecuteTemplate(name string, data any) (string, error)
	TemplateExists(name string) bool
}

// Manager manages templates from a directory
type Manager struct {
	templates *template.Template
	directory string
}

// GetDefaultFuncMap returns common template helper functions
func GetDefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"float": func(val interface{}) float64 {
			switch v := val.(type) {
			case float64:
				return v
			case float32:
				return float64(v)
			case int:
				return float64(v)
			default:
				if dec, ok := val.(interface{ InexactFloat64() float64 }); ok {
					return dec.InexactFloat64()
				}
				return 0
			}
		},
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"add": func(a, b int) int {
			return a + b
		},
		"printf": fmt.Sprintf,
		"lt":     func(a, b float64) bool { return a < b },
		"gt":     func(a, b float64) bool { return a > b },
		"le":     func(a, b float64) bool { return a <= b },
		"ge":     func(a, b float64) bool { return a >= b },
		"eq":     func(a, b interface{}) bool { return a == b },
		"ne":     func(a, b interface{}) bool { return a != b },
		"and":    func(a, b bool) bool { return a && b },
	}
}

// NewManager creates and loads all templates from directory (including all subdirectories)
func NewManager(templatesDir string) (*Manager, error) {
	tmpl := template.New("root").Funcs(GetDefaultFuncMap())

	// Load templates from root directory (if any exist)
	pattern := filepath.Join(templatesDir, "*.tmpl")
	if result, err := tmpl.ParseGlob(pattern); err == nil && result != nil {
		tmpl = result
	}

	// Load from all subdirectories (one level deep: templates/*/*.tmpl)
	subPattern := filepath.Join(templatesDir, "*", "*.tmpl")
	if result, err := tmpl.ParseGlob(subPattern); err == nil && result != nil {
		tmpl = result
	}

	// Load even deeper nesting (templates/*/*/*.tmpl)
	deepPattern := filepath.Join(templatesDir, "*", "*", "*.tmpl")
	if result, err := tmpl.ParseGlob(deepPattern); err == nil && result != nil {
		tmpl = result
	}

	if tmpl == nil {
		return nil, fmt.Errorf("failed to initialize templates")
	}

	templateCount := len(tmpl.Templates())
	if templateCount <= 1 { // "root" template doesn't count
		return nil, fmt.Errorf("no templates found in %s or subdirectories", templatesDir)
	}

	logger.Info("templates loaded recursively",
		zap.Int("count", templateCount),
		zap.String("directory", templatesDir),
	)

	return &Manager{
		templates: tmpl,
		directory: templatesDir,
	}, nil
}

// NewManagerWithValidation creates manager and validates required templates exist
func NewManagerWithValidation(templatesDir string, requiredTemplates []string) (*Manager, error) {
	manager, err := NewManager(templatesDir)
	if err != nil {
		return nil, err
	}

	// Verify all required templates exist
	for _, name := range requiredTemplates {
		if manager.templates.Lookup(name) == nil {
			return nil, fmt.Errorf("required template not found: %s", name)
		}
	}

	return manager, nil
}

// GetTemplate returns template by name
func (m *Manager) GetTemplate(name string) *template.Template {
	return m.templates.Lookup(name)
}

// ExecuteTemplate renders template with data
func (m *Manager) ExecuteTemplate(name string, data interface{}) (string, error) {
	tmpl := m.templates.Lookup(name)
	if tmpl == nil {
		return "", fmt.Errorf("template %s not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", name, err)
	}

	return buf.String(), nil
}

// TemplateExists checks if template exists
func (m *Manager) TemplateExists(name string) bool {
	return m.templates.Lookup(name) != nil
}

// GetDirectory returns templates directory path
func (m *Manager) GetDirectory() string {
	return m.directory
}
