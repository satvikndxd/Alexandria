/**
 * Data layer for the web client.
 *
 * In production this proxies the Go API (ALEXANDRIA_API_URL). In local dev and
 * in this demo deployment it serves seeded fixtures of public-domain classics
 * so every page renders honestly — no fake activity, no invented scholars:
 * fixture reviews/notes are clearly marked as demonstration seeds.
 */

export interface Author {
  id: string;
  name: string;
  years: string;
}

export interface Work {
  slug: string;
  title: string;
  author: Author;
  firstPublished: number;
  description: string;
  isPublicDomain: boolean;
  subjects: string[];
  rating: number; // 0..5 in half-star steps
  ratingCount: number;
  gutenbergId?: number;
  coverHue: "vermilion" | "botanical" | "gold" | "ink";
}

export interface Review {
  id: string;
  workSlug: string;
  username: string;
  displayName: string;
  rating: number;
  title: string;
  body: string;
  hasSpoilers: boolean;
  likeCount: number;
  createdAt: string;
  isSeed: boolean;
}

export interface ScholarNote {
  id: string;
  workSlug: string;
  scholar: string;
  field: string;
  kind: "context" | "linguistic" | "historical" | "interpretive";
  chapterRef: string;
  title: string;
  body: string;
  citations: string[];
  isSeed: boolean;
}

export interface Club {
  slug: string;
  name: string;
  description: string;
  members: number;
  currentRead: string; // work slug
  channels: { name: string; kind: "text" | "voice" | "video"; gated?: string }[];
}

export const works: Work[] = [
  {
    slug: "pride-and-prejudice",
    title: "Pride and Prejudice",
    author: { id: "austen", name: "Jane Austen", years: "1775–1817" },
    firstPublished: 1813,
    description:
      "A novel of manners in which the spirited Elizabeth Bennet and the proud Mr. Darcy must each overcome the failing named in the title before they can overcome each other. Austen's irony is a scalpel disguised as embroidery.",
    isPublicDomain: true,
    subjects: ["Classic Literature", "Novel of Manners", "Romance"],
    rating: 4.5,
    ratingCount: 128,
    gutenbergId: 1342,
    coverHue: "vermilion",
  },
  {
    slug: "the-divine-comedy",
    title: "The Divine Comedy",
    author: { id: "dante", name: "Dante Alighieri", years: "1265–1321" },
    firstPublished: 1320,
    description:
      "A pilgrim walks out of a dark wood and down through the architecture of damnation, up the terraces of purgation, and into a rose of light. The poem invented a language for the journey while making it.",
    isPublicDomain: true,
    subjects: ["Epic Poetry", "Medieval Literature", "Theology"],
    rating: 5,
    ratingCount: 96,
    gutenbergId: 8800,
    coverHue: "botanical",
  },
  {
    slug: "crime-and-punishment",
    title: "Crime and Punishment",
    author: { id: "dostoevsky", name: "Fyodor Dostoevsky", years: "1821–1881" },
    firstPublished: 1866,
    description:
      "An impoverished student convinces himself that a single act of arithmetic murder can be justified by utility, then spends five hundred pages discovering what his theory omitted: himself.",
    isPublicDomain: true,
    subjects: ["Russian Literature", "Psychological Fiction", "Philosophy"],
    rating: 4.5,
    ratingCount: 143,
    gutenbergId: 2554,
    coverHue: "ink",
  },
  {
    slug: "moby-dick",
    title: "Moby-Dick; or, The Whale",
    author: { id: "melville", name: "Herman Melville", years: "1819–1891" },
    firstPublished: 1851,
    description:
      "A digressive encyclopedia of whaling that is also a tragedy of monomania, a comedy of shipboard friendship, and the strangest prose experiment of its century. Call it what you will; call him Ishmael.",
    isPublicDomain: true,
    subjects: ["American Literature", "Sea Story", "Epic"],
    rating: 4,
    ratingCount: 87,
    gutenbergId: 2701,
    coverHue: "botanical",
  },
  {
    slug: "the-picture-of-dorian-gray",
    title: "The Picture of Dorian Gray",
    author: { id: "wilde", name: "Oscar Wilde", years: "1854–1900" },
    firstPublished: 1890,
    description:
      "A beautiful young man wishes his portrait would age in his place, and the wish is granted with compound interest. Wilde's only novel is an epigram stretched to breaking point — and it never breaks.",
    isPublicDomain: true,
    subjects: ["Gothic Fiction", "Aestheticism", "Classic Literature"],
    rating: 4.5,
    ratingCount: 112,
    gutenbergId: 174,
    coverHue: "gold",
  },
  {
    slug: "frankenstein",
    title: "Frankenstein; or, The Modern Prometheus",
    author: { id: "shelley", name: "Mary Shelley", years: "1797–1851" },
    firstPublished: 1818,
    description:
      "Written by a teenager during a wet summer on Lake Geneva, the first great myth of the industrial age: a creator who cannot love what he has made, and a creature who learns eloquence before he learns despair.",
    isPublicDomain: true,
    subjects: ["Gothic Fiction", "Science Fiction", "Romanticism"],
    rating: 4,
    ratingCount: 104,
    gutenbergId: 84,
    coverHue: "ink",
  },
  {
    slug: "middlemarch",
    title: "Middlemarch",
    author: { id: "eliot", name: "George Eliot", years: "1819–1880" },
    firstPublished: 1871,
    description:
      "A study of provincial life in which every marriage is a small government and every ideal is tested against the friction of other people. Widely held to be the wisest novel in the English language.",
    isPublicDomain: true,
    subjects: ["Victorian Literature", "Realism", "Classic Literature"],
    rating: 5,
    ratingCount: 74,
    gutenbergId: 145,
    coverHue: "vermilion",
  },
  {
    slug: "the-odyssey",
    title: "The Odyssey",
    author: { id: "homer", name: "Homer", years: "c. 8th cent. BCE" },
    firstPublished: -700,
    description:
      "Ten years of war, then ten years of sea, sorcery, and stubbornness between a man and his home. The West's founding poem about the cost of coming back.",
    isPublicDomain: true,
    subjects: ["Epic Poetry", "Greek Literature", "Mythology"],
    rating: 4.5,
    ratingCount: 91,
    gutenbergId: 1727,
    coverHue: "gold",
  },
];

