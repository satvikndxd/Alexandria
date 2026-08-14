import 'package:flutter_test/flutter_test.dart';
import 'package:alexandria_mobile/widgets/drop_cap.dart';

void main() {
  testWidgets('DropCap renders and exposes semantics', (tester) async {
    await tester.pumpWidget(const DropCap(letter: 'p'));
    final semantics = tester.getSemantics(find.byType(DropCap));
    expect(semantics.label, contains('P')); // uppercased
  });
}
