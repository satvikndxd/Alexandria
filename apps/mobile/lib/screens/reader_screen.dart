import 'package:flutter/widgets.dart';

import '../app_state.dart';
import '../core/api.dart';
import '../core/models.dart';
import '../core/reader_layout.dart';
import '../theme/tokens.dart';
import '../theme/typography.dart';
import '../widgets/ink_field.dart';
import '../widgets/ornament.dart';

/// The reader: a typeset edition with the reader's own typography, and a
/// margin that is private by database law.
///
/// Annotations anchor as (chapter, rune-start, rune-end) over the exact string
/// the server served — [ChapterLayout] keeps those offsets rune-true, so a
/// highlight made here is the same highlight on the web.
class ReaderScreen extends StatefulWidget {
  const ReaderScreen({super.key, required this.state, required this.editionId});

  final AppState state;
  final String editionId;

  @override
  State<ReaderScreen> createState() => _ReaderScreenState();
}

class _ReaderScreenState extends State<ReaderScreen> {
  late final Future<Map<String, dynamic>> _meta;
  int _chapter = 0;
  List<Annotation> _annotations = const [];
  Paragraph? _composer;
  TextEditingController? _composerNote;

  @override
  void initState() {
    super.initState();
    _meta = widget.state.api.get('/editions/${widget.editionId}/reader');
  }

  ({Color bg, Color fg}) _themeColors() {
    switch (widget.state.prefs.theme) {
      case ReaderTheme.parchment:
        return (bg: const Color(0xFFECE5D3), fg: AlexandriaColors.ink);
      case ReaderTheme.sepia:
        return (bg: const Color(0xFFE4D6BC), fg: const Color(0xFF2A2418));
      case ReaderTheme.ink:
        return (bg: AlexandriaColors.ink, fg: const Color(0xFFE8DDC4));
    }
  }

  Future<void> _loadAnnotations() async {
    if (!widget.state.signedIn) return;
    try {
      final res = await widget.state.api
          .get('/me/annotations', {'edition_id': widget.editionId, 'chapter_idx': '$_chapter'});
      if (!mounted) return;
      setState(() {
        _annotations = (res['annotations'] as List? ?? const [])
            .map((a) => Annotation.fromJson(a as Map<String, dynamic>))
            .toList();
      });
    } on ApiException {
      // A private margin that cannot load is simply absent; never an error
      // screen between the reader and the text.
    }
  }

  /// Long-press opens an inline composer (no Material dialogs in this app):
  /// the paragraph is quoted, the note is optional, and the anchors are the
  /// paragraph's rune offsets in the served chapter string.
  void _annotate(Paragraph p) {
    if (!widget.state.signedIn) return;
    setState(() {
      _composer = p;
      _composerNote = TextEditingController();
    });
  }

