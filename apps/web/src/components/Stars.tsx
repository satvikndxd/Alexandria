/** Engraved star rating — half-star precision, drawn not fonted. */

function StarShape({ fillId }: { fillId?: string }) {
  return (
    <path
      d="M12 2.5 L14.8 8.9 L21.7 9.6 L16.5 14.2 L18 21 L12 17.4 L6 21 L7.5 14.2 L2.3 9.6 L9.2 8.9 Z"
      fill={fillId ? `url(#${fillId})` : "currentColor"}
      stroke="currentColor"
      strokeWidth="1.1"
      strokeLinejoin="round"
    />
  );
}

export function Stars({
  rating,
  size = 16,
  className = "",
}: {
  rating: number; // 0..5, half-star steps
  size?: number;
  className?: string;
}) {
  const stars = [];
  for (let i = 1; i <= 5; i++) {
    const filled = rating >= i;
    const half = !filled && rating >= i - 0.5;
    const gid = `half-${i}-${size}`;
    stars.push(
      <svg key={i} width={size} height={size} viewBox="0 0 24 24" aria-hidden className="text-gold">
        {half && (
          <defs>
            <linearGradient id={gid}>
              <stop offset="50%" stopColor="currentColor" />
              <stop offset="50%" stopColor="transparent" />
            </linearGradient>
          </defs>
        )}
        {filled ? (
          <StarShape />
        ) : half ? (
          <StarShape fillId={gid} />
        ) : (
          <path
            d="M12 2.5 L14.8 8.9 L21.7 9.6 L16.5 14.2 L18 21 L12 17.4 L6 21 L7.5 14.2 L2.3 9.6 L9.2 8.9 Z"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.1"
            strokeLinejoin="round"
            opacity="0.45"
          />
        )}
      </svg>
    );
  }
  return (
    <span
      className={`inline-flex items-center gap-0.5 ${className}`}
      role="img"
      aria-label={`Rated ${rating} of 5 stars`}
    >
      {stars}
    </span>
  );
}
