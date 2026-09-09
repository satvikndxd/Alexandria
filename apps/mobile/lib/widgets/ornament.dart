import 'package:flutter/widgets.dart';

import '../theme/tokens.dart';

/// Fleuron chapter break: hairline · diamond-and-leaf · hairline, drawn —
/// never rasterized — so it stays crisp at any DPI.
class FleuronRule extends StatelessWidget {
  const FleuronRule({super.key, this.color = AlexandriaColors.ink});

  final Color color;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 14,
      child: CustomPaint(
        painter: _FleuronPainter(color),
        child: const SizedBox.expand(),
      ),
    );
  }
}

class _FleuronPainter extends CustomPainter {
  _FleuronPainter(this.color);
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final mid = size.height / 2;
    final paint = Paint()
      ..color = color
      ..strokeWidth = 1;
    canvas.drawLine(Offset(0, mid), Offset(size.width / 2 - 12, mid), paint);
    canvas.drawLine(Offset(size.width / 2 + 12, mid), Offset(size.width, mid), paint);

    final diamond = Path()
      ..moveTo(size.width / 2, mid - 5)
      ..lineTo(size.width / 2 + 4, mid)
      ..lineTo(size.width / 2, mid + 5)
      ..lineTo(size.width / 2 - 4, mid)
      ..close();
    canvas.drawPath(diamond, Paint()..color = color);

    for (final dir in const [-1.0, 1.0]) {
      final leaf = Path()
        ..moveTo(size.width / 2 + dir * 6, mid)
        ..quadraticBezierTo(
          size.width / 2 + dir * 10,
          mid - 4,
          size.width / 2 + dir * 12,
          mid,
        )
        ..quadraticBezierTo(
          size.width / 2 + dir * 10,
          mid + 4,
          size.width / 2 + dir * 6,
          mid,
        )
        ..close();
      canvas.drawPath(leaf, Paint()..color = color);
    }
  }

  @override
  bool shouldRepaint(covariant _FleuronPainter old) => old.color != color;
}

/// Ruled progress track: a hairline box with a solid ink fill, matching the
/// web reader's progress rail exactly in spirit.
class ProgressTrack extends StatelessWidget {
  const ProgressTrack({super.key, required this.fraction, this.height = 7});

  final double fraction; // 0..1
  final double height;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: height,
      decoration: BoxDecoration(
        border: Border.all(color: AlexandriaColors.ink, width: 1),
        borderRadius: BorderRadius.circular(AlexandriaMetrics.radiusNone),
      ),
      child: Align(
        alignment: Alignment.centerLeft,
        child: FractionallySizedBox(
          widthFactor: fraction.clamp(0.0, 1.0),
          child: const ColoredBox(color: AlexandriaColors.ink),
        ),
      ),
    );
  }
}

/// A hairline rule: the mobile stand-in for every <hr> in the folio.
/// Widgets-only (no Material Divider), 1px, ink — never blurred, never fat.
class InkRule extends StatelessWidget {
  const InkRule({super.key, this.thickness = 1, this.color = AlexandriaColors.ink});

  final double thickness;
  final Color color;

  @override
  Widget build(BuildContext context) => Container(height: thickness, color: color);
}
