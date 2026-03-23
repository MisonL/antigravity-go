interface SkeletonRowsProps {
  className?: string;
  lines?: number;
}

interface SkeletonCardListProps {
  cards?: number;
  className?: string;
  lines?: number;
}

export function SkeletonRows({ className = '', lines = 3 }: SkeletonRowsProps) {
  return (
    <div className={`skeleton-stack ${className}`.trim()} aria-hidden="true">
      {Array.from({ length: lines }, (_, index) => (
        <span
          key={`skeleton-line-${index}`}
          className={`skeleton-line${index === 0 ? ' skeleton-line-strong' : ''}${index === lines - 1 ? ' skeleton-line-short' : ''}`}
        />
      ))}
    </div>
  );
}

export function SkeletonCardList({ cards = 3, className = '', lines = 3 }: SkeletonCardListProps) {
  return (
    <div className={`skeleton-card-list ${className}`.trim()} aria-hidden="true">
      {Array.from({ length: cards }, (_, index) => (
        <div key={`skeleton-card-${index}`} className="skeleton-card">
          <SkeletonRows lines={lines} />
        </div>
      ))}
    </div>
  );
}
