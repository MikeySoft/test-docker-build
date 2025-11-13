import React, { useEffect, useMemo, useRef, useState } from 'react';

type Props = {
  value: string;
  onChange: (next: string) => void;
  onSubmit?: (value: string) => void;
  placeholder?: string;
  statuses?: string[];
  hosts?: string[];
  images?: string[];
};

const FIELD_SUGGESTIONS = ['name:', 'status=', 'status!=', 'image:', 'host='];

const QueryFilterInput: React.FC<Props> = ({ value, onChange, onSubmit, placeholder = 'Filter (e.g. name:nginx status=running)', statuses = ['running', 'stopped', 'paused', 'exited', 'online', 'offline'], hosts = [], images = [] }) => {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  const suggestions = useMemo(() => {
    const v = value.trim();
    if (v === '') return FIELD_SUGGESTIONS;
    const last = v.split(/\s+/).pop() || '';
    const lower = last.toLowerCase();
    // If last token looks like field prefix
    const out: string[] = [];
    for (const f of FIELD_SUGGESTIONS) {
      if (f.toLowerCase().startsWith(lower)) out.push(f);
    }
    // Values suggestions by field
    if (last.startsWith('status=')) {
      const prefix = 'status=';
      for (const s of statuses) {
        const sug = prefix + s;
        if (sug.toLowerCase().startsWith(lower)) out.push(sug);
      }
    }
    if (last.startsWith('host=')) {
      const prefix = 'host=';
      for (const h of hosts) {
        const sug = prefix + h;
        if (sug.toLowerCase().startsWith(lower)) out.push(sug);
      }
    }
    if (last.startsWith('image:')) {
      const prefix = 'image:';
      for (const img of images.slice(0, 10)) {
        const sug = prefix + img;
        if (sug.toLowerCase().startsWith(lower)) out.push(sug);
      }
    }
    return out.length ? out : FIELD_SUGGESTIONS.filter(f => f.toLowerCase().includes(lower));
  }, [value, statuses, hosts, images]);

  useEffect(() => {
    setActive(0);
  }, [suggestions]);

  const applySuggestion = (s: string) => {
    const parts = value.trim().split(/\s+/);
    parts.pop();
    const next = (parts.concat([s]).join(' ') + ' ').replace(/\s+$/, ' ');
    onChange(next);
    setOpen(false);
    inputRef.current?.focus();
  };

  return (
    <div className="relative">
      <input
        ref={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 120)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            setOpen(false);
            if (typeof onSubmit === 'function') onSubmit(value);
            return;
          }
          if (!open) return;
          if (e.key === 'ArrowDown') {
            e.preventDefault();
            setActive((a) => Math.min(a + 1, Math.max(0, suggestions.length - 1)));
          } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            setActive((a) => Math.max(0, a - 1));
          } else if (e.key === 'Tab') {
            if (suggestions[active]) {
              e.preventDefault();
              applySuggestion(suggestions[active]);
            }
          }
        }}
        placeholder={placeholder}
        className="w-full px-3 py-2 border border-gray-300 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white"
      />
      {open && suggestions.length > 0 && (
        <div className="absolute z-10 mt-1 w-full bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded shadow">
          <ul className="max-h-48 overflow-auto">
            {suggestions.map((s, i) => (
              <li
                key={s + i}
                className={`px-3 py-2 cursor-pointer text-sm ${i === active ? 'bg-gray-100 dark:bg-gray-700' : ''}`}
                onMouseDown={(e) => {
                  e.preventDefault();
                  applySuggestion(s);
                }}
                onMouseEnter={() => setActive(i)}
              >
                {s}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
};

export default QueryFilterInput;


