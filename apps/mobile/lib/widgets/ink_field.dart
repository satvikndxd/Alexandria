import 'package:flutter/widgets.dart';

import '../theme/tokens.dart';
import '../theme/typography.dart';

/// A bordered text entry built on EditableText — the widgets-only analogue of
/// Material's TextField, styled as a ruled panel in the manuscript system.
class InkField extends StatefulWidget {
  const InkField({super.key, required this.controller, this.hint = '', this.obscure = false});

  final TextEditingController controller;
  final String hint;
  final bool obscure;

  @override
  State<InkField> createState() => _InkFieldState();
}

class _InkFieldState extends State<InkField> {
  final FocusNode _focus = FocusNode();

  @override
  void dispose() {
    _focus.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final empty = widget.controller.text.isEmpty;
    return GestureDetector(
      onTap: () => _focus.requestFocus(),
      child: Container(
        padding: const EdgeInsets.all(10),
        decoration: BoxDecoration(
          border: Border.all(color: AlexandriaColors.ink, width: 1.5),
          borderRadius: BorderRadius.circular(AlexandriaMetrics.radiusNone),
        ),
        child: Stack(
          children: [
            if (empty)
              Text(
                widget.hint,
                style: AlexType.body(fontSize: 14, color: AlexandriaColors.inkFaint),
              ),
            EditableText(
              controller: widget.controller,
              focusNode: _focus,
              obscureText: widget.obscure,
              style: AlexType.body(fontSize: 16, color: AlexandriaColors.ink),
              cursorColor: AlexandriaColors.vermilion,
              backgroundCursorColor: AlexandriaColors.inkFaint,
              onChanged: (_) => setState(() {}),
            ),
          ],
        ),
      ),
    );
  }
}
