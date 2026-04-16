package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RawSymbol is a declaration extracted from a Dart file via regex.
type RawSymbol struct {
	Name             string
	Kind             string // "class", "function", "method", "extension", "typedef"
	File             string // relative path
	Line             int
	IsPrivate        bool
	IsWidget         bool
	IsOverride       bool
	IsFrameworkState bool   // true for _FooState paired with StatefulWidget
	OwnerClass       string // non-empty for methods inside a class
}

var (
	// Class-like declarations
	reClassDecl = regexp.MustCompile(
		`^\s*(?:abstract\s+|sealed\s+|base\s+|final\s+|interface\s+)*class\s+(\w+)`)
	reEnumDecl  = regexp.MustCompile(`^\s*enum\s+(\w+)`)
	reMixinDecl = regexp.MustCompile(`^\s*mixin\s+(\w+)`)
	reExtDecl   = regexp.MustCompile(`^\s*extension\s+(\w+)\s+on\s+`)
	reTypedef   = regexp.MustCompile(`^\s*typedef\s+(\w+)`)

	// Top-level function — starts at column 0, has a return type + name + open paren.
	// We match: `RetType name(` or `RetType name<T>(` at top-level indent only.
	reTopFunc = regexp.MustCompile(
		`^(\w[\w<>,?\s]*?)\s+(\w+)\s*[<(]`)

	// Widget superclass detection on the same line as a class declaration.
	reExtendsWidget = regexp.MustCompile(
		`class\s+\w+\s+extends\s+(?:Stateless|Stateful)Widget`)

	// @override annotation (may appear on its own line before a method).
	reOverride = regexp.MustCompile(`^\s*@override\b`)

	// Indented method declaration: `  RetType name(` or `  RetType name<T>(`
	reMethodDecl = regexp.MustCompile(
		`^\s+(\w[\w<>,?\s]*?)\s+(\w+)\s*[<(]`)

	// Getter/setter: `get name` or `set name`
	reGetterSetter = regexp.MustCompile(`\b(?:get|set)\s+\w+`)
)

// knownReturnTypes is a set of tokens that can appear as return types for top-level functions.
// We use this to avoid matching lines like `if (foo) {` as function declarations.
var knownReturnTypes = map[string]bool{
	"void": true, "Future": true, "Stream": true, "FutureOr": true,
	"String": true, "int": true, "double": true, "num": true, "bool": true,
	"dynamic": true, "Object": true, "List": true, "Map": true, "Set": true,
	"Iterable": true, "Widget": true, "State": true, "Color": true,
	"TextStyle": true, "EdgeInsets": true, "Offset": true, "Size": true,
	"Duration": true, "DateTime": true, "Uri": true, "Type": true,
	"Uint8List": true, "ByteData": true,
}

// knownDartTypes are names that look like methods but are actually types
// used in callback or field declarations (e.g., `void Function(` or `ValueNotifier<`).
var knownDartTypes = map[string]bool{
	"Function": true, "ValueNotifier": true, "ChangeNotifier": true,
	"Completer": true, "Timer": true, "StreamController": true,
	"StreamSubscription": true, "AnimationController": true,
	"TextEditingController": true, "ScrollController": true,
	"TabController": true, "PageController": true,
}

// lifecycleMethods are Flutter/Dart methods that are always "used" by the framework.
// They should never be flagged as unused.
var lifecycleMethods = map[string]bool{
	"build": true, "createState": true, "initState": true, "dispose": true,
	"didChangeDependencies": true, "didUpdateWidget": true, "deactivate": true,
	"reassemble": true, "setState": true, "createElement": true,
	"toJson": true, "fromJson": true, "toString": true, "hashCode": true,
	"noSuchMethod": true, "runtimeType": true, "main": true,
	"fromMap": true, "toMap": true, "copyWith": true,
}

