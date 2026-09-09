import 'package:flutter/widgets.dart';

import '../app_state.dart';
import '../core/api.dart';
import '../theme/tokens.dart';
import '../theme/typography.dart';
import '../widgets/ink_field.dart';
import '../widgets/ornament.dart';

/// Magic-link sign-in: the passkey ceremony needs a platform channel per OS,
/// so mobile leads with the fallback factor that needs nothing but email.
/// The link is pasted rather than deep-linked in this milestone; deep links
/// arrive with the release hardening.
class AuthScreen extends StatefulWidget {
  const AuthScreen({super.key, required this.state});

  final AppState state;

  @override
  State<AuthScreen> createState() => _AuthScreenState();
}

class _AuthScreenState extends State<AuthScreen> {
  final _email = TextEditingController();
  final _token = TextEditingController();
  String? _notice;
  String? _error;
  bool _busy = false;

  Future<void> _request() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await widget.state.api.post('/auth/magic-link', {'email': _email.text.trim()});
      setState(() => _notice = 'If that address has an account, a link is on its way. Paste it below.');
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      setState(() => _busy = false);
    }
  }

  Future<void> _verify() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await widget.state.api.post('/auth/magic-link/verify', {'token': _token.text.trim()});
      await widget.state.refreshMe();
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Sign in', style: AlexType.body(fontSize: 28, color: AlexandriaColors.ink)),
          const SizedBox(height: 8),
          const FleuronRule(),
          const SizedBox(height: 16),
          Text('Email', style: AlexType.body(fontSize: 12, letterSpacing: 2, color: AlexandriaColors.inkFaint)),
          const SizedBox(height: 4),
          _field(_email, 'you@example.com'),
          const SizedBox(height: 10),
          _button(_busy ? 'Sealing…' : 'Send a sign-in link', _request),
          const SizedBox(height: 18),
          Text('Link token', style: AlexType.body(fontSize: 12, letterSpacing: 2, color: AlexandriaColors.inkFaint)),
          const SizedBox(height: 4),
          _field(_token, 'paste the one-time link or token'),
          const SizedBox(height: 10),
          _button(_busy ? 'Opening…' : 'Open the link', _verify),
          if (_notice != null) ...[
            const SizedBox(height: 12),
            Text(_notice!, style: AlexType.body(fontSize: 14, fontStyle: FontStyle.italic, color: AlexandriaColors.inkSoft)),
          ],
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(_error!, style: AlexType.body(fontSize: 14, color: AlexandriaColors.vermilion)),
          ],
        ],
      ),
    );
  }

  Widget _field(TextEditingController c, String hint) => InkField(controller: c, hint: hint);

  Widget _button(String label, VoidCallback onTap) => GestureDetector(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
          decoration: BoxDecoration(
            color: AlexandriaColors.ink,
            border: Border.all(color: AlexandriaColors.ink, width: 1.5),
            borderRadius: BorderRadius.circular(AlexandriaMetrics.radiusNone),
          ),
          child: Text(label, style: AlexType.body(fontSize: 15, color: AlexandriaColors.parchmentLight)),
        ),
      );
}