export const reviews: Review[] = [
  {
    id: "r1",
    workSlug: "crime-and-punishment",
    username: "marginalia_ka",
    displayName: "Katherine A.",
    rating: 4.5,
    title: "The theory survives the man; the man does not survive the theory",
    body: "The Garnett translation flattens some of the humor — and there is humor, gallows-dark, in Porfiry's cat-and-mouse interviews — but the architecture of guilt underneath survives intact. Raskolnikov's fever dreams read like woodcut engravings: stark, overexposed, unforgettable. What struck me on this reread is how much of the novel happens on staircases: thresholds between the theory upstairs and the consequences below. Half a star docked only for the epilogue's hurried convalescence.",
    hasSpoilers: false,
    likeCount: 41,
    createdAt: "2026-08-02",
    isSeed: true,
  },
  {
    id: "r2",
    workSlug: "pride-and-prejudice",
    username: "quilldriver",
    displayName: "Tomás R.",
    rating: 5,
    title: "Free indirect style as a moral instrument",
    body: "People call this a romance, and it is, but the machinery underneath is a course in epistemology. Austen invented a narrative voice that lets us inhabit Elizabeth's confidence so completely that her misjudgment of Wickham becomes our misjudgment — and the correction lands on the reader with the same force it lands on her. The famous letter in Chapter 35 is the hinge of the whole design: the first time in the novel we read a text at the same speed as the heroine. Flawless.",
    hasSpoilers: true,
    likeCount: 67,
    createdAt: "2026-07-28",
    isSeed: true,
  },
  {
    id: "r3",
    workSlug: "moby-dick",
    username: "green_binding",
    displayName: "June O.",
    rating: 4,
    title: "Read the cetology chapters. I mean it.",
    body: "Everyone tells you to skim the whale-classification chapters. Everyone is wrong. They are the novel's ballast: Melville keeps measuring the whale with every instrument the nineteenth century owns — taxonomy, scripture, painting, butchery — and the measurements keep failing, which is precisely the point Ahab refuses to accept. The book teaches you how to read it while you read it. Dock a star for the pacing of the middle third if you must; I nearly didn't.",
    hasSpoilers: false,
    likeCount: 38,
    createdAt: "2026-08-09",
    isSeed: true,
  },
];