// ParseSymbols reads a Dart file and extracts top-level and class-level declarations.
func ParseSymbols(absPath, relPath string) ([]RawSymbol, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var symbols []RawSymbol
	sc := bufio.NewScanner(f)
	lineNum := 0
	isIgnoreFile := false
	nextIsOverride := false

	// Class context tracking for method extraction
	currentClass := ""
	braceDepth := 0      // overall brace nesting
	classBraceStart := 0 // braceDepth when class body opened

	for sc.Scan() {
		lineNum++
		line := sc.Text()

		// Check for file-level ignore in first 10 lines
		if lineNum <= 10 && strings.Contains(line, "go-scan:ignore-file") {
			isIgnoreFile = true
			break
		}

		// Track brace depth (skip strings/comments for accuracy)
		braceDepth += countBraces(line)

		// Exit class context when braces close
		if currentClass != "" && braceDepth <= classBraceStart {
			currentClass = ""
		}

		// Track @override on the line before a method
		if reOverride.MatchString(line) {
			nextIsOverride = true
			continue
		}

		// Class declaration
		if m := reClassDecl.FindStringSubmatch(line); m != nil {
			name := m[1]
			isWidget := reExtendsWidget.MatchString(line)
			symbols = append(symbols, RawSymbol{
				Name:      name,
				Kind:      "class",
				File:      relPath,
				Line:      lineNum,
				IsPrivate: strings.HasPrefix(name, "_"),
				IsWidget:  isWidget,
			})
			currentClass = name
			classBraceStart = braceDepth - countBraces(line) // depth before this line's braces
			nextIsOverride = false
			continue
		}

		// Enum declaration
		if m := reEnumDecl.FindStringSubmatch(line); m != nil {
			symbols = append(symbols, RawSymbol{
				Name:      m[1],
				Kind:      "class",
				File:      relPath,
				Line:      lineNum,
				IsPrivate: strings.HasPrefix(m[1], "_"),
			})
			nextIsOverride = false
			continue
		}

		// Mixin declaration
		if m := reMixinDecl.FindStringSubmatch(line); m != nil {
			symbols = append(symbols, RawSymbol{
				Name:      m[1],
				Kind:      "class",
				File:      relPath,
				Line:      lineNum,
				IsPrivate: strings.HasPrefix(m[1], "_"),
			})
			currentClass = m[1]
			classBraceStart = braceDepth - countBraces(line)
			nextIsOverride = false
			continue
		}

		// Extension declaration
		if m := reExtDecl.FindStringSubmatch(line); m != nil {
			symbols = append(symbols, RawSymbol{
				Name:      m[1],
				Kind:      "extension",
				File:      relPath,
				Line:      lineNum,
				IsPrivate: strings.HasPrefix(m[1], "_"),
			})
			currentClass = m[1]
			classBraceStart = braceDepth - countBraces(line)
			nextIsOverride = false
			continue
		}

		// Typedef declaration
		if m := reTypedef.FindStringSubmatch(line); m != nil {
			symbols = append(symbols, RawSymbol{
				Name:      m[1],
				Kind:      "typedef",
				File:      relPath,
				Line:      lineNum,
				IsPrivate: strings.HasPrefix(m[1], "_"),
			})
			nextIsOverride = false
			continue
		}

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			nextIsOverride = false
			continue
		}

		// Method inside a class body (indented)
		if currentClass != "" && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			if m := reMethodDecl.FindStringSubmatch(line); m != nil {
				returnType := strings.TrimSpace(m[1])
				methodName := m[2]

				baseReturn := strings.Split(returnType, "<")[0]
				baseReturn = strings.Split(baseReturn, "?")[0]

				// Skip if not a known return type
				if !knownReturnTypes[baseReturn] {
					nextIsOverride = false
					continue
				}
				// Skip constructors (name matches class)
				if methodName == currentClass || methodName == "_"+currentClass {
					nextIsOverride = false
					continue
				}
				// Skip getters/setters
				if reGetterSetter.MatchString(trimmed) {
					nextIsOverride = false
					continue
				}
				// Skip lifecycle methods
				if lifecycleMethods[methodName] {
					nextIsOverride = false
					continue
				}
				// Skip known Dart types (callback/field declarations, not methods)
				if knownDartTypes[methodName] {
					nextIsOverride = false
					continue
				}

				symbols = append(symbols, RawSymbol{
					Name:       methodName,
					Kind:       "method",
					File:       relPath,
					Line:       lineNum,
					IsPrivate:  strings.HasPrefix(methodName, "_"),
					IsOverride: nextIsOverride,
					OwnerClass: currentClass,
				})
				nextIsOverride = false
				continue
			}
		}

		// Top-level function (only if line starts at column 0, not indented)
		if currentClass == "" && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(trimmed, "//") {
			if m := reTopFunc.FindStringSubmatch(line); m != nil {
				returnType := strings.TrimSpace(m[1])
				funcName := m[2]

				baseReturn := strings.Split(returnType, "<")[0]
				baseReturn = strings.Split(baseReturn, "?")[0]

				if knownReturnTypes[baseReturn] && !lifecycleMethods[funcName] && !nextIsOverride {
					symbols = append(symbols, RawSymbol{
						Name:      funcName,
						Kind:      "function",
						File:      relPath,
						Line:      lineNum,
						IsPrivate: strings.HasPrefix(funcName, "_"),
					})
				}
			}
		}

		nextIsOverride = false
	}

	if isIgnoreFile {
		return nil, nil
	}

	markFrameworkStateClasses(symbols)

	return symbols, sc.Err()
}

