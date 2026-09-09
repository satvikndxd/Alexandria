import 'package:flutter/widgets.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../theme/tokens.dart';
import '../theme/typography.dart';
import '../widgets/ornament.dart';
import 'book_screen.dart';

/// The reader's shelves, RLS-private by server law; this screen can only ever
/// show the signed-in reader's own rows.
class LibraryScreen extends StatefulWidget {
  const LibraryScreen({super.key, required this.state});

  final AppState state;

  @override
  State<LibraryScreen> createState() => _LibraryScreenState();
}

class _LibraryScreenState extends State<LibraryScreen> {
  String _shelf = 'reading';

  @override
  Widget build(BuildContext context) {
    if (!widget.state.signedIn) {
      return const Center(child: Text('Sign in to see your shelves.'));
    }
    return Column(
      children: [
        SizedBox(
          height: 44,
          child: ListView(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 16),
            children: [
              for (final kind in const ['reading', 'want_to_read', 'read', 'dnf', 'favorites'])
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 4),
                  child: GestureDetector(
                    onTap: () => setState(() => _shelf = kind),
                    child: Container(
                      alignment: Alignment.center,
                      padding: const EdgeInsets.symmetric(horizontal: 12),
                      decoration: BoxDecoration(
                        color: _shelf == kind ? AlexandriaColors.ink : null,
                        border: Border.all(color: AlexandriaColors.ink, width: 1),
                        borderRadius: BorderRadius.circular(AlexandriaMetrics.radiusNone),
                      ),
                      child: Text(
                        kind.replaceAll('_', ' '),
                        style: AlexType.body(
                          fontSize: 13,
                          color: _shelf == kind ? AlexandriaColors.parchmentLight : AlexandriaColors.ink,
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
        const InkRule(color: AlexandriaColors.ink),
        Expanded(
          child: FutureBuilder<Map<String, dynamic>>(
            future: widget.state.api.get('/me/library', {'shelf': _shelf, 'limit': '48'}),
            builder: (context, snap) {
              final items = (snap.data?['items'] as List? ?? const [])
                  .map((i) => LibraryItem.fromJson(i as Map<String, dynamic>))
                  .toList();
              if (items.isEmpty) {
                return Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Text(
                      'This shelf is empty. Shelve something from a book page and it will keep its place here.',
                      textAlign: TextAlign.center,
                      style: AlexType.body(fontSize: 15, fontStyle: FontStyle.italic, color: AlexandriaColors.inkFaint),
                    ),
                  ),
                );
              }
              return ListView.separated(
                padding: const EdgeInsets.all(16),
                itemCount: items.length,
                separatorBuilder: (_, __) => const Padding(
                  padding: EdgeInsets.symmetric(vertical: 10),
                  child: FleuronRule(),
                ),
                itemBuilder: (context, i) {
                  final it = items[i];
                  return GestureDetector(
                    onTap: () => Navigator.of(context).push(
                      PageRouteBuilder(
                        opaque: true,
                        pageBuilder: (_, __, ___) => BookScreen(state: widget.state, workSlug: it.workSlug),
                      ),
                    ),
                    child: Row(
                      children: [
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(it.workTitle, style: AlexType.body(fontSize: 17, fontWeight: FontWeight.w600, color: AlexandriaColors.ink)),
                              Text(it.primaryAuthor, style: AlexType.body(fontSize: 13, color: AlexandriaColors.inkFaint)),
                              if (it.progressBp > 0) ...[
                                const SizedBox(height: 6),
                                ProgressTrack(fraction: it.progressBp / 10000),
                              ],
                            ],
                          ),
                        ),
                      ],
                    ),
                  );
                },
              );
            },
          ),
        ),
      ],
    );
  }
}
