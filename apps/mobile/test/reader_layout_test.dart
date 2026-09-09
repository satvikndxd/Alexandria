import 'package:alexandria_mobile/core/reader_layout.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('paragraphs split on blank lines with rune offsets', () {
    final layout = ChapterLayout('One paragraph.\n\nTwo paragraph.');
    expect(layout.paragraphs, hasLength(2));
    expect(layout.paragraphs[0].start, 0);
    expect(layout.paragraphs[0].end, 14);
    expect(layout.paragraphs[1].start, 16);
    expect(layout.paragraphs[1].text, 'Two paragraph.');
  });

  test('non-BMP characters keep offsets rune-true (server-compatible)', () {
    // '𝔘' is U+1D558: two UTF-16 code units, ONE rune. A codeUnit-based
    // layout would report end=15 here and disagree with the server.
    final layout = ChapterLayout('𝔘nicode begin.\n\nSecond.');
    expect(layout.paragraphs[0].end, 14); // 13 chars + 1 = 14 runes
    expect(layout.paragraphs[1].start, 16);
  });

  test('paragraphsInRange finds intersecting paragraphs', () {
    final layout = ChapterLayout('aaa\n\nbbb\n\nccc');
    expect(layout.paragraphsInRange(0, 3).map((p) => p.text), contains('aaa'));
    expect(layout.paragraphsInRange(5, 8).map((p) => p.text), contains('bbb'));
    expect(layout.paragraphsInRange(0, 3).map((p) => p.text), isNot(contains('ccc')));
  });
}
