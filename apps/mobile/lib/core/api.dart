import 'dart:convert';
import 'dart:io';

/// Alexandria API client for mobile/desktop.
///
/// Sessions ride on the [HttpClient]'s cookie jar (first-party to the API
/// host), and mutating calls attach the CSRF double-submit token read back
/// out of the jar — the same contract the web client honours, with no
/// platform channel in between.
class ApiException implements Exception {
  ApiException(this.status, this.code, this.message);
  final int status;
  final String code;
  final String message;
  @override
  String toString() => 'ApiException($status $code): $message';
}

class ApiClient {
  ApiClient({required this.baseUrl}) : _base = Uri.parse(baseUrl);

  /// e.g. http://127.0.0.1:8080/v1 — paths below are relative to /v1.
  final String baseUrl;
  final Uri _base;
  final HttpClient _http = HttpClient();

  /// A minimal cookie jar: the session and CSRF cookies are the only state
  /// the client keeps, and hand-rolling the jar keeps it inspectable (and
  /// testable) rather than buried in dart:io internals.
  final Map<String, String> _cookies = {} ;

  void _storeCookies(HttpClientResponse resp) {
    for (final raw in resp.headers[HttpHeaders.setCookieHeader] ?? const <String>[]) {
      final kv = raw.split(';').first.split('=');
      if (kv.length == 2) _cookies[kv[0].trim()] = kv[1].trim();
    }
  }

  String? _cookieHeader() {
    if (_cookies.isEmpty) return null;
    return _cookies.entries.map((e) => '${e.key}=${e.value}').join('; ');
  }

  Uri _uri(String path, [Map<String, String>? query]) => _base.replace(
        path: '${_base.path}$path',
        queryParameters: query?.isEmpty ?? true ? null : query,
      );

  String? _csrf() => _cookies['alexandria_csrf'];

  Future<dynamic> _send(String method, String path, [Object? body, Map<String, String>? query]) async {
    final req = await _http.openUrl(method, _uri(path, query));
    req.headers.set('accept', 'application/json');
    final jar = _cookieHeader();
    if (jar != null) req.headers.set('cookie', jar);
    if (body != null) {
      req.headers.set('content-type', 'application/json');
      final token = await _csrf();
      if (token != null) req.headers.set('x-csrf-token', token);
      req.write(jsonEncode(body));
    }
    final resp = await req.close();
    _storeCookies(resp);
    final text = await resp.transform(utf8.decoder).join();
    final decoded = text.isEmpty ? <String, dynamic>{} : jsonDecode(text);
    if (resp.statusCode >= 300) {
      final err = (decoded is Map && decoded['error'] is Map)
          ? decoded['error'] as Map
          : <String, dynamic>{};
      throw ApiException(
        resp.statusCode,
        (err['code'] as String?) ?? 'http',
        (err['message'] as String?) ?? resp.reasonPhrase,
      );
    }
    return decoded;
  }

  Future<Map<String, dynamic>> get(String path, [Map<String, String>? query]) async =>
      (await _send('GET', path, null, query)) as Map<String, dynamic>;

  Future<Map<String, dynamic>> post(String path, [Object? body]) async =>
      (await _send('POST', path, body ?? const <String, dynamic>{})) as Map<String, dynamic>;

  Future<Map<String, dynamic>> patch(String path, [Object? body]) async =>
      (await _send('PATCH', path, body ?? const <String, dynamic>{})) as Map<String, dynamic>;

  Future<Map<String, dynamic>> delete(String path, [Object? body]) async =>
      (await _send('DELETE', path, body ?? const <String, dynamic>{})) as Map<String, dynamic>;

  void close() => _http.close(force: true);
}
