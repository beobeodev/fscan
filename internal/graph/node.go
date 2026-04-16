package graph

// Kind represents the type of a symbol node.
type Kind string

const (
	KindFile     Kind = "file"
	KindClass    Kind = "class"
	KindFunction Kind = "function"
	KindMethod   Kind = "method"
	KindVariable Kind = "variable"
	KindTypedef  Kind = "typedef"
	KindExtension Kind = "extension"
	KindAsset    Kind = "asset"
)

// Node is a vertex in the symbol dependency graph.
type Node struct {
	// ID is unique across the project, e.g., "file:lib/home.dart" or "class:lib/home.dart::HomePage"
	ID string

	Kind Kind

	// Name is the symbol name (class name, function name, file path, asset path).
	Name string

	// File is the source file path (relative to project root).
	File string

	// Line is the declaration line number (0 for file-level nodes).
	Line int

	// IsPrivate indicates a leading-underscore name.
	IsPrivate bool

	// IsOverride is true for @override annotated methods.
	IsOverride bool

	// IsEntryPoint is true for @pragma('vm:entry-point') symbols.
	IsEntryPoint bool

	// IsWidget is true for StatelessWidget/StatefulWidget subclasses.
	IsWidget bool

	// IsGenerated is true for files matching *.g.dart, *.gen.dart, *.freezed.dart patterns.
	IsGenerated bool

	// IsPartOf is true for files that are part of another file (part of 'parent.dart').
	IsPartOf bool

	// PartOfFile is the parent file path when IsPartOf is true.
	PartOfFile string

	// Refs contains IDs of nodes that THIS node references (outgoing edges).
	Refs []string

	// RefBy contains IDs of nodes that reference THIS node (incoming edges).
	RefBy []string
}

// FileID constructs the canonical ID for a file node.
func FileID(relPath string) string {
	return "file:" + relPath
}

// SymbolID constructs the canonical ID for a symbol node.
func SymbolID(kind Kind, file, name string) string {
	return string(kind) + ":" + file + "::" + name
}

// AssetID constructs the canonical ID for an asset node.
func AssetID(assetPath string) string {
	return "asset:" + assetPath
}