// countBraces counts net brace depth change in a line, skipping string literals and comments.
func countBraces(line string) int {
	depth := 0
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		// Skip escape sequences
		if (inSingleQuote || inDoubleQuote) && ch == '\\' {
			i++
			continue
		}

		// Toggle string state
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		// Skip inside strings
		if inSingleQuote || inDoubleQuote {
			continue
		}

		// Line comment — stop processing
		if ch == '/' && i+1 < len(line) && line[i+1] == '/' {
			break
		}

		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
		}
	}
	return depth
}

// markFrameworkStateClasses finds _FooState classes paired with a widget class Foo
// in the same file and marks them as framework-required (should not be flagged).
func markFrameworkStateClasses(symbols []RawSymbol) {
	// Collect all class names in this file (any class, not just widgets)
	classNames := make(map[string]bool)
	for _, s := range symbols {
		if s.Kind == "class" && !s.IsPrivate {
			classNames[s.Name] = true
		}
	}

	// Mark _FooState if Foo exists as a class in same file
	for i := range symbols {
		s := &symbols[i]
		if s.Kind != "class" || !strings.HasSuffix(s.Name, "State") {
			continue
		}

		var widgetName string
		if s.IsPrivate {
			// _FooBarState → FooBar
			widgetName = strings.TrimPrefix(s.Name, "_")
			widgetName = strings.TrimSuffix(widgetName, "State")
		} else {
			// FooBarState → FooBar (public State class)
			widgetName = strings.TrimSuffix(s.Name, "State")
		}

		if widgetName != "" && classNames[widgetName] {
			s.IsFrameworkState = true
		}
	}
}

// IsGeneratedFile returns true for build_runner generated files.
func IsGeneratedFile(relPath string) bool {
	base := filepath.Base(relPath)
	return strings.HasSuffix(base, ".g.dart") ||
		strings.HasSuffix(base, ".gen.dart") ||
		strings.HasSuffix(base, ".freezed.dart") ||
		strings.HasSuffix(base, ".gr.dart") ||
		strings.HasSuffix(base, ".config.dart") ||
		strings.HasSuffix(base, ".chopper.dart") ||
		strings.HasSuffix(base, ".mocks.dart") ||
		strings.HasSuffix(base, ".module.dart")
}

// IsGeneratedFileByContent checks if a file's content indicates it's generated.
func IsGeneratedFileByContent(absPath string) bool {
	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for i := 0; i < 5 && sc.Scan(); i++ {
		if strings.Contains(sc.Text(), "GENERATED CODE") {
			return true
		}
	}
	return false
}
