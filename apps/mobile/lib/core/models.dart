/// Wire models for the Alexandria API. Kept small and parse-strict: a field
/// the server stops sending is a loud null, not a silent default.
enum CoverHue { vermilion, botanical, gold, ink }

CoverHue hueForSlug(String slug) {
  var h = 0;
  for (final u in slug.codeUnits) {
    h = (h * 31 + u) & 0x7fffffff;
  }
  return CoverHue.values[h % CoverHue.values.length];
}

class Work {
  Work.fromJson(Map<String, dynamic> j)
      : id = j['id'] as String? ?? '',
        slug = j['slug'] as String? ?? '',
        title = j['title'] as String? ?? '',
        description = j['description'] as String? ?? '',
        firstPublished = (j['first_published'] as num?)?.toInt(),
        isPublicDomain = j['is_public_domain'] as bool? ?? false,
        ratingCount = (j['rating_count'] as num?)?.toInt() ?? 0,
        ratingSum = (j['rating_sum'] as num?)?.toInt() ?? 0,
        authorName = _firstAuthor(j),
        coverHue = hueForSlug(j['slug'] as String? ?? '');

  final String id;
  final String slug;
  final String title;
  final String description;
  final int? firstPublished;
  final bool isPublicDomain;
  final int ratingCount;
  final int ratingSum;
  final String authorName;
  final CoverHue coverHue;

  double get averageRating => ratingCount == 0 ? 0 : ratingSum / ratingCount / 2;

  static String _firstAuthor(Map<String, dynamic> j) {
    final a = j['authors'];
    if (a is List && a.isNotEmpty && a.first is Map) {
      return (a.first as Map)['name'] as String? ?? '';
    }
    if (j['primary_author'] is String) return j['primary_author'] as String;
    return '';
  }
}

class LibraryItem {
  LibraryItem.fromJson(Map<String, dynamic> j)
      : workSlug = j['work_slug'] as String? ?? '',
        workTitle = j['work_title'] as String? ?? '',
        shelfKind = j['shelf_kind'] as String? ?? '',
        progressBp = (j['progress_bp'] as num?)?.toInt() ?? 0,
        primaryAuthor = j['primary_author'] as String? ?? '',
        coverHue = hueForSlug(j['work_slug'] as String? ?? '');

  final String workSlug;
  final String workTitle;
  final String shelfKind;
  final int progressBp;
  final String primaryAuthor;
  final CoverHue coverHue;
}

class Shelf {
  Shelf.fromJson(Map<String, dynamic> j)
      : id = j['id'] as String? ?? '',
        kind = j['kind'] as String? ?? '',
        name = j['name'] as String? ?? '',
        itemCount = (j['item_count'] as num?)?.toInt() ?? 0;

  final String id;
  final String kind;
  final String name;
  final int itemCount;
}

class ReaderChapter {
  ReaderChapter.fromJson(Map<String, dynamic> j)
      : index = (j['index'] as num?)?.toInt() ?? 0,
        title = j['title'] as String? ?? '';

  final int index;
  final String title;
}

class Annotation {
  Annotation.fromJson(Map<String, dynamic> j)
      : id = j['id'] as String? ?? '',
        kind = j['kind'] as String? ?? '',
        body = j['body'] as String? ?? '',
        start = (j['start_off'] as num?)?.toInt() ?? 0,
        end = (j['end_off'] as num?)?.toInt() ?? 0;

  final String id;
  final String kind;
  final String body;
  final int start;
  final int end;
}
