import React from 'react';

const TIME_ZONE = 'Asia/Shanghai';

export const TwoLineValue: React.FC<{ id?: string; name?: string }> = ({ id, name }) => {
  if (!id) return <>-</>;
  return (
    <span style={{ display: 'inline-block', maxWidth: '100%' }}>
      <div style={{ wordBreak: 'break-all' }}>{id}</div>
      <div style={{ color: '#8c8c8c', fontSize: 12, marginTop: 2, wordBreak: 'break-all' }}>{name || '-'}</div>
    </span>
  );
};

export const DateTimeValue: React.FC<{ value?: string | Date }> = ({ value }) => {
  if (!value) return <>-</>;
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return <>-</>;
  const dateText = new Intl.DateTimeFormat('en-CA', { timeZone: TIME_ZONE, year: 'numeric', month: '2-digit', day: '2-digit' }).format(date);
  const timeText = new Intl.DateTimeFormat('en-GB', { timeZone: TIME_ZONE, hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(date);
  return (
    <span style={{ display: 'inline-block', lineHeight: 1.45 }}>
      <div>{dateText}</div>
      <div style={{ color: '#8c8c8c', fontSize: 12 }}>{timeText}</div>
    </span>
  );
};
