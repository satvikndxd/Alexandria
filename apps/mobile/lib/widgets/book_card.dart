import 'package:flutter/widgets.dart';

import '../core/models.dart';
import '../theme/tokens.dart';
import '../theme/typography.dart';
import 'woodcut_cover.dart';

/// The book plate: cover as anchor, title in serif, author beneath.
/// No chrome, no badges — the cover is the visual anchor.
class BookCard extends StatelessWidget {
  const BookCard({super.key, required this.work, this.onTap});

  final Work work;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Semantics(
        button: onTap != null,
        label: '${work.title} by ${work.authorName}',
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            WoodcutCover(title: work.title, hue: work.coverHue),
            const SizedBox(height: 8),
            Text(
              work.title,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: AlexType.body(
                fontSize: 15,
                fontWeight: FontWeight.w600,
                color: AlexandriaColors.ink,
              ),
            ),
            const SizedBox(height: 2),
            Text(
              work.authorName,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AlexType.body(
                fontSize: 13,
                color: AlexandriaColors.inkFaint,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
