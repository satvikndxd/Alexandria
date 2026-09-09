import 'dart:math' as math;

import 'package:flutter/widgets.dart';

import '../theme/tokens.dart';
import '../theme/typography.dart';

/// DropCap — the illuminated initial, rendered with CustomPainter so the
/// botanical ornament is drawn (not rasterized) at any DPI, matching the
/// generative SVG asset system in apps/web pixel-for-pixel in spirit:
/// deterministic per letter, ink block with a rough hand-printed edge,
/// vermilion blackletter initial, forest-green vines, gold flowers.
class DropCap extends StatelessWidget {
  const DropCap({super.key, required this.letter, this.size = 96});

  final String letter;
  final double size;

  @override
  Widget build(BuildContext context) {
    final ch = letter.isEmpty ? 'A' : letter[0].toUpperCase();
    return Semantics(
      label: 'Illuminated initial $ch',
      // A semantics label is spoken; without a direction the framework
      // cannot read it (asserted since Flutter 3.x).
      textDirection: TextDirection.ltr,
      image: true,
      child: CustomPaint(
        size: Size.square(size),
        painter: _DropCapPainter(ch),
      ),
    );
  }
}

class _DropCapPainter extends CustomPainter {
  _DropCapPainter(this.letter) : seed = letter.codeUnitAt(0);

  final String letter;
  final int seed;

  @override
  void paint(Canvas canvas, Size size) {
    final s = size.width / 100.0; // design space is 100×100
    canvas.scale(s);

    _paintInkBlock(canvas);
    _paintVines(canvas);
    _paintLeaves(canvas);
    _paintFlowers(canvas);
    _paintSpeckle(canvas);
    _paintLetter(canvas);
  }

  /// Rough-edged ink block: a polygon whose edge vertices are jittered by a
  /// deterministic PRNG — the "hand-printed" imperfection of the reference.
  void _paintInkBlock(Canvas canvas) {
    final rng = math.Random(seed);
    final path = Path();
    const inset = 3.0;
    const step = 6;
    double jitter() => (rng.nextDouble() - 0.5) * 2.6;

    path.moveTo(inset + jitter(), inset + jitter());
    for (var x = step; x <= 94; x += step) {
      path.lineTo(inset + x + jitter(), inset + jitter());
    }
    for (var y = step; y <= 94; y += step) {
      path.lineTo(97 + jitter(), inset + y + jitter());
    }
    for (var x = 94 - step; x >= 0; x -= step) {
      path.lineTo(inset + x + jitter(), 97 + jitter());
    }
    for (var y = 94 - step; y >= 0; y -= step) {
      path.lineTo(inset + jitter(), inset + y + jitter());
    }
    path.close();
    canvas.drawPath(path, Paint()..color = AlexandriaColors.ink);

    // Inner hairline frame.
    canvas.drawRect(
      const Rect.fromLTWH(8.5, 8.5, 83, 83),
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 0.8
        ..color = AlexandriaColors.parchment.withValues(alpha: 0.5),
    );
  }

  void _paintVines(Canvas canvas) {
    final paint = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2.4
      ..strokeCap = StrokeCap.round
      ..color = AlexandriaColors.botanicalLight;

    final left = Path()..moveTo(12, 88);
    switch (seed % 3) {
      case 0:
        left
          ..cubicTo(8, 70, 20, 62, 14, 46)
          ..cubicTo(10, 34, 22, 28, 18, 14);
      case 1:
        left
          ..cubicTo(22, 76, 10, 60, 22, 48)
          ..cubicTo(32, 38, 20, 26, 30, 16);
      default:
        left
          ..cubicTo(6, 72, 26, 66, 16, 50)
          ..cubicTo(8, 38, 26, 34, 16, 18);
    }
    canvas.drawPath(left, paint);

    final right = Path()..moveTo(88, 14);
    if (seed.isEven) {
      right
        ..cubicTo(80, 26, 92, 34, 84, 46)
        ..cubicTo(78, 56, 90, 64, 84, 78);
    } else {
      right
        ..cubicTo(92, 28, 78, 36, 86, 50)
        ..cubicTo(92, 60, 80, 70, 88, 82);
    }
    canvas.drawPath(right, paint);
  }

  void _paintLeaves(Canvas canvas) {
    final leaf = Path()
      ..moveTo(0, 0)
      ..cubicTo(4, -6, 10, -6, 12, 0)
      ..cubicTo(10, 6, 4, 6, 0, 0)
      ..close();
    final paint = Paint()..color = AlexandriaColors.botanicalLight;

    final placements = <(double, double, double)>[
      (16, 62, -30 + (seed % 40).toDouble()),
      (20, 34, 20 + (seed % 30).toDouble()),
      (84, 40, 140 + (seed % 30).toDouble()),
      (82, 68, 200 - (seed % 40).toDouble()),
    ];
    for (final (x, y, deg) in placements) {
      canvas
        ..save()
        ..translate(x, y)
        ..rotate(deg * math.pi / 180)
        ..scale(0.9)
        ..drawPath(leaf, paint)
        ..restore();
    }
  }

  void _paintFlowers(Canvas canvas) {
    final petal = Paint()..color = AlexandriaColors.goldBright;
    final heart = Paint()..color = AlexandriaColors.vermilion;

    final placements = <(double, double, double)>[(18, 20, 0.9), (84, 24, 0.75), (16, 82, 0.7)];
    for (final (x, y, scale) in placements) {
      canvas
        ..save()
        ..translate(x, y)
        ..scale(scale);
      for (var i = 0; i < 5; i++) {
        canvas
          ..save()
          ..rotate(i * 72 * math.pi / 180)
          ..drawOval(Rect.fromCenter(center: const Offset(0, -4.2), width: 5.2, height: 8.4), petal)
          ..restore();
      }
      canvas
        ..drawCircle(Offset.zero, 2.2, heart)
        ..restore();
    }
  }

  void _paintSpeckle(Canvas canvas) {
    final paint = Paint()..color = AlexandriaColors.parchment.withValues(alpha: 0.35);
    for (var i = 0; i < 7; i++) {
      canvas.drawCircle(
        Offset(
          12 + ((seed * (i + 3)) % 76).toDouble(),
          10 + ((seed * (i + 7) * 13) % 80).toDouble(),
        ),
        0.9 + ((seed + i) % 3) * 0.35,
        paint,
      );
    }
  }

  void _paintLetter(Canvas canvas) {
    final painter = TextPainter(
      text: TextSpan(
        text: letter,
        style: AlexType.display(
          fontSize: 60,
          color: AlexandriaColors.vermilion,
        ),
      ),
      textDirection: TextDirection.ltr,
    )..layout();
    painter.paint(
      canvas,
      Offset(50 - painter.width / 2, 50 - painter.height / 2),
    );
  }

  @override
  bool shouldRepaint(_DropCapPainter old) => old.letter != letter;
}
