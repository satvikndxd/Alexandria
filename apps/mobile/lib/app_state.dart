import 'package:flutter/foundation.dart';

import 'core/api.dart';

/// Session + reader-preference state. Preferences live in memory per session
/// on purpose: they are a comfort, not data, and never leave the device.
class ReaderPrefs {
  double fontSize = 18;
  double lineHeight = 1.7;
  ReaderTheme theme = ReaderTheme.parchment;
}

enum ReaderTheme { parchment, sepia, ink }

class AppState extends ChangeNotifier {
  AppState(this.api);

  final ApiClient api;
  final ReaderPrefs prefs = ReaderPrefs();

  Map<String, dynamic>? me;
  bool loading = false;
  String? lastError;

  bool get signedIn => me?['authenticated'] == true;
  String get username => (me?['user'] as Map?)?['username'] as String? ?? '';

  Future<void> refreshMe() async {
    loading = true;
    notifyListeners();
    try {
      me = await api.get('/me');
      lastError = null;
    } on ApiException catch (e) {
      lastError = e.message;
      me = {'authenticated': false};
    } finally {
      loading = false;
      notifyListeners();
    }
  }

  void tweakPrefs(void Function(ReaderPrefs) mutate) {
    mutate(prefs);
    notifyListeners();
  }
}
