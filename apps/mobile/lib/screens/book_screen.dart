import 'package:flutter/widgets.dart';

import '../app_scope.dart';
import '../app_state.dart';
import '../core/api.dart';
import '../core/models.dart';
import '../theme/tokens.dart';
import '../theme/typography.dart';
import '../widgets/ornament.dart';
import '../widgets/woodcut_cover.dart';
import 'reader_screen.dart';

/// The book page: the object first (cover, title, rating), then editions,
/// then shelf actions, then the reader door when a public-domain text exists.
class BookScreen extends StatelessWidget {
  const BookScreen({super.key, required this.state, required this.workSlug});

  final AppState state;
  final String workSlug;

  static const _shelves = ['want_to_read', 'reading', 'read', 'dnf', 'favorites'];

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Map<String, dynamic>>(
      future: state.api.get('/works/$workSlug'),
      builder: (context, snap) {
        if (snap.hasError) {
          return const Center(child: Text('The archive is momentarily unreachable.'));
        }
        final data = snap.data;
        if (data == null) return const SizedBox.shrink();
        final work = Work.fromJson(data['work'] as Map<String, dynamic>);
        final editions = (data['editions'] as List? ?? const [])
            .map((e) => e as Map<String, dynamic>)
            .toList();
        final readable = editions.firstWhere(
          (e) => e['gutenberg_id'] != null,
          orElse: () => const <String, dynamic>{},
        );

        return ListView(
          padding: const EdgeInsets.all(20),
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                WoodcutCover(title: work.title, hue: work.coverHue, width: 110),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(work.title, style: AlexType.body(fontSize: 24, color: AlexandriaColors.ink)),
                      const SizedBox(height: 4),
                      Text(work.authorName, style: AlexType.body(fontSize: 15, fontStyle: FontStyle.italic, color: AlexandriaColors.inkSoft)),
                      const SizedBox(height: 8),
                      Text(
                        work.ratingCount == 0
                            ? 'no ratings yet'
                            : '${work.averageRating.toStringAsFixed(1)} · ${work.ratingCount} ratings',
                        style: AlexType.body(fontSize: 13, color: AlexandriaColors.inkFaint),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            const FleuronRule(),
            const SizedBox(height: 16),
            Text(work.description, style: AlexType.body(fontSize: 16, height: 1.6, color: AlexandriaColors.inkSoft)),
            if (state.signedIn) ...[
              const SizedBox(height: 18),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final shelf in _shelves)
                    _ShelfButton(state: state, workSlug: workSlug, kind: shelf),
                ],
              ),
            ],
            if (readable.isNotEmpty) ...[
              const SizedBox(height: 18),
              GestureDetector(
                onTap: () => Navigator.of(context).push(
                  PageRouteBuilder(
                    opaque: true,
                    pageBuilder: (_, __, ___) => ReaderScreen(
                      state: AppStateScope.of(context),
                      editionId: readable['id'] as String,
                    ),
                  ),
                ),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                  decoration: BoxDecoration(
                    color: AlexandriaColors.ink,
                    borderRadius: BorderRadius.circular(AlexandriaMetrics.radiusNone),
                  ),
                  child: Text(
                    'Read the public-domain edition',
                    style: AlexType.body(fontSize: 15, color: AlexandriaColors.parchmentLight),
                  ),
                ),
              ),
            ],
          ],
        );
      },
    );
  }
}

class _ShelfButton extends StatefulWidget {
  const _ShelfButton({required this.state, required this.workSlug, required this.kind});

  final AppState state;
  final String workSlug;
  final String kind;

  @override
  State<_ShelfButton> createState() => _ShelfButtonState();
}

class _ShelfButtonState extends State<_ShelfButton> {
  bool _busy = false;

  Future<void> _shelve() async {
    setState(() => _busy = true);
    try {
      await widget.state.api.post('/me/library', {
        'work_slug': widget.workSlug,
        'shelf_kind': widget.kind,
      });
    } on ApiException {
      // A refused shelf move is shown by the server's message in the web
      // client; mobile keeps the button honest by simply not claiming success.
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: _busy ? null : _shelve,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
        decoration: BoxDecoration(
          border: Border.all(color: AlexandriaColors.ink, width: 1.5),
          borderRadius: BorderRadius.circular(AlexandriaMetrics.radiusNone),
        ),
        child: Text(
          widget.kind.replaceAll('_', ' '),
          style: AlexType.body(fontSize: 13, color: AlexandriaColors.ink),
        ),
      ),
    );
  }
}
