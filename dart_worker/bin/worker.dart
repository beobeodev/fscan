import 'dart:convert';
import 'dart:io';

import '../analyzer/project_analyzer.dart';
import '../protocol/protocol.dart';

/// Entry point: reads JSON-lines from stdin, dispatches methods, writes responses to stdout.
Future<void> main() async {
  await stdin
      .transform(utf8.decoder)
      .transform(const LineSplitter())
      .forEach((line) async {
    if (line.trim().isEmpty) return;

    Map<String, dynamic>? req;
    try {
      req = jsonDecode(line) as Map<String, dynamic>;
    } catch (e) {
      // Malformed JSON — ignore
      return;
    }

    final request = AnalyzeRequest.fromJson(req);

    try {
      final response = await _dispatch(request);
      stdout.writeln(jsonEncode(response));
    } catch (e, stack) {
      stderr.writeln('[go-scan worker] Error: $e\n$stack');
      stdout.writeln(jsonEncode(errorResponse(request.id, e.toString())));
    }
  });
}

Future<Map<String, dynamic>> _dispatch(AnalyzeRequest req) async {
  switch (req.method) {
    case 'ping':
      return pongResponse(req.id);

    case 'analyze_project':
      final root = req.params['root'] as String?;
      if (root == null || root.isEmpty) {
        return errorResponse(req.id, 'Missing required param: root');
      }
      final analyzer = ProjectAnalyzer(root);
      final symbols = await analyzer.analyze();
      return successResponse(req.id, symbols);

    default:
      return errorResponse(req.id, 'Unknown method: ${req.method}');
  }
}
