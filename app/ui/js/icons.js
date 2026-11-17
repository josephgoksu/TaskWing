// Inline stroke icons and the Grid + lead logo. No icon font, no network.

const PATHS = {
  plus: '<path d="M12 5v14M5 12h14"/>',
  up: '<path d="M12 19V5M5 12l7-7 7 7"/>',
  check: '<path d="M20 6 9 17l-5-5"/>',
  x: '<path d="M18 6 6 18M6 6l12 12"/>',
  lock: '<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
  arrow: '<path d="M2 12h18M15 7l5 5-5 5"/>',
  bell: '<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>',
  shield: '<path d="M12 3 5 6v5.5c0 4.3 3 7.9 7 9.5 4-1.6 7-5.2 7-9.5V6l-7-3Z"/><path d="m9.2 12.2 2 2 3.8-4"/>',
  pr: '<circle cx="6" cy="6" r="2.4"/><circle cx="6" cy="18" r="2.4"/><circle cx="18" cy="18" r="2.4"/><path d="M6 8.4v7.2M18 15.6V10a3 3 0 0 0-3-3h-4"/><path d="m13 5-2 2 2 2"/>',
};

export function icon(name, style = "") {
  const body = PATHS[name] || "";
  const viewBox = name === "arrow" ? "0 0 22 24" : "0 0 24 24";
  return `<svg viewBox="${viewBox}" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"${style ? ` style="${style}"` : ""}>${body}</svg>`;
}

// The Grid + lead mark: a 3x3 grid of agents with the lead in the centre.
export function logo(id) {
  const ink = `tw-ink-${id}`, bloom = `tw-bloom-${id}`, tile = `tw-tile-${id}`;
  const box = (x, y) => `<rect x="${x}" y="${y}" width="4" height="4" rx="1.2" fill="none" stroke="#9aa3b2" stroke-width=".8"/>`;
  return `<svg viewBox="0 0 32 32" aria-hidden="true">
    <defs>
      <radialGradient id="${bloom}" cx="1" cy="1" r="1.25"><stop offset="0" stop-color="#7d9cf3"/><stop offset=".35" stop-color="#9ab7f9"/><stop offset=".62" stop-color="#bed2fa"/><stop offset=".85" stop-color="#dfe9fb"/><stop offset="1" stop-color="#fff"/></radialGradient>
      <linearGradient id="${ink}" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#9ab7f9"/><stop offset="1" stop-color="#4f74ec"/></linearGradient>
      <clipPath id="${tile}"><rect width="32" height="32" rx="9"/></clipPath>
    </defs>
    <rect width="32" height="32" rx="9" fill="#fff"/>
    <g clip-path="url(#${tile})"><circle cx="34" cy="34" r="20" fill="url(#${bloom})" opacity=".9"/></g>
    <rect x=".5" y=".5" width="31" height="31" rx="8.5" fill="none" stroke="#12151c" stroke-opacity=".08"/>
    ${box(7.4, 7.4)}${box(14.6, 7.4)}<rect x="21.4" y="7" width="4.8" height="4.8" rx="1.4" fill="#9ab7f9"/>
    ${box(7.4, 14.6)}<rect x="13.8" y="13.8" width="5.6" height="5.6" rx="1.6" fill="url(#${ink})"/>${box(21.8, 14.6)}
    <rect x="7" y="21.4" width="4.8" height="4.8" rx="1.4" fill="#9ab7f9"/>${box(14.6, 21.8)}${box(21.8, 21.8)}
  </svg>`;
}
