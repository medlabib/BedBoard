export function normalizeRole(role) {
  const key = String(role || '').toLowerCase().trim();
  if (key === 'admin') return 'admin';
  if (key === 'dechocage') return 'dechocage';
  if (key === 'reception') return 'reception';
  if (key === 'triage') return 'triage';
  return 'user';
}
