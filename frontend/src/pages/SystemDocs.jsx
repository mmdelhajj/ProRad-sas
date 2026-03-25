import { useState, useEffect, useRef, useMemo } from 'react'

/* ── SVG Icon Components ──────────────────────────────────────────── */
const I = {
  rocket:     (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.59 14.37a6 6 0 01-5.84 7.38v-4.8m5.84-2.58a14.98 14.98 0 006.16-12.12A14.98 14.98 0 009.63 8.41m5.96 5.96a14.926 14.926 0 01-5.841 2.58m-.119-8.54a6 6 0 00-7.381 5.84h4.8m2.58-5.84a14.927 14.927 0 00-2.58 5.84m2.699 2.7c-.103.021-.207.041-.311.06a15.09 15.09 0 01-2.448-2.448 14.9 14.9 0 01.06-.312m-2.24 2.39a4.493 4.493 0 00-1.757 4.306 4.493 4.493 0 004.306-1.758M16.5 9a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z"/></svg>,
  chart:      (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z"/></svg>,
  users:      (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z"/></svg>,
  clipboard:  (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 002.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 00-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 00.75-.75 2.25 2.25 0 00-.1-.664m-5.8 0A2.251 2.251 0 0113.5 2.25H15a2.25 2.25 0 012.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25z"/></svg>,
  cube:       (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M21 7.5l-9-5.25L3 7.5m18 0l-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9"/></svg>,
  signal:     (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M3 4.5h14.25M3 9h9.75M3 13.5h9.75m4.5-4.5v12m0 0l-3.75-3.75M17.25 21L21 17.25"/></svg>,
  globe:      (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418"/></svg>,
  server:     (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z"/></svg>,
  wifi:       (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M8.288 15.038a5.25 5.25 0 017.424 0M5.106 11.856c3.807-3.808 9.98-3.808 13.788 0M1.924 8.674c5.565-5.565 14.587-5.565 20.152 0M12.53 18.22l-.53.53-.53-.53a.75.75 0 011.06 0z"/></svg>,
  clock:      (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>,
  wallet:     (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z"/></svg>,
  banknotes:  (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z"/></svg>,
  chat:       (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155"/></svg>,
  handshake:  (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z"/></svg>,
  lock:       (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"/></svg>,
  save:       (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125"/></svg>,
  search:     (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z"/></svg>,
  ticket:     (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M16.5 6v.75m0 3v.75m0 3v.75m0 3V18m-9-5.25h5.25M7.5 15h3M3.375 5.25c-.621 0-1.125.504-1.125 1.125v3.026a2.999 2.999 0 010 5.198v3.026c0 .621.504 1.125 1.125 1.125h17.25c.621 0 1.125-.504 1.125-1.125v-3.026a2.999 2.999 0 010-5.198V6.375c0-.621-.504-1.125-1.125-1.125H3.375z"/></svg>,
  pencil:     (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"/></svg>,
  wrench:     (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M11.42 15.17l-5.1 5.1a3.04 3.04 0 01-4.24-4.24l5.1-5.1m4.24 4.24l5.1-5.1a3.04 3.04 0 00-4.24-4.24l-5.1 5.1m4.24 4.24L7.66 8.93"/></svg>,
  cog:        (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z"/><path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>,
  globeAlt:   (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5a17.92 17.92 0 01-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418"/></svg>,
  shield:     (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z"/></svg>,
  user:       (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z"/></svg>,
  phone:      (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M10.5 1.5H8.25A2.25 2.25 0 006 3.75v16.5a2.25 2.25 0 002.25 2.25h7.5A2.25 2.25 0 0018 20.25V3.75a2.25 2.25 0 00-2.25-2.25H13.5m-3 0V3h3V1.5m-3 0h3m-3 18.75h3"/></svg>,
  refresh:    (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182M2.985 19.644l3.182-3.182"/></svg>,
  building:   (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 3H21m-3.75 3H21"/></svg>,
  link:       (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m13.35-.622l1.757-1.757a4.5 4.5 0 00-6.364-6.364l-4.5 4.5a4.5 4.5 0 001.242 7.244"/></svg>,
  lifering:   (c) => <svg className={c} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M11.42 15.17l-5.1 5.1a3.04 3.04 0 01-4.24-4.24l5.1-5.1m4.24 4.24l5.1-5.1a3.04 3.04 0 00-4.24-4.24l-5.1 5.1m4.24 4.24L7.66 8.93"/></svg>,
}

const SECTION_ICON_MAP = {
  'getting-started': 'rocket', 'dashboard': 'chart', 'subscribers': 'users', 'subscriber-details': 'clipboard',
  'services': 'cube', 'fup': 'signal', 'cdn': 'globe', 'nas': 'server', 'sessions': 'wifi',
  'bandwidth-rules': 'clock', 'wallet': 'wallet', 'billing': 'banknotes', 'communication': 'chat',
  'resellers': 'handshake', 'permissions': 'lock', 'backups': 'save', 'sharing': 'search',
  'tickets': 'ticket', 'audit': 'pencil', 'diagnostics': 'wrench', 'settings': 'cog',
  'remote-access': 'globeAlt', 'ssl': 'shield', 'customer-portal': 'user', 'mobile-app': 'phone',
  'updates': 'refresh', 'ha-cluster': 'building', 'api': 'link', 'troubleshooting': 'lifering',
}

function SectionIcon({ id, className }) {
  const name = SECTION_ICON_MAP[id]
  const render = I[name]
  return render ? render(className || 'w-4 h-4') : null
}

/* ── Data ────────────────────────────────────────────────────────── */
const SECTION_GROUPS = [
  { label: 'GETTING STARTED', ids: ['getting-started', 'dashboard', 'subscribers', 'subscriber-details'] },
  { label: 'SERVICES & PLANS', ids: ['services', 'fup', 'cdn', 'bandwidth-rules'] },
  { label: 'NETWORK', ids: ['nas', 'sessions', 'diagnostics'] },
  { label: 'BILLING', ids: ['wallet', 'billing', 'communication'] },
  { label: 'MANAGEMENT', ids: ['resellers', 'permissions', 'sharing', 'tickets', 'audit'] },
  { label: 'SYSTEM', ids: ['settings', 'remote-access', 'ssl', 'updates', 'ha-cluster', 'api'] },
  { label: 'PORTALS', ids: ['customer-portal', 'mobile-app'] },
  { label: 'REFERENCE', ids: ['backups', 'troubleshooting'] },
]

const SECTIONS = [
  { id: 'getting-started', label: 'Getting Started' },
  { id: 'dashboard', label: 'Dashboard' },
  { id: 'subscribers', label: 'Subscribers' },
  { id: 'subscriber-details', label: 'Subscriber Details' },
  { id: 'services', label: 'Services & Plans' },
  { id: 'fup', label: 'FUP (Fair Usage Policy)' },
  { id: 'cdn', label: 'CDN Management' },
  { id: 'nas', label: 'NAS / Routers' },
  { id: 'sessions', label: 'Sessions' },
  { id: 'bandwidth-rules', label: 'Speed Rules' },
  { id: 'wallet', label: 'Subscriber Wallet' },
  { id: 'billing', label: 'Billing & Transactions' },
  { id: 'communication', label: 'Communication Rules' },
  { id: 'resellers', label: 'Resellers' },
  { id: 'permissions', label: 'Users & Permissions' },
  { id: 'backups', label: 'Backups & Recovery' },
  { id: 'sharing', label: 'Sharing Detection' },
  { id: 'tickets', label: 'Support Tickets' },
  { id: 'audit', label: 'Audit & System Logs' },
  { id: 'diagnostics', label: 'Diagnostic Tools' },
  { id: 'settings', label: 'Settings' },
  { id: 'remote-access', label: 'Remote Access' },
  { id: 'ssl', label: 'SSL / HTTPS' },
  { id: 'customer-portal', label: 'Customer Portal' },
  { id: 'mobile-app', label: 'Mobile App' },
  { id: 'updates', label: 'System Updates' },
  { id: 'ha-cluster', label: 'High Availability' },
  { id: 'api', label: 'API Integration' },
  { id: 'troubleshooting', label: 'Troubleshooting' },
]

const HERO_CARDS = [
  { id: 'getting-started', icon: 'rocket', title: 'Getting Started', desc: 'Installation, login, navigation, and user roles' },
  { id: 'subscribers', icon: 'users', title: 'Subscribers', desc: 'Create, manage, and monitor PPPoE users' },
  { id: 'services', icon: 'cube', title: 'Services', desc: 'Speed plans, FUP tiers, quotas, and pricing' },
  { id: 'billing', icon: 'banknotes', title: 'Billing', desc: 'Transactions, invoices, prepaid cards, reports' },
  { id: 'settings', icon: 'cog', title: 'Settings', desc: 'Branding, RADIUS, notifications, SSL, cluster' },
  { id: 'troubleshooting', icon: 'lifering', title: 'Troubleshooting', desc: 'Common issues and step-by-step solutions' },
]

function Tip({ title, children }) {
  return (
    <div className="my-5 px-4 py-3.5 bg-blue-50 dark:bg-blue-900/20 border-l-4 border-blue-400 dark:border-blue-500 rounded-r-xl">
      <div className="flex items-start gap-2.5">
        <svg className="w-5 h-5 text-blue-500 dark:text-blue-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20"><path d="M11 3a1 1 0 10-2 0v1a1 1 0 102 0V3zM15.657 5.757a1 1 0 00-1.414-1.414l-.707.707a1 1 0 001.414 1.414l.707-.707zM18 10a1 1 0 01-1 1h-1a1 1 0 110-2h1a1 1 0 011 1zM5.05 6.464A1 1 0 106.464 5.05l-.707-.707a1 1 0 00-1.414 1.414l.707.707zM4 11a1 1 0 100-2H3a1 1 0 000 2h1zM10 18a1 1 0 001-1v-1a1 1 0 10-2 0v1a1 1 0 001 1zM15.657 15.657a1 1 0 00-1.414-1.414l-.707.707a1 1 0 001.414 1.414l.707-.707zM5.05 14.95a1 1 0 011.414-1.414l.707.707a1 1 0 01-1.414 1.414l-.707-.707zM10 6a4 4 0 100 8 4 4 0 000-8z" /></svg>
        <div>
          {title && <p className="text-sm font-semibold text-blue-900 dark:text-blue-200 mb-1">{title}</p>}
          <div className="text-sm text-blue-800 dark:text-blue-300 leading-relaxed">{children}</div>
        </div>
      </div>
    </div>
  )
}

function Note({ title, children }) {
  return (
    <div className="my-5 px-4 py-3.5 bg-amber-50 dark:bg-amber-900/20 border-l-4 border-amber-400 dark:border-amber-500 rounded-r-xl">
      <div className="flex items-start gap-2.5">
        <svg className="w-5 h-5 text-amber-500 dark:text-amber-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clipRule="evenodd" /></svg>
        <div>
          {title && <p className="text-sm font-semibold text-amber-900 dark:text-amber-200 mb-1">{title}</p>}
          <div className="text-sm text-amber-800 dark:text-amber-300 leading-relaxed">{children}</div>
        </div>
      </div>
    </div>
  )
}

function Warning({ title, children }) {
  return (
    <div className="my-5 px-4 py-3.5 bg-red-50 dark:bg-red-900/20 border-l-4 border-red-400 dark:border-red-500 rounded-r-xl">
      <div className="flex items-start gap-2.5">
        <svg className="w-5 h-5 text-red-500 dark:text-red-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clipRule="evenodd" /></svg>
        <div>
          {title && <p className="text-sm font-semibold text-red-900 dark:text-red-200 mb-1">{title}</p>}
          <div className="text-sm text-red-800 dark:text-red-300 leading-relaxed">{children}</div>
        </div>
      </div>
    </div>
  )
}

function SectionHeading({ id, children }) {
  return (
    <div className="mb-5">
      <h2 id={id} className="text-2xl font-bold text-gray-900 dark:text-white scroll-mt-20 flex items-center gap-3">
        <span className="flex-shrink-0 w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-900/30 flex items-center justify-center">
          <SectionIcon id={id} className="w-[18px] h-[18px] text-blue-600 dark:text-blue-400" />
        </span>
        {children}
      </h2>
      <div className="mt-3 h-px bg-gradient-to-r from-blue-500/40 via-blue-500/10 to-transparent" />
    </div>
  )
}

function SubHeading({ children }) {
  return <h3 className="text-lg font-semibold text-gray-800 dark:text-gray-200 mt-7 mb-2.5">{children}</h3>
}

function Sub2({ children }) {
  return <h4 className="text-base font-semibold text-gray-700 dark:text-gray-300 mt-5 mb-2">{children}</h4>
}

function P({ children }) {
  return <p className="text-sm text-gray-600 dark:text-gray-400 mb-3.5 leading-relaxed">{children}</p>
}

function Kbd({ children }) {
  return <code className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 rounded text-[13px] font-mono text-gray-800 dark:text-gray-200">{children}</code>
}

function FeatureList({ items }) {
  return (
    <ul className="list-disc list-inside space-y-2 mb-5 ml-1">
      {items.map((item, i) => (
        <li key={i} className="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">{item}</li>
      ))}
    </ul>
  )
}

function Steps({ items }) {
  return (
    <ol className="list-decimal list-inside space-y-2.5 mb-5 ml-1">
      {items.map((item, i) => (
        <li key={i} className="text-sm text-gray-600 dark:text-gray-400 leading-relaxed"><span className="font-medium text-gray-700 dark:text-gray-300">{item.title}</span>{item.desc ? ` — ${item.desc}` : ''}</li>
      ))}
    </ol>
  )
}

function DefTable({ headers, rows }) {
  const keys = headers || (rows[0] ? Object.keys(rows[0]) : ['Term', 'Description'])
  return (
    <div className="overflow-x-auto mb-5">
      <table className="w-full text-sm border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden">
        <thead>
          <tr className="bg-gray-50 dark:bg-gray-800">
            {keys.map((k, i) => (
              <th key={i} className="text-left px-4 py-2.5 font-semibold text-gray-700 dark:text-gray-300">{k}</th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
          {rows.map((row, i) => {
            const vals = Object.values(row)
            return (
              <tr key={i} className="hover:bg-gray-50 dark:hover:bg-gray-800/50">
                {vals.map((v, j) => (
                  <td key={j} className={`px-4 py-2.5 ${j === 0 ? 'font-medium text-gray-800 dark:text-gray-200' : 'text-gray-600 dark:text-gray-400'}`}>{v}</td>
                ))}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

function Card({ title, children }) {
  return (
    <div className="mb-5 border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden shadow-sm hover:shadow-md transition-shadow">
      {title && <div className="px-4 py-3 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 border-l-4 border-l-blue-500 text-sm font-semibold text-gray-800 dark:text-gray-200">{title}</div>}
      <div className="px-4 py-4">{children}</div>
    </div>
  )
}

export default function SystemDocs() {
  const [activeSection, setActiveSection] = useState('getting-started')
  const [mobileNav, setMobileNav] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const searchInputRef = useRef(null)
  const mainRef = useRef(null)

  const filteredSections = useMemo(() => {
    if (!searchQuery.trim()) return SECTIONS
    const q = searchQuery.toLowerCase()
    return SECTIONS.filter(s => s.label.toLowerCase().includes(q) || s.id.toLowerCase().includes(q))
  }, [searchQuery])

  const filteredGroups = useMemo(() => {
    if (!searchQuery.trim()) return SECTION_GROUPS
    return SECTION_GROUPS.map(g => ({
      ...g,
      ids: g.ids.filter(id => filteredSections.some(s => s.id === id)),
    })).filter(g => g.ids.length > 0)
  }, [searchQuery, filteredSections])

  useEffect(() => {
    const hash = window.location.hash.slice(1)
    if (hash && SECTIONS.find(s => s.id === hash)) {
      setActiveSection(hash)
      setTimeout(() => document.getElementById(hash)?.scrollIntoView({ behavior: 'smooth' }), 100)
    }
  }, [])

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveSection(entry.target.id)
            break
          }
        }
      },
      { rootMargin: '-100px 0px -70% 0px', threshold: 0 }
    )
    SECTIONS.forEach(s => {
      const el = document.getElementById(s.id)
      if (el) observer.observe(el)
    })
    return () => observer.disconnect()
  }, [])

  const scrollTo = (id) => {
    setActiveSection(id)
    setMobileNav(false)
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' })
    window.history.replaceState(null, '', `#${id}`)
  }

  const handleSearchKeyDown = (e) => {
    if (e.key === 'Enter' && filteredSections.length > 0) {
      scrollTo(filteredSections[0].id)
    }
    if (e.key === 'Escape') {
      setSearchQuery('')
      searchInputRef.current?.blur()
    }
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
      {/* Header */}
      <div className="bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-800 sticky top-0 z-20">
        <div className="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button onClick={() => setMobileNav(!mobileNav)} className="md:hidden p-1.5 -ml-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors">
              <svg className="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" /></svg>
            </button>
            <div className="w-7 h-7 rounded-md bg-blue-600 flex items-center justify-center">
              <span className="text-white font-bold text-xs">P</span>
            </div>
            <span className="text-sm font-semibold text-gray-900 dark:text-white">Docs</span>
            <span className="hidden sm:block text-xs text-gray-400 dark:text-gray-500 border-l border-gray-200 dark:border-gray-700 pl-3 ml-0.5">ProxPanel User Guide</span>
          </div>
          <div className="flex items-center gap-2">
            {/* Search bar */}
            <div className="hidden sm:flex items-center relative">
              <svg className="absolute left-2.5 w-4 h-4 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
              <input
                ref={searchInputRef}
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={handleSearchKeyDown}
                placeholder="Search..."
                className="w-44 lg:w-56 pl-8 pr-3 py-1.5 text-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 bg-gray-50 dark:bg-gray-800 rounded-md border border-gray-200 dark:border-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500/30 focus:border-blue-400 dark:focus:border-blue-500 transition-all"
              />
              {searchQuery && (
                <button onClick={() => setSearchQuery('')} className="absolute right-2 p-0.5 hover:bg-gray-200 dark:hover:bg-gray-700 rounded">
                  <svg className="w-3.5 h-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
                </button>
              )}
            </div>
            <a href="/api-docs" className="hidden md:inline-flex items-center px-2.5 py-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-gray-800 rounded-md transition-colors">API Reference</a>
            <a href="/" className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-md transition-colors">
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" /></svg>
              <span className="hidden sm:inline">Panel</span>
            </a>
          </div>
        </div>
        {/* Mobile search */}
        <div className="sm:hidden px-4 pb-3">
          <div className="relative">
            <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyDown={handleSearchKeyDown}
              placeholder="Search documentation..."
              className="w-full pl-8 pr-3 py-2 text-sm bg-gray-50 dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 rounded-md border border-gray-200 dark:border-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500/30"
            />
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto flex" ref={mainRef}>
        {/* Mobile nav overlay */}
        {mobileNav && <div className="fixed inset-0 z-30 bg-black/40 md:hidden" onClick={() => setMobileNav(false)} />}

        {/* Sidebar */}
        <nav className={`${mobileNav ? 'fixed inset-y-0 left-0 z-40 w-72 transform translate-x-0' : 'hidden'} md:block md:static md:w-60 flex-shrink-0 sticky top-[60px] h-[calc(100vh-60px)] overflow-y-auto border-r border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 py-4 px-3`}>
          <div className="md:hidden flex justify-between items-center mb-3 pb-2 border-b border-gray-200 dark:border-gray-700 px-1">
            <span className="text-sm font-semibold text-gray-700 dark:text-gray-300">Navigation</span>
            <button onClick={() => setMobileNav(false)} className="p-1 hover:bg-gray-100 dark:hover:bg-gray-800 rounded">
              <svg className="w-4 h-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
          {searchQuery && filteredSections.length === 0 && (
            <div className="px-3 py-4 text-sm text-gray-500 dark:text-gray-400 text-center">
              No results for "{searchQuery}"
            </div>
          )}
          <div className="space-y-4">
            {filteredGroups.map(group => (
              <div key={group.label}>
                <div className="px-3 mb-1.5">
                  <span className="text-[10px] font-bold tracking-widest text-gray-400 dark:text-gray-500 uppercase">{group.label}</span>
                </div>
                <ul className="space-y-0.5">
                  {group.ids.map(id => {
                    const s = SECTIONS.find(sec => sec.id === id)
                    if (!s) return null
                    return (
                      <li key={s.id}>
                        <button
                          onClick={() => scrollTo(s.id)}
                          className={`w-full text-left px-3 py-1.5 text-sm rounded-lg transition-colors flex items-center gap-2 ${
                            activeSection === s.id
                              ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 font-semibold border-l-[3px] border-l-blue-500'
                              : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800/60'
                          }`}
                        >
                          <SectionIcon id={s.id} className="w-4 h-4 flex-shrink-0 text-gray-400 dark:text-gray-500" />
                          <span className="truncate">{s.label}</span>
                        </button>
                      </li>
                    )
                  })}
                </ul>
              </div>
            ))}
          </div>
        </nav>

        {/* Main Content */}
        <main className="flex-1 min-w-0 p-5 md:p-8 max-w-4xl">

          {/* Hero Section */}
          {!searchQuery && (
            <div className="mb-12">
              <div className="mb-6">
                <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">Welcome to ProxPanel Documentation</h2>
                <p className="text-sm text-gray-500 dark:text-gray-400">Everything you need to manage your ISP network. Select a topic below or browse the sections.</p>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                {HERO_CARDS.map(card => (
                  <button
                    key={card.id}
                    onClick={() => scrollTo(card.id)}
                    className="text-left p-4 bg-white dark:bg-gray-800/80 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm hover:shadow-md hover:border-blue-300 dark:hover:border-blue-600 transition-all group"
                  >
                    <span className="mb-2 block">{I[card.icon] ? I[card.icon]('w-6 h-6 text-blue-600 dark:text-blue-400') : null}</span>
                    <h3 className="text-sm font-semibold text-gray-900 dark:text-white group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors">{card.title}</h3>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 leading-relaxed">{card.desc}</p>
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* ============================================================
              GETTING STARTED
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="getting-started">Getting Started</SectionHeading>
            <P>
              ProxPanel is a professional ISP management platform built for Internet Service Providers. It provides complete
              subscriber management, RADIUS authentication, MikroTik RouterOS integration, billing, real-time bandwidth monitoring,
              and much more — all from a single web-based control panel.
            </P>

            <SubHeading>System Overview</SubHeading>
            <P>
              ProxPanel runs as a set of Docker containers on your server. The main components are:
            </P>
            <DefTable rows={[
              { Component: 'API Server', Description: 'Go backend handling all business logic, subscriber management, and background services (port 8080)' },
              { Component: 'RADIUS Server', Description: 'Custom RADIUS server for PPPoE authentication and accounting (ports 1812/1813 UDP)' },
              { Component: 'Frontend', Description: 'React web application served by Nginx (port 80/443)' },
              { Component: 'PostgreSQL', Description: 'Primary database storing all subscribers, services, transactions, and configuration' },
              { Component: 'Redis', Description: 'In-memory cache for sessions, rate limiting, and subscriber data caching' },
            ]} />

            <SubHeading>Logging In</SubHeading>
            <P>
              Open your browser and navigate to your server's IP address or domain name. You will see the ProxPanel login page.
              Enter your admin username and password to access the management panel.
            </P>
            <FeatureList items={[
              'Default login: use the credentials provided during installation',
              'Remember Me: check this box to save your login credentials in the browser for faster access next time',
              'Session timeout: your session will expire after the configured inactivity period (default 10 minutes). You\'ll be redirected to the login page with a notice',
              'If you forget your password, contact the system administrator to reset it from the Users page',
            ]} />

            <SubHeading>User Roles</SubHeading>
            <P>ProxPanel supports four distinct user roles, each with different access levels:</P>
            <DefTable rows={[
              { Role: 'Admin', Access: 'Full system access', Description: 'Can manage everything: settings, users, subscribers, NAS devices, backups, and all features. Has access to all pages without restriction.' },
              { Role: 'Reseller', Access: 'Controlled by permissions', Description: 'Manages their own subscribers within limits set by the admin. Can create/renew/modify subscribers, view reports, and manage billing. Access is controlled via Permission Groups.' },
              { Role: 'Collector', Access: 'Collection page only', Description: 'A specialized role for field agents who collect payments. Can only see the "My Collections" page with assigned subscribers and payment recording.' },
              { Role: 'Customer', Access: 'Self-service portal', Description: 'End subscribers who log in with their PPPoE credentials. See the Customer Portal section for details.' },
            ]} />

            <SubHeading>Navigation &amp; Sidebar</SubHeading>
            <P>
              The left sidebar contains all navigation items organized in a tree-view layout. On mobile devices,
              tap the hamburger icon to open the sidebar. On desktop, click the hamburger icon in the blue title bar
              to collapse/expand the sidebar.
            </P>
            <Card title="Customizing Your Sidebar">
              <P>Every user can personalize their sidebar menu:</P>
              <Steps items={[
                { title: 'Enter edit mode', desc: 'click the menu customize icon (three horizontal lines) in the title bar' },
                { title: 'Reorder items', desc: 'use the up/down arrow buttons to move items' },
                { title: 'Hide items', desc: 'click the eye icon to hide menu items you don\'t use' },
                { title: 'Show hidden items', desc: 'click "Show All" to reveal hidden items' },
                { title: 'Reset', desc: 'click "Reset" to restore default order and visibility' },
                { title: 'Save', desc: 'click "Done" when finished — preferences are saved per browser' },
              ]} />
            </Card>

            <SubHeading>Dark Mode</SubHeading>
            <P>
              ProxPanel supports both light and dark themes. Toggle between them using any of these methods:
            </P>
            <FeatureList items={[
              'Click the sun/moon icon in the blue title bar',
              'Click the company name in the sidebar header',
              'Your preference is saved and persists across sessions and page refreshes',
            ]} />

            <SubHeading>Title Bar</SubHeading>
            <P>The blue title bar at the top shows important information and quick actions:</P>
            <FeatureList items={[
              'Username @ Server IP — shows who is logged in and which server',
              'Company name and version number',
              'Reseller balance (for reseller users) — green for positive, red for negative',
              'Clock with session countdown timer',
              'Update notification bell — shows when a new system update is available',
              'Profile icon — link to your profile page',
              'Theme toggle — switch between light and dark mode',
              'Fullscreen toggle — enter/exit fullscreen mode (desktop only)',
              'Logout button (X icon) — safely end your session',
            ]} />
          </section>

          {/* ============================================================
              DASHBOARD
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="dashboard">Dashboard</SectionHeading>
            <P>
              The Dashboard is your home page and provides a real-time overview of your entire ISP network at a glance.
              All statistics refresh automatically every 30 seconds.
            </P>

            <SubHeading>Subscriber Statistics</SubHeading>
            <P>Four stat cards show your subscriber counts with trend indicators:</P>
            <DefTable rows={[
              { Card: 'Total Subscribers', Description: 'Total number of registered subscriber accounts in the system (excluding deleted)' },
              { Card: 'Online Now', Description: 'Number of subscribers currently connected via PPPoE. Green indicator with real-time count.' },
              { Card: 'Expired', Description: 'Subscribers whose expiry date has passed. These accounts cannot connect until renewed.' },
              { Card: 'Expiring Soon', Description: 'Subscribers expiring within the next 7 days. Helps you proactively contact them for renewal.' },
            ]} />

            <SubHeading>Revenue &amp; Session Statistics</SubHeading>
            <DefTable rows={[
              { Card: 'Today\'s Revenue', Description: 'Total income from all transactions created today (renewals, payments, new subscriptions)' },
              { Card: 'Monthly Revenue', Description: 'Total income for the current calendar month with percentage change vs previous month' },
              { Card: 'Active Sessions', Description: 'Number of currently active PPPoE sessions across all NAS devices' },
              { Card: 'Total Resellers', Description: 'Number of registered reseller accounts (admin only)' },
            ]} />

            <SubHeading>System Metrics (Admin Only)</SubHeading>
            <P>
              Three real-time progress bars show your server's resource utilization. These refresh every 10 seconds:
            </P>
            <FeatureList items={[
              'CPU Usage — current processor utilization. Color changes from green (<50%) to orange (<80%) to red (>=80%)',
              'Memory Usage — RAM consumption showing used vs total available. High usage may indicate need for more RAM',
              'Disk Usage — storage space on the root filesystem. Monitor this to ensure you have enough space for backups and logs',
            ]} />
            <Note title="Resellers">System metrics are hidden from reseller accounts. Resellers only see subscriber and revenue statistics relevant to their account.</Note>

            <SubHeading>Charts</SubHeading>
            <P>Two interactive charts provide visual analytics:</P>
            <FeatureList items={[
              'New vs Expired — a 30-day line chart showing daily new subscriber registrations and expired accounts. Helps identify growth trends.',
              'Subscribers by Service — a donut/pie chart showing the distribution of subscribers across your service plans. Hover for exact counts.',
            ]} />

            <SubHeading>Server Capacity (Admin Only)</SubHeading>
            <P>
              Shows your server's estimated capacity based on hardware specifications (CPU cores, storage type) with a usage bar
              and current subscriber count. If HA Cluster is configured, a table of all cluster nodes with their status,
              CPU/memory usage, and replication lag is displayed.
            </P>

            <SubHeading>Recent Transactions</SubHeading>
            <P>
              A table of the latest 5 financial transactions with date, type, subscriber username, amount, and description.
              Click "Transactions" in the sidebar to see the full transaction history.
            </P>

            <SubHeading>Help &amp; Documentation</SubHeading>
            <P>
              Quick-access cards at the bottom of the dashboard link to this documentation page, the API reference,
              and the setup guide.
            </P>
          </section>

          {/* ============================================================
              SUBSCRIBERS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="subscribers">Subscribers</SectionHeading>
            <P>
              The Subscribers page is the heart of ProxPanel. It manages all PPPoE subscriber accounts — the end users
              who connect to the internet through your network. This is where you create, modify, monitor, and manage
              every subscriber.
            </P>

            <SubHeading>Subscriber List</SubHeading>
            <P>
              The main view is a powerful data table with search, filtering, sorting, column customization, and bulk actions.
              The table supports thousands of subscribers with server-side pagination.
            </P>

            <Sub2>Available Columns</Sub2>
            <P>Click the <Kbd>Columns</Kbd> button to show or hide any of these columns:</P>
            <DefTable rows={[
              { Column: 'Username', Description: 'PPPoE login name (e.g., user@domain). Click to open subscriber details. Green dot = online.' },
              { Column: 'Full Name', Description: 'Subscriber\'s real name for identification' },
              { Column: 'Phone', Description: 'Contact phone number' },
              { Column: 'MAC Address', Description: 'Client device MAC address (bound after first connection)' },
              { Column: 'IP Address', Description: 'Currently assigned IP address (only shown when online)' },
              { Column: 'Service', Description: 'Assigned service plan name with daily usage progress bar' },
              { Column: 'Reseller', Description: 'Which reseller owns this subscriber (shows parent chain for sub-resellers)' },
              { Column: 'Status', Description: 'Account status badge: Active (green), Inactive (red), Expired (orange), Stopped (gray)' },
              { Column: 'Expiry Date', Description: 'When the account expires. Red text when expired or expiring within 3 days.' },
              { Column: 'Last Seen', Description: '"Online Now" (green) if connected, or relative time since last session (5m ago, 3h ago, 2d ago)' },
              { Column: 'Daily Quota', Description: 'Daily download usage with sortable header for server-side sorting by usage' },
              { Column: 'Monthly Quota', Description: 'Monthly download usage progress' },
              { Column: 'CDN Usage', Description: 'CDN daily download usage with FUP badge (when CDN FUP is enabled on the service)' },
              { Column: 'Balance', Description: 'Subscriber credit balance for prepaid billing' },
              { Column: 'Price', Description: 'Service price or custom override price (orange star icon when override is active)' },
              { Column: 'Address / Region / Notes', Description: 'Additional subscriber information fields' },
              { Column: 'Created At', Description: 'Date and time when the subscriber account was created' },
            ]} />

            <SubHeading>Searching &amp; Filtering</SubHeading>
            <FeatureList items={[
              'Search bar — type to search across username, full name, phone number, and IP address simultaneously',
              'Status filter — dropdown to show only Active, Inactive, Expired, or Stopped subscribers',
              'Service filter — show only subscribers on a specific service plan',
              'NAS filter — show only subscribers connected to a specific router',
              'Reseller filter — show only subscribers belonging to a specific reseller',
              'Online filter — show only currently online or offline subscribers',
              'Clickable status counters — click Online, Offline, Active, Inactive, or Expired in the stats bar to quickly filter. Click again to remove the filter. Active filter shows a highlighted ring.',
              'FUP level counters — click FUP 0, FUP 1, FUP 2, or FUP 3 to filter by current FUP tier',
            ]} />

            <SubHeading>Status Types Explained</SubHeading>
            <DefTable rows={[
              { Status: 'Active', Badge: 'Green', Description: 'Account is operational. The subscriber can connect via PPPoE and use the internet normally.' },
              { Status: 'Inactive', Badge: 'Red', Description: 'Account has been manually suspended/deactivated by admin or reseller. PPPoE authentication will be rejected. Used for non-payment or policy violations.' },
              { Status: 'Expired', Badge: 'Orange', Description: 'Account has passed its expiry date. The system automatically marks accounts as expired. Authentication is rejected until the account is renewed.' },
              { Status: 'Stopped', Badge: 'Gray', Description: 'Account manually stopped. Similar to Inactive but used for temporary pauses (e.g., subscriber traveling).' },
            ]} />

            <SubHeading>Bulk Actions</SubHeading>
            <P>
              Select one or more subscribers using checkboxes (selected rows highlight in red for visibility), then
              use the sticky action toolbar at the top to perform operations on all selected users at once:
            </P>
            <DefTable rows={[
              { Action: 'Select All', Description: 'Select all subscribers on the current page. Use with filters to target specific groups.' },
              { Action: 'Renew', Description: 'Extend expiry date by the service period (typically 1 month). Also resets monthly FUP counters.' },
              { Action: 'Reset FUP', Description: 'Reset daily quota counters and FUP level back to 0. Restores original service speed. Does NOT reset monthly counters.' },
              { Action: 'Activate', Description: 'Set status to Active for selected subscribers. They can now connect via PPPoE.' },
              { Action: 'Deactivate', Description: 'Set status to Inactive. Disconnects any active session and blocks future authentication.' },
              { Action: 'Disconnect', Description: 'Force-disconnect active PPPoE sessions via RADIUS CoA. User reconnects automatically and gets a fresh IP.' },
              { Action: 'Change Service', Description: 'Move all selected subscribers to a different service plan. Active sessions are disconnected so new speed/pool applies.' },
              { Action: 'Add Days', Description: 'Add a specific number of extra days to each subscriber\'s expiry date.' },
              { Action: 'Refill Quota', Description: 'Reset both daily and monthly usage counters to zero.' },
              { Action: 'Ping', Description: 'Ping each selected subscriber through their MikroTik router. Results shown in a popup.' },
              { Action: 'Delete', Description: 'Soft-delete selected subscribers. A confirmation dialog shows the name of each subscriber being deleted.' },
            ]} />
            <Tip title="Bulk Operations Page">For more complex bulk operations with advanced filters (by reseller, service, status, expiry range, etc.), use the <strong>Change Bulk</strong> page from the sidebar. It supports filtering subscribers before applying actions.</Tip>

            <SubHeading>Creating a New Subscriber</SubHeading>
            <P>Click <Kbd>+ New Subscriber</Kbd> to open the creation form:</P>
            <Steps items={[
              { title: 'Enter a username', desc: 'this is the PPPoE login (e.g., john@yourisp.com). Must be unique across all subscribers.' },
              { title: 'Set a password', desc: 'the PPPoE password. If left blank, a random password is generated automatically.' },
              { title: 'Select a service', desc: 'choose the speed/quota plan for this subscriber. This determines their speed, FUP limits, and IP pool.' },
              { title: 'Fill in personal info', desc: 'name, phone, address, region, etc. (optional but recommended for customer management)' },
              { title: 'Set expiry date', desc: 'defaults to 1 month from today. Adjust as needed.' },
              { title: 'Assign to reseller', desc: '(optional) if this subscriber belongs to a reseller, select them from the dropdown' },
              { title: 'Click Save', desc: 'the subscriber is created and can immediately connect via PPPoE' },
            ]} />

            <SubHeading>CSV Import</SubHeading>
            <P>
              Import subscribers in bulk from a CSV file via <Kbd>Subscribers &rarr; Import</Kbd>. The import wizard:
            </P>
            <Steps items={[
              { title: 'Upload CSV file', desc: 'select your file. Supported columns: username, password, full_name, phone, service, expiry_date, address, region, notes' },
              { title: 'Map columns', desc: 'match your CSV headers to ProxPanel fields' },
              { title: 'Preview', desc: 'review the data before importing. Invalid rows are highlighted in red.' },
              { title: 'Import', desc: 'click Import to create all subscribers. A summary shows how many were created vs skipped.' },
            ]} />

            <SubHeading>Archived (Deleted) Subscribers</SubHeading>
            <P>
              Deleted subscribers are soft-deleted (not permanently removed). View them in the Archived section with
              stats showing deletions by period (today, this week, this month), filter by who deleted them,
              and see the deletion timestamp. Top deleters are shown for admin oversight.
            </P>
          </section>

          {/* ============================================================
              SUBSCRIBER DETAILS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="subscriber-details">Subscriber Details</SectionHeading>
            <P>
              Click any subscriber in the list to open their detail/edit page. This comprehensive page lets you
              manage every aspect of the subscriber's account.
            </P>

            <SubHeading>Personal Information</SubHeading>
            <DefTable rows={[
              { Field: 'Username', Description: 'PPPoE login name. Can be changed (subscriber must reconnect after change).' },
              { Field: 'Password', Description: 'PPPoE password. Shown in clear text for admin reference. Can be changed anytime.' },
              { Field: 'Full Name', Description: 'Subscriber\'s real name for identification and communication.' },
              { Field: 'Phone', Description: 'Contact phone number. Used by WhatsApp/SMS communication rules.' },
              { Field: 'Address', Description: 'Physical address for records and field visits.' },
              { Field: 'Region / Building', Description: 'Additional location details for organizing subscribers by area.' },
              { Field: 'Nationality / Country', Description: 'Dropdown selections with 70+ options each.' },
              { Field: 'Notes', Description: 'Free-text notes about the subscriber (visible to admin/reseller only).' },
              { Field: 'Created At', Description: 'Read-only timestamp showing when the account was first created.' },
            ]} />

            <SubHeading>Account Settings</SubHeading>
            <DefTable rows={[
              { Field: 'Service', Description: 'The service plan this subscriber uses. Changing service disconnects the user to apply new speed/pool.' },
              { Field: 'Status', Description: 'Active, Inactive, Expired, or Stopped. Changing to Inactive disconnects the user immediately.' },
              { Field: 'Expiry Date', Description: 'When the account expires. The system checks this and marks expired accounts automatically.' },
              { Field: 'Balance', Description: 'Credit balance for prepaid billing. Deducted on renewal. Can go negative if credit is allowed.' },
              { Field: 'Reseller', Description: 'Which reseller owns this subscriber. Admin-only field. Affects who can manage the subscriber.' },
              { Field: 'Price Override', Description: 'Check "Override service price" to set a custom price for this specific subscriber, different from the service default.' },
            ]} />

            <SubHeading>Quota &amp; Usage Display</SubHeading>
            <P>
              The edit page shows real-time quota information:
            </P>
            <FeatureList items={[
              'Daily download/upload used — progress bar showing percentage of daily quota consumed',
              'Monthly download/upload used — progress bar for monthly consumption',
              'Current FUP level — badge showing which FUP tier the subscriber is in (0 = normal, 1-3 = reduced speed)',
              'CDN daily usage — separate counter when CDN FUP is enabled on the service',
            ]} />

            <SubHeading>Live Bandwidth Graph (Torch)</SubHeading>
            <P>
              For online subscribers, click the signal icon next to their username to view real-time bandwidth:
            </P>
            <FeatureList items={[
              'Live download/upload speed in Mbps — updates every 2 seconds via MikroTik Torch',
              'CDN traffic — shown as a separate series when CDN subnets are configured',
              'Port rule traffic — custom series for each CDN Port Rule with "Show in Graph" enabled (dashed line, custom color)',
              'Ping latency — color-coded RTT pill in the header: green (<20ms), yellow (<80ms), red (>=80ms)',
              'Dual Y-axis — left axis for bandwidth (Mbps), right axis for ping (ms, amber dotted line)',
              'Statistics cards — Download, Upload, CDN, Latency, and any port rule cards with running averages',
            ]} />

            <SubHeading>Per-Subscriber Speed Rules</SubHeading>
            <P>
              Create custom speed overrides for individual subscribers in the Speed Rules section of the edit page.
              These take priority over service defaults. For example, give a VIP customer 50 Mbps regardless of their
              service plan speed.
            </P>
            <Note>Custom speed rules are applied within 30 seconds by the QuotaSync background service. The subscriber does not need to reconnect.</Note>

            <SubHeading>Usage History Chart</SubHeading>
            <P>
              The Usage tab shows a bar chart of daily download/upload usage for the current month. Each bar represents
              one day. Today's bar updates in real-time from the live usage counter. Past days use stored daily history
              data that is saved before each daily reset.
            </P>

            <SubHeading>Location Map</SubHeading>
            <P>
              In the Status &amp; Options section, an interactive map lets you pin the subscriber's physical location:
            </P>
            <FeatureList items={[
              'Click or drag the pin to set location on the map',
              '"My Location" button uses GPS to set your current position (useful for field installers)',
              'Manual coordinate entry for latitude/longitude',
              '"Navigate" button opens Google Maps directions to the subscriber\'s location',
              'Blue pin icon in the subscriber list opens the location modal for quick access',
            ]} />
          </section>

          {/* ============================================================
              SERVICES & PLANS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="services">Services &amp; Plans</SectionHeading>
            <P>
              Services define the speed, quotas, pricing, and network configuration for subscriber plans.
              Each subscriber is assigned to exactly one service that controls their entire connection experience.
            </P>

            <SubHeading>Creating a Service</SubHeading>
            <P>Click <Kbd>+ New Service</Kbd> and configure the following fields:</P>

            <Sub2>Basic Information</Sub2>
            <DefTable rows={[
              { Field: 'Name', Description: 'Descriptive plan name shown to subscribers (e.g., "8MB-20GB", "Premium Unlimited")' },
              { Field: 'Download Speed (kb)', Description: 'Download speed in kilobits per second. Enter 2000 for 2 Mbps, 8000 for 8 Mbps, 12000 for 12 Mbps.' },
              { Field: 'Upload Speed (kb)', Description: 'Upload speed in kilobits per second. Same format as download.' },
              { Field: 'Price', Description: 'Monthly cost of the service. Used for billing, renewals, and reseller balance deduction.' },
              { Field: 'Daily Quota (bytes)', Description: 'Maximum daily download allowance. Set 0 for unlimited. Common values: 1073741824 = 1 GB, 21474836480 = 20 GB' },
              { Field: 'Monthly Quota (bytes)', Description: 'Maximum monthly download allowance. Set 0 for unlimited.' },
            ]} />
            <Warning title="Speed Format">
              Speeds are stored in <strong>kb (kilobits)</strong> format, NOT Mbps. Enter 2000 for 2 Mbps, not 2.
              The system sends these values directly to MikroTik (e.g., "2000k/1000k" rate limit).
            </Warning>

            <Sub2>RADIUS Settings</Sub2>
            <DefTable rows={[
              { Field: 'NAS / Router', Description: 'Select which router this service uses. Loads IP pools from that router.' },
              { Field: 'IP Pool', Description: 'Select an IP pool from the router. Determines which IP range subscribers get when connecting. Sent as Framed-Pool RADIUS attribute.' },
            ]} />

            <Sub2>Burst Settings (Optional)</Sub2>
            <P>MikroTik burst gives users a temporary speed boost when they first start downloading:</P>
            <DefTable rows={[
              { Field: 'Burst Download / Upload', Description: 'Maximum burst speed in kb (e.g., 16000 for 16 Mbps burst on a 8 Mbps plan)' },
              { Field: 'Burst Threshold', Description: 'Speed threshold below which burst activates (typically same as normal speed)' },
              { Field: 'Burst Time', Description: 'Duration of the burst period in seconds (e.g., 10 for 10 seconds)' },
            ]} />

            <SubHeading>Duplicate Service</SubHeading>
            <P>
              Click any service row in the table to duplicate it. A dialog appears with the name pre-filled
              as "Name (Copy)". All settings are copied: speed, price, FUP tiers, CDN config, burst, pool,
              quotas, and time-based speed settings. This is the fastest way to create similar plans.
            </P>

            <SubHeading>Column Sorting</SubHeading>
            <P>
              Click the Name, Speed, or Price column headers to sort. Speed sorting is numeric (1M, 2M, 4M, 8M, 10M
              sorted correctly, not alphabetically). Sort preference persists across sessions via localStorage.
            </P>
          </section>

          {/* ============================================================
              FUP (FAIR USAGE POLICY)
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="fup">FUP (Fair Usage Policy)</SectionHeading>
            <P>
              FUP is ProxPanel's bandwidth management system that progressively reduces subscriber speed when they
              exceed their allocated quota. This ensures fair network usage and prevents heavy users from degrading
              performance for others.
            </P>

            <SubHeading>How FUP Works</SubHeading>
            <Steps items={[
              { title: 'Normal speed (FUP Level 0)', desc: 'subscriber uses internet at full service speed' },
              { title: 'Usage exceeds Tier 1 threshold', desc: 'speed is automatically reduced to FUP Level 1 speeds' },
              { title: 'Usage exceeds Tier 2 threshold', desc: 'speed is further reduced to FUP Level 2 speeds' },
              { title: 'Usage exceeds Tier 3 threshold', desc: 'speed drops to minimum FUP Level 3 speeds' },
              { title: 'Daily reset', desc: 'at the configured daily reset time, all daily counters and FUP levels reset to 0' },
            ]} />

            <SubHeading>Daily FUP — 3 Tiers</SubHeading>
            <P>Configured per service plan in the Services page. Each tier has three settings:</P>
            <DefTable rows={[
              { Setting: 'Threshold (bytes)', Description: 'How much data the subscriber must use before this tier activates. E.g., 5368709120 = 5 GB' },
              { Setting: 'Download Speed (kb)', Description: 'Reduced download speed when this tier is active. E.g., 4000 = 4 Mbps' },
              { Setting: 'Upload Speed (kb)', Description: 'Reduced upload speed when this tier is active' },
            ]} />
            <P>Example configuration for a "8MB-20GB" plan:</P>
            <DefTable rows={[
              { Tier: 'FUP Level 1', Threshold: '7 GB', Speed: '6000k/3000k (6M/3M)', Effect: 'Speed drops from 12M to 6M after 7 GB daily usage' },
              { Tier: 'FUP Level 2', Threshold: '14 GB', Speed: '4000k/2000k (4M/2M)', Effect: 'Speed drops to 4M after 14 GB' },
              { Tier: 'FUP Level 3', Threshold: '20 GB', Speed: '2000k/1000k (2M/1M)', Effect: 'Minimum speed of 2M after 20 GB' },
            ]} />
            <Tip>Leave threshold at 0 to disable a tier. For example, if you only want 2 FUP levels, leave Tier 3 threshold as 0.</Tip>

            <SubHeading>Monthly FUP — 3 Tiers</SubHeading>
            <P>
              Works identically to Daily FUP but tracks cumulative monthly usage. Monthly FUP counters only reset
              when the subscriber is <strong>renewed</strong> (not at the daily reset). Configure separate thresholds
              and speeds for each of the 3 monthly tiers.
            </P>

            <SubHeading>CDN FUP</SubHeading>
            <P>
              When CDN FUP is enabled on a service (checkbox in service settings), CDN traffic is tracked separately
              from regular internet traffic. Three CDN FUP tiers limit only CDN speeds when CDN thresholds are exceeded,
              while regular internet speed remains unaffected.
            </P>

            <SubHeading>Free Hours (Quota Discount)</SubHeading>
            <P>
              Configure a time window during which data usage is discounted (not counted fully toward the daily quota):
            </P>
            <DefTable rows={[
              { Setting: 'Start / End Time', Description: 'The time window for the discount (e.g., 00:00 to 06:00 for nighttime)' },
              { Setting: 'Download Free %', Description: 'Percentage of download usage that is FREE during this window' },
              { Setting: 'Upload Free %', Description: 'Percentage of upload usage that is FREE during this window' },
            ]} />
            <P>Examples:</P>
            <FeatureList items={[
              '100% free = nighttime usage is completely free (not counted at all toward daily quota)',
              '70% free = only 30% of usage during this window counts toward quota',
              '0% free = no discount (normal counting)',
            ]} />
            <Note>Free Hours only affect quota counting, NOT speed. To change speed during certain hours, use Speed Rules.</Note>

            <SubHeading>Daily Quota Reset</SubHeading>
            <P>
              The daily quota reset time is configured in Settings &rarr; RADIUS &rarr; "Daily Quota Reset Time".
              At this time each day, the system resets <Kbd>daily_download_used</Kbd>, <Kbd>daily_upload_used</Kbd>,
              and <Kbd>fup_level</Kbd> back to 0 for all subscribers. Usage history is saved before the reset.
            </P>

            <SubHeading>FUP Counters Page</SubHeading>
            <P>
              The FUP Counters page (admin only) shows all subscribers grouped by their current FUP level.
              Use the Reset button to manually reset FUP for individual subscribers or in bulk.
            </P>
          </section>

          {/* ============================================================
              CDN MANAGEMENT
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="cdn">CDN Management</SectionHeading>
            <P>
              CDN Management enables ISPs to apply separate speed policies for CDN (Content Delivery Network) traffic.
              This is essential when your network has a dedicated CDN peering link with different bandwidth than your
              regular internet uplink.
            </P>

            <SubHeading>CDN List (Subnets)</SubHeading>
            <P>
              Configure IP subnet ranges that represent your CDN network. When traffic from these subnets is detected,
              it is classified as CDN traffic and can have separate speed limits via PCQ (Per-Connection Queue) on MikroTik.
            </P>
            <Steps items={[
              { title: 'Add CDN subnets', desc: 'enter IP ranges like 172.16.0.0/16, 10.100.0.0/16 that represent your CDN' },
              { title: 'Assign to services', desc: 'link CDN subnets to specific service plans' },
              { title: 'Configure PCQ speed', desc: 'set the PCQ speed limit for CDN traffic per subscriber' },
              { title: 'Select target pool', desc: 'choose which MikroTik IP pool the PCQ rules apply to' },
              { title: 'Sync to MikroTik', desc: 'click "Sync to MikroTik" to push the configuration to routers' },
            ]} />

            <SubHeading>CDN Speed Rules (Night Boost)</SubHeading>
            <P>
              Time-based speed rules for CDN traffic only. For example, double CDN speed between midnight and 6 AM
              when CDN peering capacity is underutilized. Works independently from regular Speed Rules.
              Configure multiplier percentages, time windows, and active days.
            </P>

            <SubHeading>CDN Port Rules</SubHeading>
            <P>
              Port-based speed limiting using MikroTik PCQ queue and mangle rules. Each rule targets a specific
              port and creates dedicated speed queues:
            </P>
            <DefTable rows={[
              { Field: 'Name', Description: 'Descriptive rule name (e.g., "Streaming Limit", "Gaming Priority")' },
              { Field: 'Port', Description: 'Target TCP/UDP port number' },
              { Field: 'Direction', Description: 'src = match source port, dst = match destination port, both = match either, dscp = match DSCP value' },
              { Field: 'Speed (Mbps)', Description: 'Speed limit applied to matching traffic' },
              { Field: 'DSCP Value', Description: '(when direction = dscp) DSCP marking value 0-63 for QoS-based matching' },
              { Field: 'NAS', Description: 'Optional: target specific router (blank = all routers)' },
              { Field: 'Color', Description: 'Chart color when displaying on the live bandwidth graph' },
              { Field: 'Show in Graph', Description: 'Enable to display this rule\'s traffic as a separate series on the subscriber live graph' },
            ]} />

            <SubHeading>Live CDN Traffic Visualization</SubHeading>
            <P>
              When CDN subnets are configured, the subscriber live bandwidth graph shows CDN traffic as a separate
              colored series. Traffic is detected via MikroTik Torch by matching remote IPs against configured CDN
              subnet ranges. Port rule traffic is shown as dashed lines with their configured colors.
            </P>
          </section>

          {/* ============================================================
              NAS / ROUTERS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="nas">NAS / Routers</SectionHeading>
            <P>
              NAS (Network Access Server) devices are the MikroTik routers that authenticate PPPoE subscribers and
              enforce speed limits. Each router that handles PPPoE connections must be registered in ProxPanel.
            </P>

            <SubHeading>Adding a Router</SubHeading>
            <P>Click <Kbd>+ Add NAS</Kbd> and configure:</P>
            <DefTable rows={[
              { Field: 'Name', Description: 'Friendly name (e.g., "Main Router", "Building A Router")' },
              { Field: 'IP Address', Description: 'Management IP of the MikroTik router. This must be reachable from the ProxPanel server.' },
              { Field: 'RADIUS Secret', Description: 'Shared secret string that must match the MikroTik RADIUS configuration. Used to authenticate RADIUS packets.' },
              { Field: 'Auth Port', Description: 'RADIUS authentication port (default: 1812)' },
              { Field: 'Acct Port', Description: 'RADIUS accounting port (default: 1813). Receives session usage data from MikroTik.' },
              { Field: 'CoA Port', Description: 'Change of Authorization port (default: 1700 for MikroTik). Used to disconnect users and change speed.' },
              { Field: 'API Username', Description: 'MikroTik RouterOS API login. Required for live features (Torch, Ping, Traceroute, queue management).' },
              { Field: 'API Password', Description: 'MikroTik RouterOS API password' },
              { Field: 'Use SSL', Description: 'Use API-SSL port instead of plain API' },
              { Field: 'API SSL Port', Description: 'MikroTik API-SSL port (default: 8729)' },
            ]} />

            <SubHeading>MikroTik Configuration</SubHeading>
            <Card title="Required MikroTik Setup">
              <P>On your MikroTik router, configure these settings:</P>
              <FeatureList items={[
                'Add RADIUS server: /radius add address=YOUR_SERVER_IP secret=YOUR_SECRET service=ppp',
                'Enable RADIUS for PPP: /ppp aaa set use-radius=yes accounting=yes interim-update=00:00:30',
                'Enable API access: /ip service set api address=YOUR_SERVER_IP/32 (or api-ssl for encrypted)',
                'Ensure RADIUS ports are not firewalled (UDP 1812, 1813, 1700)',
              ]} />
              <Tip>Set interim-update to 00:00:30 (30 seconds) for accurate real-time quota tracking. This is how often MikroTik sends usage data to ProxPanel.</Tip>
            </Card>

            <SubHeading>Connectivity Status</SubHeading>
            <P>
              The NAS list shows two status indicators per router:
            </P>
            <FeatureList items={[
              'RADIUS — green check if ProxPanel can reach the router for authentication',
              'API — green check if MikroTik API credentials are configured and connection works',
              'Active Sessions — number of currently connected PPPoE users on this router',
              'Wrench icon — quick link to Diagnostic Tools pre-selecting this NAS',
            ]} />

            <SubHeading>IP Pool Auto-Import</SubHeading>
            <P>
              When a NAS is added with working API credentials, ProxPanel automatically imports all IP pools
              from the MikroTik router. These pools appear in the Services page for IP assignment. The system
              tracks every allocated IP to prevent duplicate IP addresses being assigned to multiple subscribers.
            </P>
          </section>

          {/* ============================================================
              SESSIONS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="sessions">Sessions</SectionHeading>
            <P>
              The Sessions page shows all currently active PPPoE sessions across all registered routers in real-time.
            </P>

            <SubHeading>Session Information</SubHeading>
            <DefTable rows={[
              { Column: 'Username', Description: 'PPPoE login name of the connected subscriber' },
              { Column: 'NAS', Description: 'Which MikroTik router the user is connected to' },
              { Column: 'IP Address', Description: 'Assigned IP address from the pool for this session' },
              { Column: 'MAC Address', Description: 'Client device MAC address used for this connection' },
              { Column: 'Session Time', Description: 'Duration of the current session (hours:minutes:seconds)' },
              { Column: 'Download / Upload', Description: 'Total bytes transferred in this session (since connection)' },
            ]} />

            <SubHeading>Disconnecting Users</SubHeading>
            <P>
              Select sessions and click <Kbd>Disconnect</Kbd> to send a RADIUS CoA (Change of Authorization)
              disconnect request. The MikroTik immediately terminates the PPPoE session. The client will typically
              auto-reconnect within seconds, getting a fresh IP from the pool.
            </P>

            <SubHeading>Stale Session Cleanup</SubHeading>
            <P>
              A background service runs every 5 minutes and automatically closes sessions that have no RADIUS
              accounting update for 30+ minutes. This handles cases where MikroTik doesn't send STOP packets
              (router reboot, network issues). It also syncs the <Kbd>is_online</Kbd> status of all subscribers.
            </P>
          </section>

          {/* ============================================================
              SPEED RULES
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="bandwidth-rules">Speed Rules</SectionHeading>
            <P>
              Speed Rules provide scheduled, time-based speed adjustments for services. Use them to implement
              policies like "double speed at night" or "full speed on weekends."
            </P>

            <SubHeading>Creating a Rule</SubHeading>
            <DefTable rows={[
              { Field: 'Name', Description: 'Descriptive name (e.g., "NIGHT BOOST", "WEEKEND TURBO")' },
              { Field: 'Services', Description: 'Select which service plans this rule applies to (multi-select)' },
              { Field: 'Download Multiplier %', Description: 'Speed multiplier for download. 100% = no change, 200% = double speed, 300% = triple speed.' },
              { Field: 'Upload Multiplier %', Description: 'Speed multiplier for upload. Same logic as download.' },
              { Field: 'Start Time', Description: 'When the rule activates (24h format, e.g., 00:00)' },
              { Field: 'End Time', Description: 'When the rule deactivates (e.g., 06:00)' },
              { Field: 'Active Days', Description: 'Which days of the week this rule runs (checkboxes for Mon-Sun)' },
            ]} />

            <SubHeading>How Rules Are Applied</SubHeading>
            <P>
              The Bandwidth Rule Service checks every 30 seconds. When a rule's time window is active:
            </P>
            <Steps items={[
              { title: 'Calculate new speed', desc: 'base speed (from service or subscriber override) x multiplier percentage' },
              { title: 'Update MikroTik queue', desc: 'adjusts the subscriber\'s PPPoE queue on the router via API' },
              { title: 'Update RADIUS radreply', desc: 'stores new speed so reconnections get the boosted speed' },
              { title: 'Falls back to CoA', desc: 'if API update fails, sends CoA to force reconnect with new speed' },
            ]} />

            <SubHeading>Rule Stacking with Per-Subscriber Rules</SubHeading>
            <P>
              If a subscriber has both a per-subscriber bandwidth rule (custom speed from their edit page) AND
              a global time-based bandwidth rule, the system uses the custom speed as the base:
            </P>
            <P>
              <strong>Final Speed = Per-Subscriber Custom Speed x Time-Based Multiplier</strong>
            </P>
            <P>Example: subscriber has 50M custom speed + NIGHT rule at 200% = 100M during night hours.</P>

            <Note title="Speed Rules vs Free Hours">
              <strong>Speed Rules</strong> change SPEED during a time window. <strong>Free Hours</strong> change QUOTA COUNTING
              during a time window. They serve different purposes and can be combined on the same service.
            </Note>
          </section>

          {/* ============================================================
              SUBSCRIBER WALLET
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="wallet">Subscriber Wallet</SectionHeading>
            <P>
              Each subscriber has a personal wallet balance that can be used to purchase services like Public IP
              addresses. The wallet system allows admins and resellers to add credit to subscriber accounts, and
              subscribers can spend their balance directly from the Customer Portal.
            </P>

            <SubHeading>How It Works</SubHeading>
            <Steps items={[
              { title: 'Admin or Reseller adds balance', desc: 'from the Subscribers page or Subscriber Edit page using the "Add Balance" button' },
              { title: 'Balance appears in subscriber wallet', desc: 'visible on the Subscriber Edit page and in the Customer Portal' },
              { title: 'Subscriber purchases services', desc: 'e.g., Public IP — amount is deducted from their wallet automatically' },
              { title: 'Transactions are logged', desc: 'every top-up and purchase creates a transaction record with before/after balance' },
            ]} />

            <SubHeading>Adding Balance (Admin)</SubHeading>
            <P>
              Admins can add balance to any subscriber for free — no deduction from anyone. This is useful for
              giving promotional credit, compensation, or manual top-ups after cash payment.
            </P>
            <FeatureList items={[
              'Go to Subscribers page → select subscriber → click "Add Balance" in the toolbar',
              'Or open Subscriber Edit page → click "Add Balance" button next to the wallet balance card',
              'Enter the amount and select a reason (Cash, Bank Transfer, Prepaid Card, Credit, Other)',
              'Balance is added instantly. A transaction record is created automatically.',
            ]} />

            <SubHeading>Adding Balance (Reseller)</SubHeading>
            <P>
              When a reseller adds balance to a subscriber, the amount is deducted from the reseller's own balance
              and transferred to the subscriber's wallet. This ensures resellers only give credit they have.
            </P>
            <FeatureList items={[
              'Reseller must have sufficient balance — the system checks before transferring',
              'Two transactions are created: a deduction from the reseller and a top-up for the subscriber',
              'Resellers need the "Refill Quota" permission to use this feature',
            ]} />

            <SubHeading>Subscriber Wallet in Customer Portal</SubHeading>
            <P>
              Subscribers can view their wallet balance and transaction history from the Customer Portal:
            </P>
            <DefTable rows={[
              { Feature: 'Dashboard Card', Description: 'Wallet balance is displayed on the dashboard. Click to go to the Wallet tab.' },
              { Feature: 'Wallet Tab', Description: 'Shows current balance and a full transaction history table with date, type, description, amount, and balance after each transaction.' },
              { Feature: 'Public IP Tab', Description: 'Subscribers can purchase a Public IP using their wallet balance. Shows price, balance check, and buy button.' },
            ]} />

            <SubHeading>Public IP Purchase</SubHeading>
            <P>
              Subscribers can buy a Public IP address directly from the Customer Portal if they have sufficient
              wallet balance. This is useful for customers who need remote access to devices like NVR cameras,
              servers, or smart home systems.
            </P>
            <Steps items={[
              { title: 'Subscriber opens Public IP tab', desc: 'in the Customer Portal' },
              { title: 'System shows available pools and pricing', desc: 'e.g., $20.00/mo' },
              { title: 'If balance is sufficient', desc: 'a "Buy" button appears. If not, a message shows how much more balance is needed.' },
              { title: 'On purchase', desc: 'the monthly price is deducted from the wallet and a Public IP is assigned. The connection reconnects automatically.' },
            ]} />

            <SubHeading>Transaction Types</SubHeading>
            <DefTable rows={[
              { Type: 'subscriber_topup', Color: 'Green', Description: 'Balance added to subscriber wallet (by admin or reseller)' },
              { Type: 'subscriber_purchase', Color: 'Red', Description: 'Balance spent on a service (e.g., Public IP purchase)' },
            ]} />

            <SubHeading>Balance Column in Subscribers Table</SubHeading>
            <P>
              The Subscribers list includes an optional "Balance" column (enable via the Columns button).
              Shows the current wallet balance — green for positive, gray for zero.
            </P>
          </section>

          {/* ============================================================
              BILLING & TRANSACTIONS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="billing">Billing &amp; Transactions</SectionHeading>
            <P>
              ProxPanel includes a complete billing system with transaction tracking, invoicing, prepaid cards,
              collector management, and revenue reporting.
            </P>

            <SubHeading>Transactions</SubHeading>
            <P>
              Every financial operation creates a transaction record. The Transactions page shows all records
              with filters for subscriber, type, date range, and amount.
            </P>
            <DefTable rows={[
              { Type: 'Renewal', Color: 'Green', Description: 'Service renewal — extends expiry date. Amount deducted from balance or billed to reseller.' },
              { Type: 'New', Color: 'Blue', Description: 'New subscriber registration. Records the initial charge.' },
              { Type: 'Payment', Color: 'Green', Description: 'Payment received from subscriber. Increases their balance.' },
              { Type: 'Charge', Color: 'Red', Description: 'Manual charge applied to account. Decreases balance.' },
              { Type: 'Refund', Color: 'Orange', Description: 'Credit returned to subscriber balance.' },
              { Type: 'Subscriber Top-up', Color: 'Green', Description: 'Balance added to subscriber wallet by admin or reseller.' },
              { Type: 'Subscriber Purchase', Color: 'Red', Description: 'Subscriber spent wallet balance on a service (e.g., Public IP).' },
            ]} />

            <SubHeading>Invoices</SubHeading>
            <P>
              Generate, view, and manage invoices from the Invoices page. Features include:
            </P>
            <FeatureList items={[
              'Auto-generated invoice numbers with configurable format',
              'Print-ready invoice layout with company branding',
              'Filter by subscriber, date range, and payment status',
              'Track paid vs unpaid invoices',
            ]} />

            <SubHeading>Prepaid Cards</SubHeading>
            <P>
              Generate batches of prepaid scratch cards that subscribers use to recharge their accounts:
            </P>
            <Steps items={[
              { title: 'Set denomination', desc: 'the card value (e.g., $10, $25, $50)' },
              { title: 'Set quantity', desc: 'how many cards to generate in this batch' },
              { title: 'Set expiry', desc: 'optional expiry date for the cards' },
              { title: 'Generate', desc: 'system creates unique codes for each card' },
              { title: 'Export', desc: 'download card codes for printing or distribution' },
            ]} />

            <SubHeading>Collectors</SubHeading>
            <P>
              Collectors are field agents who collect payments from subscribers. Create collector accounts from the
              Collectors page. Each collector gets their own login showing only:
            </P>
            <FeatureList items={[
              'Assigned subscribers — the list of subscribers they need to collect from',
              'Payment recording — mark payments as collected with amount and notes',
              'Collection history — their past collection records',
            ]} />
            <P>Admins can view all collectors, their performance, assigned subscribers, and collection totals.</P>

            <SubHeading>Revenue Reports</SubHeading>
            <P>
              The Reports page provides professional revenue analytics:
            </P>
            <FeatureList items={[
              '4 summary cards with total revenue and percentage change vs previous period',
              'Revenue breakdown by transaction type (pie chart + table)',
              'Revenue breakdown by service plan — see which plans generate the most income',
              'Revenue breakdown by reseller — track reseller contribution',
              'Daily revenue chart showing income trends over time',
              'Configurable date range picker (today, this week, this month, custom range)',
              'CSV export for all report data — download for accounting software',
            ]} />
          </section>

          {/* ============================================================
              COMMUNICATION RULES
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="communication">Communication Rules</SectionHeading>
            <P>
              Communication Rules automate notifications to subscribers via WhatsApp, SMS, or Email.
              Rules trigger automatically when specific conditions are met, keeping subscribers informed about
              their account status.
            </P>

            <SubHeading>Trigger Events</SubHeading>
            <DefTable rows={[
              { Trigger: 'FUP Applied', When: 'Real-time', Description: 'Fires when a subscriber hits a FUP tier (speed reduced). Filter by specific FUP levels (1, 2, or 3). Useful to notify users their speed was reduced.' },
              { Trigger: 'Quota Warning', When: 'Real-time', Description: 'Fires when monthly usage crosses a threshold percentage (e.g., 80%). The "days before" field is used as the percentage threshold (1-99).' },
              { Trigger: 'Expiry Warning', When: 'Scheduled', Description: 'Fires N days before a subscriber\'s account expires. Set "days before" to control when (e.g., 3 = fires 3 days before expiry).' },
              { Trigger: 'Expired', When: 'Scheduled', Description: 'Fires on the exact day the subscriber\'s account expires.' },
              { Trigger: 'Birthday', When: 'Scheduled', Description: 'Fires on the subscriber\'s birthday if date of birth is set.' },
            ]} />

            <SubHeading>Notification Channels</SubHeading>
            <DefTable rows={[
              { Channel: 'WhatsApp', Setup: 'Settings > Notifications > WhatsApp', Description: 'Messages sent via ProxRad WhatsApp integration. Requires scanning QR code to connect.' },
              { Channel: 'SMS', Setup: 'Settings > Notifications > SMS', Description: 'Supports Twilio, Vonage, or custom HTTP API providers.' },
              { Channel: 'Email', Setup: 'Settings > Notifications > SMTP', Description: 'SMTP email with TLS/STARTTLS support. Test button verifies configuration.' },
            ]} />

            <SubHeading>Template Variables</SubHeading>
            <P>Personalize notification messages with these template variables:</P>
            <DefTable rows={[
              { Variable: '{username}', Description: 'Subscriber PPPoE username' },
              { Variable: '{full_name}', Description: 'Subscriber full name' },
              { Variable: '{service}', Description: 'Service plan name (e.g., "8MB-20GB")' },
              { Variable: '{expiry_date}', Description: 'Account expiry date' },
              { Variable: '{days_before}', Description: 'Days until expiry (for expiry_warning trigger)' },
              { Variable: '{fup_level}', Description: 'Current FUP level number (for fup_applied trigger)' },
              { Variable: '{quota_used}', Description: 'Current usage amount (for quota_warning)' },
              { Variable: '{quota_total}', Description: 'Total quota limit (for quota_warning)' },
              { Variable: '{quota_percent}', Description: 'Usage percentage (for quota_warning)' },
            ]} />

            <SubHeading>Scheduling &amp; Deduplication</SubHeading>
            <FeatureList items={[
              'Expiry and expired notifications fire at the configured Notification Send Time (Settings > RADIUS)',
              'FUP and quota warnings fire in real-time when the condition is detected during QuotaSync',
              'Each rule is deduplicated per subscriber per day — a subscriber won\'t receive the same notification twice in one day',
              'The "Send to Reseller" option sends a copy to the subscriber\'s reseller (if they have WhatsApp connected)',
            ]} />

            <SubHeading>Per-Reseller WhatsApp Routing</SubHeading>
            <P>
              When a subscriber belongs to a reseller who has connected their own WhatsApp, notifications are sent
              from the reseller's WhatsApp number (personalized). Subscribers without a reseller, or whose reseller
              hasn't connected WhatsApp, receive messages from the admin's WhatsApp as a fallback.
            </P>
            <P>
              Individual subscriber WhatsApp notifications can be toggled on/off per subscriber from the WhatsApp page
              (bell icon toggle).
            </P>

            <SubHeading>Notification Banners</SubHeading>
            <P>
              Create in-panel notification banners that appear at the top of the admin panel for all users.
              Useful for maintenance notices, announcements, and alerts. Configure via Communication &rarr; Notifications.
            </P>
          </section>

          {/* ============================================================
              RESELLERS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="resellers">Resellers</SectionHeading>
            <P>
              The reseller system enables multi-level distribution where resellers manage their own subscribers and
              billing within controlled boundaries set by the admin.
            </P>

            <SubHeading>Reseller Features</SubHeading>
            <DefTable rows={[
              { Feature: 'Balance System', Description: 'Each reseller has a balance. When they create or renew a subscriber, the service price is deducted. Admin adds credit manually.' },
              { Feature: 'Credit Limit', Description: 'Maximum negative balance allowed. Set to 0 to require positive balance for all operations.' },
              { Feature: 'Multi-Level Hierarchy', Description: 'Resellers can have sub-resellers under them, creating a distribution chain.' },
              { Feature: 'Permission Groups', Description: 'Admin assigns permission groups to control which pages and actions each reseller can access.' },
              { Feature: 'Impersonation', Description: 'Admin can "Login as Reseller" to see exactly what the reseller sees. Useful for support.' },
              { Feature: 'WhatsApp Integration', Description: 'Each reseller can connect their own WhatsApp for subscriber notifications.' },
              { Feature: 'White-Label Branding', Description: 'Resellers with branding enabled can customize logo, company name, colors, and domain.' },
              { Feature: 'Custom Domain', Description: 'Resellers can point their own domain to the panel and request SSL certificates.' },
              { Feature: 'View All Subscribers', Description: 'With the "subscribers.view_all" permission, a reseller can see ALL subscribers (not just their own).' },
            ]} />

            <SubHeading>Creating a Reseller</SubHeading>
            <Steps items={[
              { title: 'Go to Resellers page', desc: 'click + Add Reseller' },
              { title: 'Set credentials', desc: 'username, password, email, phone' },
              { title: 'Set balance', desc: 'initial credit balance (e.g., $500)' },
              { title: 'Set credit limit', desc: 'how much negative balance is allowed' },
              { title: 'Assign permission group', desc: 'controls which features the reseller can access' },
              { title: 'Enable branding', desc: '(optional) allows reseller to customize their panel appearance' },
              { title: 'Assign parent', desc: '(optional) make this a sub-reseller under another reseller' },
            ]} />

            <SubHeading>Reseller Branding (White-Label)</SubHeading>
            <P>
              Resellers with branding enabled see a "Branding" menu in their sidebar. They can customize:
            </P>
            <FeatureList items={[
              'Company logo — replaces the ProxPanel logo in the sidebar and login page',
              'Company name — displayed in the sidebar header and browser title',
              'Primary color — changes the theme accent color',
              'Tagline — subtitle shown on the login page',
              'Footer text — custom text in the portal footer',
              'Live preview — changes are previewed in the sidebar immediately before saving',
            ]} />
            <P>
              Their subscribers' login page and customer portal display the reseller's branding instead of the
              default panel branding.
            </P>

            <SubHeading>Reseller Balance Display</SubHeading>
            <P>
              The reseller's current balance is shown in the blue title bar (next to the clock) and in the
              status bar at the bottom. Green for positive balance, red for negative. Balance refreshes
              automatically every 60 seconds.
            </P>
          </section>

          {/* ============================================================
              USERS & PERMISSIONS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="permissions">Users &amp; Permissions</SectionHeading>
            <P>
              The permission system provides granular access control with 50+ individual permissions organized
              into permission groups that can be assigned to resellers.
            </P>

            <SubHeading>Managing Users</SubHeading>
            <P>
              The Users page (admin only) manages admin and operator accounts. Create users with username,
              password, email, and user type. Show/hide passwords with the eye icon toggle.
            </P>

            <SubHeading>Permission Groups</SubHeading>
            <P>
              Create permission groups from the Permissions page:
            </P>
            <Steps items={[
              { title: 'Click + New Group', desc: 'enter a name like "Sales", "Support", "Finance"' },
              { title: 'Select permissions', desc: 'check individual permissions from the full list' },
              { title: 'Save', desc: 'the group is now available for assignment' },
              { title: 'Assign to resellers', desc: 'in the Resellers page, select the group for each reseller' },
            ]} />

            <SubHeading>Permission Categories</SubHeading>
            <DefTable rows={[
              { Category: 'Subscribers', Permissions: 'view, create, edit, delete, renew, reset_fup, reset_mac, disconnect, inactivate, change_service, add_days, refill_quota, rename, ping, view_graph, bandwidth_rules, view_all, change_bulk, torch' },
              { Category: 'Services', Permissions: 'view, create, edit, delete' },
              { Category: 'Sessions', Permissions: 'view' },
              { Category: 'NAS', Permissions: 'view, create, edit, delete' },
              { Category: 'Resellers', Permissions: 'view' },
              { Category: 'Transactions', Permissions: 'view' },
              { Category: 'Invoices', Permissions: 'view' },
              { Category: 'Prepaid', Permissions: 'view, create, edit' },
              { Category: 'Reports', Permissions: 'view' },
              { Category: 'Tickets', Permissions: 'view' },
              { Category: 'Backups', Permissions: 'view, create, restore, delete, edit' },
              { Category: 'Settings', Permissions: 'view' },
              { Category: 'Communication', Permissions: 'access_module, notifications' },
              { Category: 'Bandwidth', Permissions: 'view' },
              { Category: 'CDN', Permissions: 'view' },
              { Category: 'Sharing', Permissions: 'view' },
              { Category: 'Audit', Permissions: 'view' },
              { Category: 'Users', Permissions: 'view' },
              { Category: 'Permissions', Permissions: 'view' },
              { Category: 'Dashboard', Permissions: 'view' },
              { Category: 'Collectors', Permissions: 'view' },
            ]} />

            <SubHeading>How Permissions Work</SubHeading>
            <FeatureList items={[
              'Admin users always have ALL permissions (no restrictions)',
              'Resellers with NO permission group assigned have ALL permissions (backward compatibility)',
              'Resellers WITH a permission group only have the specific permissions enabled in that group',
              'Permissions refresh automatically every 2 minutes — no logout required when admin changes a group',
              'Both backend (API) and frontend (page/button visibility) enforce permissions',
              'The Permissions page shows which resellers are assigned to each group',
            ]} />
          </section>

          {/* ============================================================
              BACKUPS & RECOVERY
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="backups">Backups &amp; Recovery</SectionHeading>
            <P>
              ProxPanel provides comprehensive backup capabilities to protect your data with encrypted backups,
              scheduled automation, cloud storage, and cross-server restoration.
            </P>

            <SubHeading>Backup Types</SubHeading>
            <DefTable rows={[
              { Type: 'Manual Backup', Description: 'Click "Create Backup" to generate an encrypted backup immediately. File is stored on the server.' },
              { Type: 'Scheduled Backup', Description: 'Automatic backups on a schedule (daily, weekly, monthly) with configurable retention policies.' },
              { Type: 'Cloud Backup', Description: 'Store backups on ProxPanel Cloud. Free tier: 500 MB. Paid tiers available up to 100 GB.' },
              { Type: 'FTP Backup', Description: 'Send backups to an external FTP/SFTP server. Configure host, port, credentials, and path.' },
              { Type: 'Local + Cloud', Description: 'Keep a local copy AND upload to cloud for maximum protection.' },
            ]} />

            <SubHeading>Backup Encryption</SubHeading>
            <P>
              All backups are encrypted with AES-256-GCM using a key derived from your license. This means:
            </P>
            <FeatureList items={[
              'Backups cannot be read without the encryption key',
              'Each customer has a unique encryption key (license-bound)',
              'V2 backup format embeds the license key in the header for automatic cross-server restoration',
              'V1 backups (older) require manually entering the source license key for cross-server restore',
            ]} />

            <SubHeading>Restoring Backups</SubHeading>
            <Card title="Same Server Restore">
              <Steps items={[
                { title: 'Go to Backups page' },
                { title: 'Click the restore icon next to the backup file' },
                { title: 'Confirm the restore' },
                { title: 'System decrypts and restores the database automatically' },
              ]} />
            </Card>
            <Card title="Cross-Server Restore (V2 Backups)">
              <Steps items={[
                { title: 'Download the backup file from the source server' },
                { title: 'Upload it to the new server via "Upload & Restore"' },
                { title: 'The system auto-detects the source license key from the V2 header' },
                { title: 'Fetches the correct decryption password from the license server' },
                { title: 'Decrypts and restores — fully automatic, no manual key entry needed' },
              ]} />
            </Card>
            <Card title="Cross-Server Restore (V1 Backups)">
              <Steps items={[
                { title: 'Upload the V1 backup file' },
                { title: 'Enter the source server\'s license key in the "Source License Key" field' },
                { title: 'Click Restore — system fetches the decryption password using the provided key' },
              ]} />
            </Card>

            <SubHeading>Scheduled Backup Configuration</SubHeading>
            <DefTable rows={[
              { Setting: 'Frequency', Description: 'Daily, Weekly (select day), or Monthly (select date)' },
              { Setting: 'Time of Day', Description: 'When the backup runs (uses your server\'s configured timezone)' },
              { Setting: 'Retention', Description: 'How many backups to keep. Older backups are automatically deleted.' },
              { Setting: 'Storage Type', Description: 'Local, Cloud, FTP, or Local + Cloud' },
            ]} />

            <Warning title="Test Your Backups">
              Regularly download a backup and test restoration on a separate server. A backup you can't restore is worthless.
            </Warning>
          </section>

          {/* ============================================================
              SHARING DETECTION
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="sharing">Sharing Detection</SectionHeading>
            <P>
              Sharing Detection identifies subscribers who share their PPPoE account across multiple households
              or locations by analyzing network packet TTL (Time To Live) values.
            </P>

            <SubHeading>How TTL Detection Works</SubHeading>
            <P>
              Every device sets a default TTL value in its outgoing packets: Windows uses 128, Linux/macOS use 64.
              When a device is behind a NAT router, the TTL decreases by 1 per hop. If ProxPanel sees packets from
              the same subscriber with significantly different TTL values (e.g., both 127 and 63), it indicates
              multiple devices are connected — possibly in different locations sharing one PPPoE account.
            </P>

            <SubHeading>Automatic Scanning</SubHeading>
            <DefTable rows={[
              { Setting: 'Enable Auto Scan', Description: 'Toggle automatic nightly scanning on/off' },
              { Setting: 'Scan Schedule', Description: 'When to run (default: 2:00 AM daily, when most sharing is detectable)' },
              { Setting: 'TTL Threshold', Description: 'Minimum TTL difference to flag as sharing (e.g., 3 = differences of 3+ are flagged)' },
              { Setting: 'Auto Disconnect', Description: 'Automatically disconnect flagged subscribers (use with caution)' },
            ]} />

            <SubHeading>Manual Scan</SubHeading>
            <P>
              Click <Kbd>Run Scan Now</Kbd> to perform an on-demand scan. Results show each flagged subscriber
              with detected TTL values, estimated device count, and scan timestamp. Review results before taking action.
            </P>
          </section>

          {/* ============================================================
              SUPPORT TICKETS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="tickets">Support Tickets</SectionHeading>
            <P>
              A built-in support ticket system allows subscribers to submit issues and requests directly from the
              Customer Portal, and admins/resellers to manage and respond from the panel.
            </P>

            <SubHeading>How Tickets Work</SubHeading>
            <Steps items={[
              { title: 'Customer creates a ticket', desc: 'from the Customer Portal, the subscriber writes a subject and description' },
              { title: 'Admin/reseller sees new ticket', desc: 'open the Tickets page from the sidebar to view all tickets with status filters' },
              { title: 'Reply to ticket', desc: 'click a ticket to open it, read the conversation, and type a reply' },
              { title: 'Close ticket', desc: 'mark the ticket as resolved when the issue is handled' },
            ]} />

            <SubHeading>Ticket Statuses</SubHeading>
            <DefTable rows={[
              { Status: 'Open', Description: 'New or active ticket awaiting response' },
              { Status: 'In Progress', Description: 'Ticket is being worked on' },
              { Status: 'Closed', Description: 'Issue resolved and ticket closed' },
            ]} />

            <SubHeading>Filtering & Management</SubHeading>
            <FeatureList items={[
              'Filter by status (Open, In Progress, Closed)',
              'Filter by subscriber name or username',
              'View ticket history and all replies in conversation format',
              'Resellers only see tickets from their own subscribers',
            ]} />
          </section>

          {/* ============================================================
              AUDIT & SYSTEM LOGS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="audit">Audit &amp; System Logs</SectionHeading>
            <P>
              ProxPanel maintains detailed audit trails and system logs for security monitoring, compliance,
              and troubleshooting.
            </P>

            <SubHeading>Audit Logs</SubHeading>
            <P>
              The Audit Logs page records every significant action performed in the panel:
            </P>
            <FeatureList items={[
              'Who performed the action (username and IP address)',
              'What was done (e.g., "Updated subscriber info1633: Status: Active → Inactive, Price: $0.00 → $25.00")',
              'When it happened (timestamp with timezone)',
              'Which entity was affected (subscriber, service, NAS, etc.)',
              'Filter by user, action type, entity, and date range',
            ]} />
            <P>
              Audit logs track subscriber changes (with exact field-by-field diff), login attempts,
              setting modifications, backup operations, and administrative actions.
            </P>

            <SubHeading>System Logs</SubHeading>
            <P>
              The System Logs page (admin only) shows technical log output from the backend services:
            </P>
            <FeatureList items={[
              'API server logs — request/response activity, errors, warnings',
              'RADIUS server logs — authentication attempts, accounting updates',
              'Background service logs — QuotaSync, FUP enforcement, backup scheduler',
              'Useful for diagnosing connectivity issues, RADIUS failures, and performance problems',
            ]} />
          </section>

          {/* ============================================================
              DIAGNOSTIC TOOLS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="diagnostics">Diagnostic Tools</SectionHeading>
            <P>
              Network troubleshooting utilities that execute through your MikroTik routers or directly from the server.
              Access from the sidebar or via the wrench icon on any NAS row (pre-selects that router).
            </P>

            <SubHeading>Ping</SubHeading>
            <FeatureList items={[
              'Runs through the selected MikroTik router (not from the server)',
              'Search field with subscriber autocomplete — type a username to auto-fill their IP',
              'Configurable packet count (1-100) and packet size (64-64000 bytes)',
              'Results stream live showing each reply with round-trip time',
              'Summary shows packets sent/received, loss percentage, and min/avg/max RTT',
            ]} />

            <SubHeading>Traceroute</SubHeading>
            <FeatureList items={[
              'Runs through MikroTik for private IPs, server-side for public IPs',
              'Shows each hop with IP address, packet loss percentage, and RTT stats',
              'Useful for diagnosing routing issues between router and subscriber',
            ]} />

            <SubHeading>NSLookup</SubHeading>
            <FeatureList items={[
              'DNS resolution from the server',
              'Shows A (IPv4), AAAA (IPv6), CNAME, MX, NS, and TXT records',
              'Useful for verifying DNS configuration and troubleshooting resolution issues',
            ]} />
          </section>

          {/* ============================================================
              SETTINGS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="settings">Settings</SectionHeading>
            <P>
              The Settings page contains all system configuration organized into tabs. Each tab URL is persistent
              (e.g., <Kbd>/settings?tab=license</Kbd>) so you can bookmark specific tabs.
            </P>

            <SubHeading>Branding Tab</SubHeading>
            <P>Customize the panel's visual identity:</P>
            <DefTable rows={[
              { Setting: 'Company Name', Description: 'Shown in sidebar, login page, and browser title' },
              { Setting: 'Company Logo', Description: 'Replaces the company name text when uploaded (recommended: 200x50px PNG)' },
              { Setting: 'Primary Color', Description: 'Theme accent color. 6 preset colors available or use custom hex picker.' },
              { Setting: 'Favicon', Description: 'Browser tab icon. Upload a square image (32x32 or 64x64 PNG).' },
              { Setting: 'Login Background', Description: 'Custom background image for the login page (replaces default blue gradient)' },
              { Setting: 'Tagline', Description: 'Subtitle text on login page (default: "High Performance ISP Management Solution")' },
              { Setting: 'Footer Copyright', Description: 'Custom text in the login page footer' },
              { Setting: 'Feature Boxes', Description: 'Toggle and customize 3 feature highlight boxes on the login page' },
            ]} />

            <SubHeading>General Tab</SubHeading>
            <P>System timezone, date format, currency, and general preferences.</P>

            <SubHeading>Billing Tab</SubHeading>
            <P>Invoice numbering format, tax settings, default payment terms.</P>

            <SubHeading>Service Change Tab</SubHeading>
            <P>Configure what happens when a subscriber's service plan is changed: prorate billing, keep expiry date, force disconnect.</P>

            <SubHeading>RADIUS Tab</SubHeading>
            <DefTable rows={[
              { Setting: 'Daily Quota Reset Time', Description: 'When daily quotas and FUP levels reset (e.g., 00:05 for 12:05 AM)' },
              { Setting: 'Notification Send Time', Description: 'When scheduled notifications fire (expiry warnings, expired notifications)' },
              { Setting: 'Session Timeout', Description: 'Inactivity timeout for admin/reseller panel sessions (default: 10 minutes)' },
            ]} />

            <SubHeading>Notifications Tab</SubHeading>
            <P>Configure notification providers:</P>
            <FeatureList items={[
              'SMTP Email — server, port, TLS mode, username/password, from address. Test button sends a test email.',
              'SMS — choose Twilio, Vonage, or Custom HTTP API. Configure API credentials and sender ID. Test button sends a test SMS.',
              'WhatsApp — connect via ProxRad by scanning a QR code. Shows connection status and connected phone number.',
            ]} />

            <SubHeading>Security Tab</SubHeading>
            <P>Session timeout, rate limiting, and remote support toggle.</P>

            <SubHeading>Account Tab</SubHeading>
            <P>Change admin password, update email and profile information.</P>

            <SubHeading>License Tab</SubHeading>
            <FeatureList items={[
              'License status — Active/Expired/Blocked with tier name',
              'Subscriber count — current vs maximum allowed by license tier',
              'Expiration date — when the license expires',
              'Check for Updates — checks the license server for available system updates',
              'Install Update — downloads and installs the latest update (see System Updates section)',
              'Restart API / Restart All Services — restart containers without SSH access',
            ]} />

            <SubHeading>System Info Tab</SubHeading>
            <P>
              Server hardware information: environment type (Physical/VM/LXC/Docker), CPU model and cores,
              RAM total and usage, disk size and type (SSD/NVMe/HDD), estimated capacity, OS version, and uptime.
              Shows recommendations based on your hardware configuration.
            </P>

            <SubHeading>Network Tab</SubHeading>
            <P>View and configure server network interfaces, IP addresses, routes, and DNS settings.</P>

            <SubHeading>SSL Certificate Tab</SubHeading>
            <P>
              Configure HTTPS for your panel domain. Enter your domain name and email, then click
              "Get SSL Certificate" to automatically obtain a free Let's Encrypt certificate. See the
              <a href="#ssl" className="text-blue-600 dark:text-blue-400 hover:underline ml-1">SSL / HTTPS section</a> for
              full details. This tab also contains the Remote Access settings for Cloudflare tunnel.
            </P>

            <SubHeading>API Keys Tab</SubHeading>
            <P>
              Generate and manage API keys for external integrations. Each key has configurable scopes
              (read, write, delete). See the <a href="#api" className="text-blue-600 dark:text-blue-400 hover:underline ml-1">API Integration section</a> for details.
            </P>

            <SubHeading>Cluster Tab</SubHeading>
            <P>
              High Availability cluster configuration. Set up main/secondary server roles, monitor node health,
              and perform failover. See the <a href="#ha-cluster" className="text-blue-600 dark:text-blue-400 hover:underline ml-1">High Availability section</a> for details.
            </P>
          </section>

          {/* ============================================================
              REMOTE ACCESS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="remote-access">Remote Access</SectionHeading>
            <P>
              ProxPanel supports Cloudflare tunnel-based remote access, allowing you to access your panel from
              anywhere without opening any ports on your firewall.
            </P>

            <SubHeading>How It Works</SubHeading>
            <Steps items={[
              { title: 'Enable Remote Access', desc: 'toggle in Settings' },
              { title: 'System creates a Cloudflare tunnel', desc: 'using the built-in ProxRad Cloudflare credentials' },
              { title: 'DNS record created', desc: 'your panel becomes accessible at panel-XXXXXX.proxrad.com' },
              { title: 'Access from anywhere', desc: 'use the proxrad.com URL from any internet connection' },
            ]} />
            <P>
              The tunnel is encrypted end-to-end and managed by the cloudflared daemon. No port forwarding or
              firewall changes are needed. Disable Remote Access to remove the tunnel and DNS record.
            </P>

            <SubHeading>Remote Support</SubHeading>
            <P>
              When Remote Support is enabled in Settings &rarr; Security, an SSH tunnel is established to the
              ProxPanel license server. This allows authorized ProxPanel support staff to access your server
              for troubleshooting. Disable Remote Support to immediately close the tunnel and clear credentials.
            </P>
          </section>

          {/* ============================================================
              SSL / HTTPS
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="ssl">SSL / HTTPS</SectionHeading>
            <P>
              Secure your panel with HTTPS using a free Let's Encrypt SSL certificate. The entire process takes
              less than a minute — just point your domain and click one button.
            </P>

            <SubHeading>Setting Up SSL (3 Steps)</SubHeading>
            <Steps items={[
              { title: 'Point your domain to the server', desc: 'create a DNS A record for your domain (e.g., panel.yourisp.com) pointing to your server\'s public IP address. Wait for DNS propagation (usually 1-5 minutes).' },
              { title: 'Enter domain and email', desc: 'go to Settings → SSL Certificate tab. Type your domain name and your email address (used for Let\'s Encrypt certificate expiry notifications).' },
              { title: 'Click "Get SSL Certificate"', desc: 'the system automatically runs certbot, obtains the certificate, configures nginx for HTTPS on port 443, and redirects all HTTP traffic to HTTPS. A live terminal log shows the progress.' },
            ]} />
            <Tip title="That's it!">
              After clicking "Get SSL Certificate", your panel will be accessible at <strong>https://yourdomain.com</strong> within seconds.
              The green status bar will confirm "SSL active" with your domain name.
            </Tip>

            <SubHeading>SSL Status Indicators</SubHeading>
            <DefTable rows={[
              { Status: 'Green bar — "SSL active"', Description: 'Certificate is installed and working. Panel is accessible via HTTPS.' },
              { Status: 'Yellow bar — "Domain configured but no certificate"', Description: 'Domain is saved but certbot hasn\'t run yet. Click "Get SSL Certificate" to install.' },
              { Status: 'No bar shown', Description: 'SSL not configured yet. Enter your domain and email to get started.' },
              { Status: '"Renew SSL Certificate" button', Description: 'Shown when SSL is already active. Click to renew the certificate (Let\'s Encrypt certificates expire every 90 days).' },
            ]} />

            <SubHeading>Prerequisites</SubHeading>
            <FeatureList items={[
              'A registered domain name (e.g., panel.yourisp.com, isp.example.com)',
              'DNS A record pointing to your server\'s public IP address',
              'Port 80 must be accessible from the internet (Let\'s Encrypt uses HTTP challenge to verify domain ownership)',
              'Server must have a public IP (not behind NAT without port forwarding)',
            ]} />

            <SubHeading>Important Notes</SubHeading>
            <FeatureList items={[
              'SSL configuration is preserved during system updates — nginx.conf is NOT overwritten when SSL is detected',
              'Let\'s Encrypt certificates are valid for 90 days. Renew from the same SSL tab before expiry.',
              'After SSL is configured, nginx automatically handles port 443 and redirects HTTP to HTTPS',
              'If you change your server IP, update the DNS A record and re-run the certificate installation',
            ]} />

            <SubHeading>Reseller Custom Domains</SubHeading>
            <P>
              Resellers with branding enabled can set a custom domain (e.g., portal.myisp.com). They configure
              an A record pointing to the server IP, then request an SSL certificate from their Branding page.
              The certbot output streams in real-time in a terminal log. This allows resellers to offer a
              fully white-labeled experience to their subscribers.
            </P>
          </section>

          {/* ============================================================
              CUSTOMER PORTAL
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="customer-portal">Customer Portal</SectionHeading>
            <P>
              The Customer Portal is a self-service interface for end subscribers. Customers log in at the same
              URL as the admin panel using their PPPoE username and password, and are automatically redirected
              to the portal view.
            </P>

            <SubHeading>Portal Features</SubHeading>
            <DefTable rows={[
              { Feature: 'Dashboard', Description: 'Connection status (online/offline), service plan details, expiry date, monthly price, wallet balance with circular progress rings for usage' },
              { Feature: 'Usage Statistics', Description: 'Daily and monthly download/upload usage with visual progress indicators and percentage bars' },
              { Feature: 'Wallet', Description: 'View wallet balance and full transaction history. Balance is used to purchase services like Public IP.' },
              { Feature: 'Public IP', Description: 'Purchase a Public IP address using wallet balance. Explains what a Public IP is and how to get one. Shows available pools and pricing.' },
              { Feature: 'Account Info', Description: 'Personal details, subscription information, assigned service' },
              { Feature: 'Invoices', Description: 'View and download past invoices' },
              { Feature: 'Support Tickets', Description: 'Create and track support requests. View ticket history and replies.' },
              { Feature: 'Password Change', Description: 'Update PPPoE password directly from the portal' },
            ]} />

            <SubHeading>Branding in Portal</SubHeading>
            <P>
              The Customer Portal inherits branding from the subscriber's reseller (if branding is enabled).
              If the subscriber belongs to a reseller with a custom logo and colors, the portal displays
              the reseller's branding. Otherwise, the default admin branding is shown.
            </P>
          </section>

          {/* ============================================================
              MOBILE APP
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="mobile-app">Mobile App</SectionHeading>
            <P>
              ProxPanel includes a mobile application for admin, reseller, and customer access on smartphones.
            </P>

            <SubHeading>Features</SubHeading>
            <FeatureList items={[
              'Admin screens — manage subscribers, view dashboard, handle basic operations',
              'Reseller screens — manage owned subscribers, view balance, record payments',
              'Customer screens — view usage, account details, submit tickets',
              'Shared screens — login, profile, password change',
              'Responsive design optimized for mobile touch interactions',
            ]} />

            <SubHeading>Installation</SubHeading>
            <P>
              The mobile app can be installed from Settings &rarr; General. A QR code is provided for quick
              download. The app connects to your panel server using the same URL.
            </P>
          </section>

          {/* ============================================================
              SYSTEM UPDATES
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="updates">System Updates</SectionHeading>
            <P>
              ProxPanel receives regular updates with new features, bug fixes, and security improvements.
              Updates are managed from Settings &rarr; License tab.
            </P>

            <SubHeading>Update Process</SubHeading>
            <Steps items={[
              { title: 'Check for updates', desc: 'click "Check for Updates" in Settings > License' },
              { title: 'Review version', desc: 'the available version number and release notes are displayed' },
              { title: 'Install', desc: 'click "Install Update" to download and apply' },
              { title: 'Automatic restart', desc: 'containers restart automatically. Takes about 30 seconds.' },
              { title: 'Verify', desc: 'check the version number in the title bar to confirm the update applied' },
            ]} />

            <SubHeading>What Gets Updated</SubHeading>
            <FeatureList items={[
              'API server binary (Go backend) — new features, bug fixes, performance improvements',
              'RADIUS server binary — authentication and accounting improvements',
              'Frontend (React app) — UI changes, new pages, bug fixes',
              'Docker compose configuration — new container settings if needed',
              'Nginx configuration — new routes (except SSL config, which is preserved)',
            ]} />

            <SubHeading>Update Notification</SubHeading>
            <P>
              A bell icon in the title bar shows when a new update is available. Click it to go directly to the
              License settings tab for installation.
            </P>

            <Note title="After Updates">
              Browsers may cache the old frontend. If you notice visual issues after an update, press <Kbd>Ctrl + F5</Kbd> (hard refresh).
              ProxPanel's nginx is configured to prevent caching of index.html, but browser extensions may override this.
            </Note>
          </section>

          {/* ============================================================
              HIGH AVAILABILITY
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="ha-cluster">High Availability (HA Cluster)</SectionHeading>
            <P>
              For ISPs requiring maximum uptime, ProxPanel supports High Availability clustering with automatic
              failover. Configure a secondary server that takes over if the primary goes down.
            </P>

            <SubHeading>Architecture</SubHeading>
            <DefTable rows={[
              { Role: 'Main Server', Description: 'Primary server handling all reads and writes. PostgreSQL primary, RADIUS primary.' },
              { Role: 'Secondary Server', Description: 'Standby server with real-time data replication. PostgreSQL streaming replica. RADIUS backup.' },
              { Role: 'Standalone', Description: 'Default single-server mode (no clustering).' },
            ]} />

            <SubHeading>Setting Up HA</SubHeading>
            <Card title="On the Main Server">
              <Steps items={[
                { title: 'Go to Settings > Cluster', desc: '' },
                { title: 'Click "Configure as Main Server"', desc: 'generates Cluster ID and Secret Key' },
                { title: 'Note the Cluster Secret', desc: 'you\'ll need this for the secondary server' },
              ]} />
            </Card>
            <Card title="On the Secondary Server">
              <Steps items={[
                { title: 'Go to Settings > Cluster', desc: '' },
                { title: 'Enter Main Server IP and Cluster Secret', desc: '' },
                { title: 'Click "Test Connection"', desc: 'validates API, database, and Redis connectivity' },
                { title: 'Click "Join Cluster"', desc: 'PostgreSQL replication begins automatically' },
              ]} />
            </Card>

            <SubHeading>Failover</SubHeading>
            <P>
              When the main server goes offline, the secondary server detects this within 2 minutes.
              A prominent banner appears: "Main Server Offline." Click <Kbd>Promote to Main Server</Kbd>
              for one-click failover. The secondary promotes its PostgreSQL to primary, stops Redis replication,
              and becomes the new main server. Then update your MikroTik RADIUS to point to the new IP.
            </P>

            <SubHeading>Disaster Recovery</SubHeading>
            <P>
              If your main server is destroyed and you need to set up a completely new server, the Cluster page
              offers "Recover from Existing Server." Enter the surviving server's IP and credentials to
              automatically transfer all data, uploads, and configuration to the new installation.
            </P>
          </section>

          {/* ============================================================
              API INTEGRATION
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="api">API Integration</SectionHeading>
            <P>
              ProxPanel provides a REST API for integrating with external systems such as CRM, billing software,
              accounting systems, and custom applications.
            </P>

            <SubHeading>Getting Started</SubHeading>
            <Steps items={[
              { title: 'Generate an API key', desc: 'go to Settings > API Keys and create a new key' },
              { title: 'Select scopes', desc: 'choose read, write, and/or delete permissions for the key' },
              { title: 'Use the key', desc: 'pass it in the X-API-Key header on every request' },
            ]} />

            <SubHeading>Available Endpoints</SubHeading>
            <FeatureList items={[
              'Subscribers — list, create, update, delete, suspend, activate, get usage',
              'Services — list all service plans, get plan details',
              'NAS Devices — list routers, get device info and session counts',
              'Transactions — list with filters, create new transactions',
              'System — get system-wide statistics and health check',
            ]} />

            <SubHeading>Rate Limiting</SubHeading>
            <P>API requests are limited to 60 per minute per API key. Exceeding this returns HTTP 429.</P>

            <div className="mt-5 p-5 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm">
              <p className="text-sm text-gray-700 dark:text-gray-300 font-semibold mb-2">Full API Reference</p>
              <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                Complete endpoint documentation with request/response examples, parameters, cURL and JavaScript code samples.
              </p>
              <a
                href="/api-docs"
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors shadow-sm"
              >
                View Full API Documentation
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
                </svg>
              </a>
            </div>
          </section>

          {/* ============================================================
              TROUBLESHOOTING
              ============================================================ */}
          <section className="mb-16 pt-10 border-t border-gray-200 dark:border-gray-800 first:border-t-0 first:pt-0">
            <SectionHeading id="troubleshooting">Troubleshooting</SectionHeading>
            <P>Common issues and their solutions:</P>

            <SubHeading>Subscriber Can't Connect (PPPoE Timeout)</SubHeading>
            <Card title="Possible Causes & Solutions">
              <FeatureList items={[
                'Account expired or inactive — check subscriber status in the panel. Renew or activate as needed.',
                'Wrong password — verify the password on the subscriber edit page. Reset if needed.',
                'NAS not registered — the MikroTik\'s IP must be added as a NAS device with correct RADIUS secret.',
                'RADIUS secret mismatch — the secret in ProxPanel must exactly match the MikroTik RADIUS configuration.',
                'RADIUS ports blocked — ensure UDP 1812 and 1813 are not firewalled between MikroTik and ProxPanel server.',
                'MAC binding — if MAC binding is enabled and the subscriber is using a different device, authentication fails.',
              ]} />
            </Card>

            <SubHeading>Speed Not Matching Service Plan</SubHeading>
            <Card title="Possible Causes & Solutions">
              <FeatureList items={[
                'FUP applied — subscriber may have exceeded daily quota. Check their FUP level on the edit page.',
                'Speed Rule active — a time-based speed rule may be modifying the speed. Check Speed Rules page.',
                'Per-subscriber rule — a custom speed rule may override the service speed. Check the subscriber\'s Speed Rules section.',
                'MikroTik queue conflict — check if there are manual queues on MikroTik that conflict with RADIUS-applied queues.',
                'Speed format issue — ensure service speeds are in kb format (2000 = 2 Mbps, not 2).',
              ]} />
            </Card>

            <SubHeading>Panel Shows Wrong Online Count</SubHeading>
            <Card title="Possible Causes & Solutions">
              <FeatureList items={[
                'Stale sessions — if MikroTik didn\'t send STOP packets (reboot), ghost sessions remain. The StaleSessionCleanup service handles this automatically every 5 minutes.',
                'RADIUS accounting disabled — ensure interim-update is set to 30 seconds on MikroTik PPP AAA settings.',
                'Multiple NAS devices — online count aggregates all routers. Check Sessions page for per-NAS breakdown.',
              ]} />
            </Card>

            <SubHeading>502 Bad Gateway After Restart</SubHeading>
            <P>
              If you see 502 after restarting containers, Nginx may need to be reloaded. Wait 30 seconds for all
              containers to start, then the issue typically resolves automatically. If it persists, restart the
              frontend container from Settings &rarr; License &rarr; Restart All Services.
            </P>

            <SubHeading>Forgot Admin Password</SubHeading>
            <P>
              Contact your system administrator or ProxPanel support. The password can be reset via the database
              or the license server admin panel.
            </P>

            <SubHeading>License Issues</SubHeading>
            <FeatureList items={[
              'License expired — contact ProxPanel to renew your license.',
              'Hardware mismatch — if you migrated to a new server, the hardware binding needs to be updated on the license server.',
              'Subscriber limit reached — upgrade your license tier to support more subscribers.',
              'Revalidate — click "Revalidate License" in Settings > License to force a fresh license check.',
            ]} />
          </section>

          {/* Footer */}
          <div className="mt-16 pt-8 border-t border-gray-200 dark:border-gray-800 text-center space-y-3">
            <p className="text-sm text-gray-500 dark:text-gray-400 font-medium">
              ProxPanel System Documentation
            </p>
            <p className="text-xs text-gray-400 dark:text-gray-500">
              Need additional help? Contact your system administrator or reach out to ProxPanel support.
            </p>
            <a href="/" className="inline-flex items-center gap-2 text-xs text-blue-600 dark:text-blue-400 hover:underline">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" /></svg>
              Back to Panel
            </a>
          </div>
        </main>
      </div>
    </div>
  )
}