  Future<void> _keepNote() async {
    final p = _composer;
    final note = _composerNote;
    if (p == null || note == null) return;
    setState(() => _composer = null);
    try {
      await widget.state.api.post('/me/annotations', {
        'edition_id': widget.editionId,
        'chapter_idx': _chapter,
        'start_off': p.start,
        'end_off': p.end,
        'kind': 'note',
        'body': note.text,
      });
      await _loadAnnotations();
    } on ApiException {
      // refused: the server's friction rules apply on mobile too
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = _themeColors();
    return ColoredBox(
      color: colors.bg,
      child: SafeArea(
        child: FutureBuilder<Map<String, dynamic>>(
          future: _meta,
          builder: (context, metaSnap) {
            final chapters = (metaSnap.data?['chapters'] as List? ?? const [])
                .map((c) => ReaderChapter.fromJson(c as Map<String, dynamic>))
                .toList();
            return Column(
              children: [
                _controls(colors.fg, chapters),
                const InkRule(color: AlexandriaColors.inkFaint),
                if (_composer != null) _composerBar(colors.fg),
                Expanded(
                  child: FutureBuilder<Map<String, dynamic>>(
                    future: widget.state.api.get('/editions/${widget.editionId}/reader/chapter/$_chapter'),
                    builder: (context, snap) {
                      if (snap.hasError || snap.data == null) {
                        return const Center(child: Text('The text is momentarily unreachable.'));
                      }
                      final text = snap.data!['text'] as String? ?? '';
                      final layout = ChapterLayout(text);
                      WidgetsBinding.instance.addPostFrameCallback((_) => _loadAnnotations());
                      return ListView(
                        padding: const EdgeInsets.fromLTRB(20, 24, 20, 40),
                        children: [
                          if (chapters.isNotEmpty && _chapter < chapters.length)
                            Text(
                              chapters[_chapter].title,
                              style: AlexType.body(fontSize: 20, fontWeight: FontWeight.w600, color: colors.fg),
                            ),
                          const SizedBox(height: 12),
                          for (final p in layout.paragraphs)
                            _paragraph(p, colors.fg),
                          const SizedBox(height: 20),
                          FleuronRule(color: colors.fg),
                          const SizedBox(height: 12),
                          Text(
                            metaSnap.data?['license_note'] as String? ?? '',
                            style: AlexType.body(fontSize: 10, letterSpacing: 1.2, color: colors.fg.withValues(alpha: 0.7)),
                          ),
                        ],
                      );
                    },
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _paragraph(Paragraph p, Color fg) {
    final marked = _annotations.any((a) => a.start < p.end && a.end > p.start);
    return GestureDetector(
      onLongPress: () => _annotate(p),
      child: Container(
        margin: const EdgeInsets.only(bottom: 14),
        decoration: marked
            ? BoxDecoration(
                border: Border(left: BorderSide(color: AlexandriaColors.gold, width: 3)),
              )
            : null,
        padding: marked ? const EdgeInsets.only(left: 10) : null,
        child: Text(
          p.text,
          style: AlexType.body(
            fontSize: widget.state.prefs.fontSize,
            height: widget.state.prefs.lineHeight,
            color: fg,
          ),
        ),
      ),
    );
  }

  Widget _composerBar(Color fg) {
    final p = _composer!;
    return Container(
      padding: const EdgeInsets.all(12),
      color: AlexandriaColors.parchmentLight,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '“${p.text.length > 90 ? '${p.text.substring(0, 90)}…' : p.text}”',
            style: AlexType.body(fontSize: 13, fontStyle: FontStyle.italic, color: AlexandriaColors.inkSoft),
          ),
          const SizedBox(height: 8),
          InkField(controller: _composerNote!, hint: 'margin note (optional)'),
          const SizedBox(height: 8),
          Row(
            children: [
              GestureDetector(
                onTap: _keepNote,
                child: Text('Keep', style: AlexType.body(fontSize: 15, color: AlexandriaColors.botanical)),
              ),
              const SizedBox(width: 16),
              GestureDetector(
                onTap: () => setState(() => _composer = null),
                child: Text('Cancel', style: AlexType.body(fontSize: 15, color: AlexandriaColors.inkFaint)),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _controls(Color fg, List<ReaderChapter> chapters) {
    final prefs = widget.state.prefs;
    TextStyle label = AlexType.body(fontSize: 13, color: fg);
    Widget btn(String text, VoidCallback onTap) => GestureDetector(
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 10),
            child: Text(text, style: label),
          ),
        );
    return SizedBox(
      height: 44,
      child: ListView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 12),
        children: [
          btn('A−', () => widget.state.tweakPrefs((p) => p.fontSize = (p.fontSize - 1).clamp(13, 30))),
          btn('A+', () => widget.state.tweakPrefs((p) => p.fontSize = (p.fontSize + 1).clamp(13, 30))),
          btn('leading ${prefs.lineHeight.toStringAsFixed(2)}',
              () => widget.state.tweakPrefs((p) => p.lineHeight = p.lineHeight >= 2.1 ? 1.4 : p.lineHeight + 0.1)),
          btn(prefs.theme.name, () => widget.state.tweakPrefs((p) => p.theme =
              ReaderTheme.values[(p.theme.index + 1) % ReaderTheme.values.length])),
          if (chapters.length > 1)
            btn('ch ${_chapter + 1}/${chapters.length}', () async {
              final next = (_chapter + 1) % chapters.length;
              setState(() => _chapter = next);
            }),
        ],
      ),
    );
  }
}
