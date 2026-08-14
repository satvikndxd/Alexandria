import 'package:flutter/widgets.dart';
import 'package:google_fonts/google_fonts.dart';

import 'theme/tokens.dart';
import 'widgets/drop_cap.dart';

void main() => runApp(const AlexandriaApp());

/// App shell. Deliberately built on WidgetsApp, not MaterialApp:
/// Alexandria's UI is drawn from its own design system — no Material
/// ink ripples, no Cupertino chrome, no default SaaS look to fight.
class AlexandriaApp extends StatelessWidget {
  const AlexandriaApp({super.key});

  @override
  Widget build(BuildContext context) {
    return WidgetsApp(
      title: 'Alexandria',
      color: AlexandriaColors.ink,
      builder: (context, _) => const _Atrium(),
    );
  }
}

class _Atrium extends StatelessWidget {
  const _Atrium();

  @override
  Widget build(BuildContext context) {
    final body = GoogleFonts.ebGaramond(
      fontSize: 17,
      height: 1.55,
      color: AlexandriaColors.inkSoft,
    );

    return ColoredBox(
      color: AlexandriaColors.parchment,
      child: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(
                child: Text(
                  'Alexandria',
                  style: GoogleFonts.unifrakturMaguntia(
                    fontSize: 40,
                    color: AlexandriaColors.ink,
                  ),
                ),
              ),
              const SizedBox(height: 4),
              Center(
                child: Text(
                  'A HUMAN LIBRARY',
                  style: GoogleFonts.ebGaramond(
                    fontSize: 11,
                    letterSpacing: AlexandriaMetrics.letterSpacingEngraved,
                    color: AlexandriaColors.botanical,
                  ),
                ),
              ),
              const SizedBox(height: 8),
              const _DoubleRule(),
              const SizedBox(height: 24),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const DropCap(letter: 'B', size: 84),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      'ooks are written by humans, discussed by humans, '
                      'explained by humans, and read by humans.',
                      style: body,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _DoubleRule extends StatelessWidget {
  const _DoubleRule();

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Container(height: 1, color: AlexandriaColors.ink),
        const SizedBox(height: 2),
        Container(height: 1, color: AlexandriaColors.ink),
      ],
    );
  }
}
