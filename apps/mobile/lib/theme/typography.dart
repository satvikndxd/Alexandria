import 'package:flutter/widgets.dart';

import 'tokens.dart';

/// The two bundled OFL families, exposed with the same call shape the code
/// already used. Fonts are ASSETS, never runtime fetches: the app renders
/// identically offline, and no third party learns which book is open.
abstract final class AlexType {
  static TextStyle body({
    double fontSize = 16,
    double height = 1.55,
    Color color = AlexandriaColors.ink,
    FontWeight? fontWeight,
    FontStyle? fontStyle,
    double? letterSpacing,
  }) =>
      TextStyle(
        fontFamily: 'EBGaramond',
        fontSize: fontSize,
        height: height,
        color: color,
        fontWeight: fontWeight,
        fontStyle: fontStyle,
        letterSpacing: letterSpacing,
      );

  static TextStyle display({
    double fontSize = 32,
    Color color = AlexandriaColors.ink,
    double? letterSpacing,
  }) =>
      TextStyle(
        fontFamily: 'UnifrakturMaguntia',
        fontSize: fontSize,
        color: color,
        letterSpacing: letterSpacing,
      );
}
