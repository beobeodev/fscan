import 'dart:io';
import 'package:analyzer/dart/element/element.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';
import 'package:analyzer/file_system/physical_file_system.dart';
import 'package:analyzer/src/dart/analysis/analysis_context_collection.dart';
import 'package:analyzer/dart/analysis/results.dart';

import '../protocol/protocol.dart';

/// Analyzes a Flutter/Dart project using package:analyzer.
/// Returns a flat list of SymbolInfo for all classes, functions, and methods.
class ProjectAnalyzer {
  final String projectRoot;

  ProjectAnalyzer(this.projectRoot);

  /// Compute relative path from an absolute path, matching declaration IDs.
  String _relPath(String absPath) {
    if (absPath.startsWith(projectRoot)) {
      var rel = absPath.substring(projectRoot.length);
      if (rel.startsWith('/') || rel.startsWith(Platform.pathSeparator)) {
        rel = rel.substring(1);
      }
      return rel.replaceAll(r'\', '/');
    }
    return absPath;
  }

  Future<List<SymbolInfo>> analyze() async {
    final collection = AnalysisContextCollectionImpl(
      includedPaths: [projectRoot],
      resourceProvider: PhysicalResourceProvider.INSTANCE,
    );

    final allSymbols = <SymbolInfo>[];
    final refTracker = ReferenceTracker();

    // First pass: collect all symbol declarations + references
    for (final context in collection.contexts) {
      for (final filePath in context.contextRoot.analyzedFiles()) {
        if (!filePath.endsWith('.dart')) continue;
        if (_shouldSkipFile(filePath)) continue;

        final result = await context.currentSession.getResolvedUnit(filePath);
        if (result is! ResolvedUnitResult) continue;

        final relPath = _relPath(filePath);
        final visitor = SymbolDeclarationVisitor(relPath, result.unit);
        result.unit.accept(visitor);
        allSymbols.addAll(visitor.symbols);

        // Collect references — pass projectRoot so it can compute relative paths
        final refVisitor = ReferenceVisitor(relPath, refTracker, projectRoot);
        result.unit.accept(refVisitor);
      }
    }

    // Second pass: attach reference data to symbols
    for (final sym in allSymbols) {
      sym.refs.addAll(refTracker.refsFor(sym.id));
    }

    return allSymbols;
  }

  bool _shouldSkipFile(String path) {
    final base = path.split('/').last;
    return base.endsWith('.g.dart') ||
        base.endsWith('.gen.dart') ||
        base.endsWith('.freezed.dart') ||
        base.endsWith('.gr.dart') ||
        base.endsWith('.mocks.dart') ||
        path.contains('/test/') ||
        path.contains('/build/') ||
        path.contains('/.dart_tool/');
  }
}

/// Tracks which symbol IDs are referenced from which files.
class ReferenceTracker {
  // symbolID → set of file paths that reference it
  final Map<String, Set<String>> _refs = {};

  void addRef(String symbolID, String fromFile) {
    _refs.putIfAbsent(symbolID, () => {}).add(fromFile);
  }

  List<String> refsFor(String symbolID) {
    return _refs[symbolID]?.toList() ?? [];
  }
}

/// Visits top-level and class-level declarations and collects SymbolInfo.
class SymbolDeclarationVisitor extends RecursiveAstVisitor<void> {
  final String file;
  final CompilationUnit unit;
  final List<SymbolInfo> symbols = [];

  SymbolDeclarationVisitor(this.file, this.unit);

  @override
  void visitClassDeclaration(ClassDeclaration node) {
    final name = node.name.lexeme;
    final elem = node.declaredElement;
    if (elem == null) return;

    final isPrivate = name.startsWith('_');
    final isWidget = _isWidgetSubclass(elem);
    final isEntryPoint = _hasPragmaEntryPoint(elem);

    symbols.add(SymbolInfo(
      id: 'class:$file::$name',
      kind: 'class',
      name: name,
      file: file,
      line: node.name.offset > 0
          ? unit.lineInfo.getLocation(node.name.offset).lineNumber
          : 0,
      isPrivate: isPrivate,
      isOverride: false,
      isEntryPoint: isEntryPoint,
      isWidget: isWidget,
      refs: [],
    ));

    super.visitClassDeclaration(node);
  }

  @override
  void visitEnumDeclaration(EnumDeclaration node) {
    final name = node.name.lexeme;
    symbols.add(SymbolInfo(
      id: 'class:$file::$name',
      kind: 'class',
      name: name,
      file: file,
      line: unit.lineInfo.getLocation(node.name.offset).lineNumber,
      isPrivate: name.startsWith('_'),
      isOverride: false,
      isEntryPoint: false,
      isWidget: false,
      refs: [],
    ));
    super.visitEnumDeclaration(node);
  }

