import 'package:flutter/widgets.dart';

import '../app_state.dart';
import '../core/api.dart';
import '../core/models.dart';
import '../theme/tokens.dart';
import '../theme/typography.dart';
import '../widgets/book_card.dart';
import '../widgets/ornament.dart';
import 'book_screen.dart';
import '../app_scope.dart';

// The Atrium's children need the session/api without prop-drilling through
// every builder; the scope keeps that honest and testable.
AppState _stateOf(BuildContext context) => AppStateScope.of(context);
ApiClient _apiOf(BuildContext context) => _stateOf(context).api;

/// The Atrium: greeting, continue-reading, and a catalogue grid whose order is
/// a Bayesian rating — never an engagement model.
class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Map<String, dynamic>>(
      future: state.api.get('/works', {'limit': '12', 'sort': 'rating'}),
      builder: (context, snap) {
        final works = (snap.data?['works'] as List? ?? const [])
            .map((w) => Work.fromJson(w as Map<String, dynamic>))
            .toList();
        return ListView(
          padding: const EdgeInsets.all(20),
          children: [
            Text(
              state.signedIn ? 'Good reading, ${state.username}.' : 'A human library.',
              style: AlexType.body(fontSize: 26, color: AlexandriaColors.ink),
            ),
            const SizedBox(height: 10),
            const FleuronRule(),
            const SizedBox(height: 18),
            if (state.signedIn) const _ContinueReading(),
            Text('The catalogue', style: AlexType.body(fontSize: 20, color: AlexandriaColors.ink)),
            const SizedBox(height: 12),
            if (snap.hasError)
              Text('The archive is momentarily unreachable.',
                  style: AlexType.body(fontSize: 14, fontStyle: FontStyle.italic, color: AlexandriaColors.inkFaint))
            else
              GridView.builder(
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: 2,
                  childAspectRatio: 0.62,
                  crossAxisSpacing: 16,
                  mainAxisSpacing: 20,
                ),
                itemCount: works.length,
                itemBuilder: (context, i) => BookCard(
                  work: works[i],
                  onTap: () => Navigator.of(context).push(
                    PageRouteBuilder(
                      opaque: true,
                      pageBuilder: (_, __, ___) => BookScreen(state: state, workSlug: works[i].slug),
                    ),
                  ),
                ),
              ),
          ],
        );
      },
    );
  }
}

class _ContinueReading extends StatefulWidget {
  const _ContinueReading();

  @override
  State<_ContinueReading> createState() => _ContinueReadingState();
}

class _ContinueReadingState extends State<_ContinueReading> {
  late final Future<Map<String, dynamic>> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<Map<String, dynamic>> _load() async {
    // Injected via state would be cleaner; kept local so the Atrium stays
    // readable. The API is the same one the web rail uses.
    final api = _apiOf(context);
    return api.get('/me/currently-reading');
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Map<String, dynamic>>(
      future: _future,
      builder: (context, snap) {
        final rows = (snap.data?['currently_reading'] as List? ?? const []);
        if (rows.isEmpty) return const SizedBox.shrink();
        final first = rows.first as Map<String, dynamic>;
        final slug = first['work_slug'] as String? ?? '';
        final title = first['work_title'] as String? ?? '';
        final bp = (first['progress_bp'] as num?)?.toInt() ?? 0;
        return Container(
          margin: const EdgeInsets.only(bottom: 22),
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            border: Border.all(color: AlexandriaColors.ink, width: 1.5),
            borderRadius: BorderRadius.circular(AlexandriaMetrics.radiusNone),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(title, style: AlexType.body(fontSize: 19, fontWeight: FontWeight.w600, color: AlexandriaColors.ink)),
              Text(first['primary_author'] as String? ?? '', style: AlexType.body(fontSize: 14, color: AlexandriaColors.inkSoft)),
              const SizedBox(height: 10),
              Row(
                children: [
                  Expanded(child: ProgressTrack(fraction: bp / 10000)),
                  const SizedBox(width: 8),
                  Text('${(bp / 100).round()}%', style: AlexType.body(fontSize: 13, color: AlexandriaColors.inkSoft)),
                ],
              ),
              const SizedBox(height: 12),
              GestureDetector(
                onTap: () => Navigator.of(context).push(
                  PageRouteBuilder(opaque: true, pageBuilder: (_, __, ___) => BookScreen(state: _stateOf(context), workSlug: slug)),
                ),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                  decoration: BoxDecoration(
                    border: Border.all(color: AlexandriaColors.ink, width: 1.5),
                    borderRadius: BorderRadius.circular(AlexandriaMetrics.radiusNone),
                  ),
                  child: Text('Continue reading →', style: AlexType.body(fontSize: 14, color: AlexandriaColors.ink)),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}
