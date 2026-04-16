package dart

// Symbol represents a Dart symbol extracted by the semantic worker.
type Symbol struct {
	// ID is the canonical symbol identifier: "class:lib/foo.dart::MyClass"
	ID string `json:"id"`

	// Kind: class, function, method, variable, typedef, extension
	Kind string `json:"kind"`

	// Name is the symbol name.
	Name string `json:"name"`

	// File is the file path relative to project root.
	File string `json:"file"`

	// Line is the declaration line number.
	Line int `json:"line"`

	// IsPrivate is true for names starting with _.
	IsPrivate bool `json:"is_private"`

	// IsOverride is true for @override annotated methods.
	IsOverride bool `json:"is_override"`

	// IsEntryPoint is true for @pragma('vm:entry-point') symbols.
	IsEntryPoint bool `json:"is_entry_point"`

	// IsWidget is true for StatelessWidget/StatefulWidget subclasses.
	IsWidget bool `json:"is_widget"`

	// IsFrameworkState is true for _FooState classes paired with StatefulWidget Foo.
	IsFrameworkState bool `json:"is_framework_state"`

	// OwnerClass is the enclosing class name for methods (empty for top-level symbols).
	OwnerClass string `json:"owner_class,omitempty"`

	// Refs are file paths where this symbol is referenced.
	Refs []string `json:"refs"`
}

// AnalyzeRequest is the Go side of a JSON-lines request to the Dart worker.
type AnalyzeRequest struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

// AnalyzeParams holds parameters for the "analyze_project" method.
type AnalyzeParams struct {
	Root         string   `json:"root"`
	EntryPoints  []string `json:"entry_points"`
}

// PingParams holds parameters for the "ping" method.
type PingParams struct{}

// AnalyzeResponse is the Go side of a JSON-lines response from the Dart worker.
type AnalyzeResponse struct {
	ID      int       `json:"id"`
	Symbols []*Symbol `json:"symbols,omitempty"`
	Pong    bool      `json:"pong,omitempty"`
	Error   string    `json:"error,omitempty"`
}
