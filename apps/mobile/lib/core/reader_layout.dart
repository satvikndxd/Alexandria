/// Chapter layout with RUNE-true paragraph offsets.
///
/// The server anchors annotations as (chapter_idx, start_off, end_off) in
/// runes over the served chapter string. Dart strings are UTF-16, so naive
/// codeUnit offsets would drift on any non-BMP character; this layout walks
/// runes and reports rune offsets, keeping mobile highlights aligned with
/// web highlights forever.
class Paragraph {
  const Paragraph(this.start, this.end, this.text);
  final int start; // rune offset into the chapter
  final int end; // rune offset, exclusive
  final String text;
}

class ChapterLayout {
  ChapterLayout(this.text) : paragraphs = _split(text);

  final String text;
  final List<Paragraph> paragraphs;

  static List<Paragraph> _split(String text) {
    final runes = text.runes.toList(growable: false);
    final out = <Paragraph>[];
    var start = 0;
    var i = 0;
    void flush(int end) {
      if (end > start) {
        out.add(Paragraph(start, end, String.fromCharCodes(runes.sublist(start, end))));
      }
    }

    while (i < runes.length) {
      if (runes[i] == 0x0A && i + 1 < runes.length && runes[i + 1] == 0x0A) {
        flush(i);
        i += 2;
        start = i;
        continue;
      }
      i++;
    }
    flush(runes.length);
    return out;
  }

  /// Paragraphs intersecting a rune range — used to paint highlights.
  Iterable<Paragraph> paragraphsInRange(int start, int end) =>
      paragraphs.where((p) => p.start < end && p.end > start);
}