export const scholarNotes: ScholarNote[] = [
  {
    id: "n1",
    workSlug: "the-divine-comedy",
    scholar: "Demonstration Note",
    field: "Editorial seed — awaiting a verified scholar",
    kind: "historical",
    chapterRef: "Inferno, Canto I",
    title: "Why Virgil? The politics of choosing a pagan guide",
    body: "Dante's choice of Virgil as guide is a compressed political and literary argument. Virgil was the poet of empire — the Aeneid legitimized Augustus — and Dante, exiled from Florence by the papal-aligned Black Guelphs in 1302, wrote the Comedy partly as a case for a restored universal empire that could check papal temporal power. A pagan guide through a Christian cosmos also stakes a claim: classical reason can carry the soul a great distance — to the summit of Purgatory — but no further. The handoff to Beatrice is the argument's conclusion in narrative form.",
    citations: [
      "This is seeded demonstration content. On the live platform, notes publish only after two verified scholars approve and at least one citation is attached.",
    ],
    isSeed: true,
  },
  {
    id: "n2",
    workSlug: "frankenstein",
    scholar: "Demonstration Note",
    field: "Editorial seed — awaiting a verified scholar",
    kind: "context",
    chapterRef: "Volume I, Letters",
    title: "The wet summer that produced a myth",
    body: "The 1815 eruption of Mount Tambora threw enough ash into the atmosphere that 1816 became 'the year without a summer.' Confined indoors at the Villa Diodati by incessant rain, Byron proposed that his guests each write a ghost story. Mary Shelley — eighteen, travelling with Percy Shelley — produced the seed of Frankenstein. The novel's frame of polar expedition letters and its anxieties about galvanism both come straight from the period's scientific press.",
    citations: [
      "This is seeded demonstration content. On the live platform, notes publish only after two verified scholars approve and at least one citation is attached.",
    ],
    isSeed: true,
  },
];

export const clubs: Club[] = [
  {
    slug: "russian-literature",
    name: "Russian Literature Circle",
    description:
      "Slow readings of the Russian canon, one doorstopper per season. Currently descending the staircases of Crime and Punishment.",
    members: 84,
    currentRead: "crime-and-punishment",
    channels: [
      { name: "general", kind: "text" },
      { name: "crime-and-punishment", kind: "text" },
      { name: "part-five-and-beyond", kind: "text", gated: "Requires 66% progress" },
      { name: "Reading Room", kind: "voice" },
      { name: "Weekly Discussion", kind: "video" },
    ],
  },
  {
    slug: "the-scriptorium",
    name: "The Scriptorium",
    description:
      "Medieval and early-modern texts read in good translations, with side-by-side passages for the brave. Dante in autumn, always.",
    members: 57,
    currentRead: "the-divine-comedy",
    channels: [
      { name: "general", kind: "text" },
      { name: "inferno", kind: "text" },
      { name: "purgatorio", kind: "text", gated: "Requires 33% progress" },
      { name: "Lectio", kind: "voice" },
    ],
  },
  {
    slug: "sea-stories",
    name: "Sea Stories",
    description:
      "Maritime literature and the people obsessed with it. We are currently measuring the whale with every instrument we own.",
    members: 41,
    currentRead: "moby-dick",
    channels: [
      { name: "general", kind: "text" },
      { name: "cetology-defenders", kind: "text" },
      { name: "Quarterdeck", kind: "voice" },
    ],
  },
];

// ————— Lookup helpers —————

export function getWork(slug: string): Work | undefined {
  return works.find((w) => w.slug === slug);
}

export function getReviewsFor(slug: string): Review[] {
  return reviews.filter((r) => r.workSlug === slug);
}

export function getNotesFor(slug: string): ScholarNote[] {
  return scholarNotes.filter((n) => n.workSlug === slug);
}

export function getClub(slug: string): Club | undefined {
  return clubs.find((c) => c.slug === slug);
}

/** The signed-in demo reader's library shelf state (fixture). */
export const demoLibrary = {
  reading: [
    { slug: "crime-and-punishment", progress: 0.62, format: "Public-domain edition" },
    { slug: "middlemarch", progress: 0.31, format: "Physical · Penguin Clothbound" },
  ],
  wantToRead: ["the-odyssey", "the-picture-of-dorian-gray"],
  read: ["pride-and-prejudice", "frankenstein", "moby-dick"],
};
