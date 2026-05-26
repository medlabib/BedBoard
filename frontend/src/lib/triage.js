export const triageMeta = {
  0: { label: 'Niveau 0', color: '#8bc34a' },
  1: { label: 'Niveau 1', color: '#4caf50' },
  2: { label: 'Niveau 2', color: '#ffb74d' },
  3: { label: 'Niveau 3', color: '#ff7043' },
  4: { label: 'Niveau 4', color: '#e53935' },
};

export function triageLevelOf(patient) {
  const value = Number(patient?.triageScore ?? 0);
  if (Number.isNaN(value)) return 0;
  return Math.max(0, Math.min(4, value));
}
