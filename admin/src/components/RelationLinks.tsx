import React from 'react';
import { Link } from 'react-router-dom';
import type { ReactNode } from 'react';

export type RelationMetric = {
  key: string;
  to: string;
  icon: ReactNode;
  value: number | string;
  title: string;
};

/** One-to-many jump cell: stacked icon + count rows (≈1/3 each). */
export const RelationStack: React.FC<{ items: RelationMetric[] }> = ({ items }) => (
  <div style={{ display: 'flex', flexDirection: 'column', gap: 4, minWidth: 72, lineHeight: 1.2 }}>
    {items.map((item) => (
      <Link
        key={item.key}
        to={item.to}
        title={item.title}
        style={{ display: 'flex', alignItems: 'center', gap: 8, color: '#1677ff' }}
      >
        <span style={{ width: 16, display: 'inline-flex', justifyContent: 'center', color: '#595959' }}>{item.icon}</span>
        <span style={{ fontVariantNumeric: 'tabular-nums' }}>{item.value}</span>
      </Link>
    ))}
  </div>
);

/** One-to-one jump: prefer a human list label as link text. */
export const EntityLink: React.FC<{ to: string; label?: string; fallback?: string }> = ({ to, label, fallback }) => {
  const text = (label || '').trim() || fallback || '-';
  return <Link to={to}>{text}</Link>;
};
