// Request/response protocol types for the JSON-lines stdio worker.
library protocol;

class AnalyzeRequest {
  final int id;
  final String method;
  final Map<String, dynamic> params;

  AnalyzeRequest({required this.id, required this.method, required this.params});

  factory AnalyzeRequest.fromJson(Map<String, dynamic> json) {
    return AnalyzeRequest(
      id: json['id'] as int,
      method: json['method'] as String,
      params: (json['params'] as Map<String, dynamic>?) ?? {},
    );
  }
}

class SymbolInfo {
  final String id;
  final String kind;
  final String name;
  final String file;
  final int line;
  final bool isPrivate;
  final bool isOverride;
  final bool isEntryPoint;
  final bool isWidget;
  final List<String> refs;

  SymbolInfo({
    required this.id,
    required this.kind,
    required this.name,
    required this.file,
    required this.line,
    required this.isPrivate,
    required this.isOverride,
    required this.isEntryPoint,
    required this.isWidget,
    required this.refs,
  });

  Map<String, dynamic> toJson() => {
        'id': id,
        'kind': kind,
        'name': name,
        'file': file,
        'line': line,
        'is_private': isPrivate,
        'is_override': isOverride,
        'is_entry_point': isEntryPoint,
        'is_widget': isWidget,
        'refs': refs,
      };
}

Map<String, dynamic> successResponse(int id, List<SymbolInfo> symbols) => {
      'id': id,
      'symbols': symbols.map((s) => s.toJson()).toList(),
    };

Map<String, dynamic> errorResponse(int id, String message) => {
      'id': id,
      'error': message,
    };

Map<String, dynamic> pongResponse(int id) => {
      'id': id,
      'pong': true,
    };
