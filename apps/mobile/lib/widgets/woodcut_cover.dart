import 'dart:math' as math;

import 'package:flutter/widgets.dart';

import '../core/models.dart';
import '../theme/tokens.dart';
import '../theme/typography.dart';

/// License-safe generated cover in the house style, deterministic per title:
/// ink field, double hairline frame, illuminated initial, letterpress title.
/// Mirrors apps/web's WoodcutCover so a book looks like itself on every
/// platform until a rights-cleared cover exists.
class WoodcutCover extends StatelessWidget {
  const WoodcutCover({super.key, required this.title, required this.hue, this.width = 120});

  final String title;
  final CoverHue hue;
  final double width;

  @override
  Widget build(BuildContext context) {
    final colors = _palette(hue);
    return AspectRatio(
      aspectRatio: 2 / 3,
      child: Container(
        width: width,
        decoration: BoxDecoration(
          color: colors.field,
          border: Border.all(color: AlexandriaColors.ink, width: 2),
        ),
        child: CustomPaint(
          painter: _CoverPainter(title: title, accent: colors.accent),
          child: Padding(
            padding: const EdgeInsets.all(14),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  'PUBLIC DOMAIN',
                  style: AlexType.body(
                    fontSize: 7,
                    letterSpacing: 1.6,
                    color: colors.accent.withValues(alpha: 0.85),
                  ),
                ),
                Text(
                  title,
                  textAlign: TextAlign.center,
                  maxLines: 4,
                  overflow: TextOverflow.ellipsis,
                  style: AlexType.body(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    letterSpacing: 1.1,
                    color: colors.accent,
                  ),
                ),
                Text(
                  _initial(title),
                  style: AlexType.display(
                    fontSize: 34,
                    color: colors.accent,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  static String _initial(String title) {
    final t = title.replaceFirst(RegExp(r'^(The|A|An)\s+', caseSensitive: false), '');
    return t.isEmpty ? 'A' : t[0].toUpperCase();
  }

  static ({Color field, Color accent}) _palette(CoverHue hue) {
    switch (hue) {
      case CoverHue.vermilion:
        return (field: const Color(0xFFD9471F), accent: const Color(0xFFF0E7D2));
      case CoverHue.botanical:
        return (field: const Color(0xFF0D3B2E), accent: const Color(0xFFE8DDC4));
      case CoverHue.gold:
        return (field: const Color(0xFFC79522), accent: const Color(0xFF111713));
      case CoverHue.ink:
        return (field: const Color(0xFF111713), accent: const Color(0xFFE8DDC4));
    }
  }
}

class _CoverPainter extends CustomPainter {
  _CoverPainter({required this.title, required this.accent});
  final String title;
  final Color accent;

  @override
  void paint(Canvas canvas, Size size) {
    final frame = Paint()
      ..color = accent
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1;
    canvas.drawRect(Rect.fromLTWH(8, 8, size.width - 16, size.height - 16), frame);
    canvas.drawRect(
      Rect.fromLTWH(12, 12, size.width - 24, size.height - 24),
      frame..strokeWidth = 0.6,
    );
    // Deterministic speckle: the hand-printed imperfection, seeded by title.
    final rng = math.Random(title.hashCode);
    final speckle = Paint()..color = accent.withValues(alpha: 0.25);
    for (var i = 0; i < 14; i++) {
      canvas.drawCircle(
        Offset(16 + rng.nextDouble() * (size.width - 32), 16 + rng.nextDouble() * (size.height - 32)),
        0.9,
        speckle,
      );
    }
  }

  @override
  bool shouldRepaint(covariant _CoverPainter old) => old.title != title;
}
