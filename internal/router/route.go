package router

// RouteDef describes a single route defined in router configuration code.
// It is produced by the parser and consumed by the unused-route rule.
type RouteDef struct {
	// File is the relative path (slash-separated) where the route is declared.
	File string
	// Line is the 1-based line number of the declaration.
	Line int
	// Kind is the detected router flavour: "auto_route" | "go_router" | "navigator".
	Kind string
	// Label is a human-readable route identifier used in issue messages.
	Label string
	// Identifiers is the set of tokens that, if any appears in a navigation
	// call site anywhere in the project, makes this route "live". Examples:
	//   auto_route: [ "FooRoute" ]
	//   go_router:  [ "/foo/:id", "fooName" ]
	//   navigator:  [ "/foo" ]
	Identifiers []string
	// Initial marks routes that are reachable as the app entry point
	// (initialLocation, initial:true, or path "/"). Never reported as unused.
	Initial bool
}