  @override
  void visitMixinDeclaration(MixinDeclaration node) {
    final name = node.name.lexeme;
    symbols.add(SymbolInfo(
      id: 'class:$file::$name',
      kind: 'class',
      name: name,
      file: file,
      line: unit.lineInfo.getLocation(node.name.offset).lineNumber,
      isPrivate: name.startsWith('_'),
      isOverride: false,
      isEntryPoint: false,
      isWidget: false,
      refs: [],
    ));
    super.visitMixinDeclaration(node);
  }

  @override
  void visitFunctionDeclaration(FunctionDeclaration node) {
    final name = node.name.lexeme;
    // Skip main() — it's always an entry point
    if (name == 'main') return;

    symbols.add(SymbolInfo(
      id: 'function:$file::$name',
      kind: 'function',
      name: name,
      file: file,
      line: unit.lineInfo.getLocation(node.name.offset).lineNumber,
      isPrivate: name.startsWith('_'),
      isOverride: false,
      isEntryPoint: _hasPragmaEntryPoint(node.declaredElement),
      isWidget: false,
      refs: [],
    ));
  }

  @override
  void visitMethodDeclaration(MethodDeclaration node) {
    final name = node.name.lexeme;
    final isOverride = node.metadata.any((a) => a.name.name == 'override');

    symbols.add(SymbolInfo(
      id: 'method:$file::$name',
      kind: 'method',
      name: name,
      file: file,
      line: unit.lineInfo.getLocation(node.name.offset).lineNumber,
      isPrivate: name.startsWith('_'),
      isOverride: isOverride,
      isEntryPoint: false,
      isWidget: false,
      refs: [],
    ));
  }

  bool _isWidgetSubclass(ClassElement elem) {
    ClassElement? current = elem.supertype?.element as ClassElement?;
    int depth = 0;
    while (current != null && depth < 10) {
      final name = current.name;
      if (name == 'StatelessWidget' ||
          name == 'StatefulWidget' ||
          name == 'Widget') {
        return true;
      }
      current = current.supertype?.element as ClassElement?;
      depth++;
    }
    return false;
  }

  bool _hasPragmaEntryPoint(Element? elem) {
    if (elem == null) return false;
    return elem.metadata.any((m) {
      final source = m.toSource();
      return source.contains("vm:entry-point");
    });
  }
}

/// Visits all identifier references and records them in ReferenceTracker.
class ReferenceVisitor extends RecursiveAstVisitor<void> {
  final String file;
  final ReferenceTracker tracker;
  final String projectRoot;

  ReferenceVisitor(this.file, this.tracker, this.projectRoot);

  @override
  void visitSimpleIdentifier(SimpleIdentifier node) {
    final elem = node.staticElement;
    if (elem == null) return;

    _trackElement(elem);
    super.visitSimpleIdentifier(node);
  }

  void _trackElement(Element elem) {
    // Resolve ConstructorElement → enclosing ClassElement
    // so that `MyWidget()` registers as a reference to class MyWidget
    Element target = elem;
    if (elem is ConstructorElement) {
      target = elem.enclosingElement;
    }

    // Only track user-code elements
    final source = target.library?.source;
    if (source == null) return;

    String? kind;
    if (target is ClassElement) {
      kind = 'class';
    } else if (target is FunctionElement) {
      kind = 'function';
    } else if (target is MethodElement) {
      kind = 'method';
    } else if (target is EnumElement) {
      kind = 'class';
    } else if (target is MixinElement) {
      kind = 'class';
    }

    if (kind == null) return;

    // Compute relative path from the source file of the referenced element
    final symbolFile = _resolveRelPath(source.fullName);
    if (symbolFile == null) return;

    // Don't track self-references within the same file
    if (symbolFile == file) return;

    final symbolID = '$kind:$symbolFile::${target.name}';
    tracker.addRef(symbolID, file);
  }

  /// Resolves an absolute path to a relative path from projectRoot.
  String? _resolveRelPath(String absPath) {
    // Normalize separators
    final normalized = absPath.replaceAll(r'\', '/');

    // Find the projectRoot in the path (handle both URI and filesystem paths)
    final normalizedRoot = projectRoot.replaceAll(r'\', '/');

    if (normalized.startsWith(normalizedRoot)) {
      var rel = normalized.substring(normalizedRoot.length);
      if (rel.startsWith('/')) {
        rel = rel.substring(1);
      }
      return rel;
    }

    // Handle file:// URI format
    final uriPrefix = 'file://';
    if (normalized.startsWith(uriPrefix)) {
      final fsPath = normalized.substring(uriPrefix.length);
      if (fsPath.startsWith(normalizedRoot)) {
        var rel = fsPath.substring(normalizedRoot.length);
        if (rel.startsWith('/')) {
          rel = rel.substring(1);
        }
        return rel;
      }
    }

    // External package — not part of our project
    return null;
  }
}
