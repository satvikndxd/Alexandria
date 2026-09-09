import 'package:flutter/widgets.dart';

import 'app_scope.dart';
import 'app_state.dart';
import 'core/api.dart';
import 'screens/auth_screen.dart';
import 'screens/home_screen.dart';
import 'screens/library_screen.dart';
import 'theme/tokens.dart';
import 'theme/typography.dart';
import 'widgets/drop_cap.dart';
import 'widgets/ornament.dart';

/// App shell. Deliberately built on WidgetsApp, not MaterialApp: Alexandria's
/// UI is drawn from its own design system — no Material ink ripples, no
/// Cupertino chrome, no default SaaS look to fight.
void main() {
  const apiBase = String.fromEnvironment('ALEXANDRIA_API', defaultValue: 'http://127.0.0.1:8080/v1');
  runApp(AlexandriaApp(state: AppState(ApiClient(baseUrl: apiBase))));
}

class AlexandriaApp extends StatelessWidget {
  const AlexandriaApp({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    return AppStateScope(
      state: state,
      child: WidgetsApp(
        title: 'Alexandria',
        color: AlexandriaColors.ink,
        onGenerateRoute: (_) => PageRouteBuilder(
          opaque: true,
          pageBuilder: (_, __, ___) => const _Root(),
        ),
      ),
    );
  }
}

class _Root extends StatefulWidget {
  const _Root();

  @override
  State<_Root> createState() => _RootState();
}

class _RootState extends State<_Root> {
  int _tab = 0;

  @override
  void initState() {
    super.initState();
    // Refresh the session once the tree exists; failures are silent because
    // an unreachable API must still show the catalogue.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      AppStateScope.of(context).refreshMe();
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = AppStateScope.of(context);
    return AnimatedBuilder(
      animation: state,
      builder: (context, _) => ColoredBox(
        color: AlexandriaColors.parchment,
        child: SafeArea(
          child: Column(
            children: [
              _masthead(state),
              const InkRule(color: AlexandriaColors.inkFaint),
              Expanded(
                child: switch (_tab) {
                  1 => LibraryScreen(state: state),
                  2 => state.signedIn ? const _SignOutPane() : AuthScreen(state: state),
                  _ => HomeScreen(state: state),
                },
              ),
              _bottomBar(),
            ],
          ),
        ),
      ),
    );
  }

  Widget _masthead(AppState state) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          const DropCap(letter: 'A', size: 44),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Alexandria',
                  style: AlexType.body(fontSize: 26, letterSpacing: 6, color: AlexandriaColors.ink),
                ),
                Text(
                  'BOOKS · PEOPLE · IDEAS · FOREVER',
                  style: AlexType.body(fontSize: 9, letterSpacing: 2.2, color: AlexandriaColors.inkFaint),
                ),
              ],
            ),
          ),
          if (state.signedIn)
            Text(state.username, style: AlexType.body(fontSize: 12, color: AlexandriaColors.botanical)),
        ],
      ),
    );
  }

  Widget _bottomBar() {
    const labels = ['Atrium', 'Library', 'Account'];
    return Container(
      decoration: const BoxDecoration(
        border: Border(top: BorderSide(color: AlexandriaColors.ink, width: 2)),
      ),
      child: Row(
        children: [
          for (var i = 0; i < labels.length; i++)
            Expanded(
              child: GestureDetector(
                onTap: () => setState(() => _tab = i),
                child: Container(
                  color: _tab == i ? AlexandriaColors.ink : null,
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  alignment: Alignment.center,
                  child: Text(
                    labels[i],
                    style: AlexType.body(
                      fontSize: 12,
                      letterSpacing: 1.8,
                      color: _tab == i ? AlexandriaColors.parchmentLight : AlexandriaColors.inkSoft,
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class _SignOutPane extends StatelessWidget {
  const _SignOutPane();

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Text('Signed in. Your shelves and margin live in the Library tab.'),
    );
  }
}
