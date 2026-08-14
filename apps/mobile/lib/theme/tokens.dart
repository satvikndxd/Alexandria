import 'dart:ui';

/// Alexandria design tokens — must stay in lockstep with
/// apps/web/tailwind.config.ts and packages/core/tokens.json.
abstract final class AlexandriaColors {
  static const parchment = Color(0xFFE8DDC4);
  static const parchmentLight = Color(0xFFF0E7D2);
  static const parchmentDark = Color(0xFFD9CCAE);

  static const ink = Color(0xFF111713);
  static const inkSoft = Color(0xFF2A322B);
  static const inkFaint = Color(0xFF4A5348);

  static const botanical = Color(0xFF0D3B2E);
  static const botanicalMid = Color(0xFF124A38);
  static const botanicalLight = Color(0xFF1B5B42);

  static const vermilion = Color(0xFFD9471F);
  static const vermilionBright = Color(0xFFE84B1C);

  static const gold = Color(0xFFC79522);
  static const goldBright = Color(0xFFD4A62A);
}

abstract final class AlexandriaMetrics {
  /// The only two radii in the system: sharp, and a 2px "nick".
  static const radiusNone = 0.0;
  static const radiusNick = 2.0;

  /// Hard print-block shadow offset (never blurred).
  static const blockOffset = Offset(4, 4);

  static const letterSpacingEngraved = 2.4;
}
