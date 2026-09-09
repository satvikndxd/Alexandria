import 'package:flutter/widgets.dart';

import 'app_state.dart';

/// Provides [AppState] to the tree without prop-drilling through every
/// FutureBuilder; the mobile analogue of the web's server context.
class AppStateScope extends InheritedWidget {
  const AppStateScope({super.key, required this.state, required super.child});

  final AppState state;

  static AppState of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AppStateScope>()!.state;

  @override
  bool updateShouldNotify(covariant AppStateScope old) => old.state != state;
}
