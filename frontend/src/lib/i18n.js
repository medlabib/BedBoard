export const supportedLocales = ['fr', 'en', 'ar'];

export function normalizeLocale(value) {
  const key = String(value || '').trim().toLowerCase();
  if (key === 'en') return 'en';
  if (key === 'ar') return 'ar';
  return 'fr';
}

export function isRtlLocale(value) {
  return normalizeLocale(value) === 'ar';
}

export function tr(locale, fr, en, ar) {
  const key = normalizeLocale(locale);
  if (key === 'en') return en;
  if (key === 'ar') return ar;
  return fr;
}

export function patientTypeLabel(locale, value) {
  const key = String(value || '').trim().toLowerCase();
  switch (key) {
    case 'traumato':
      return tr(locale, 'Traumato', 'Trauma', 'رضوض');
    case 'medical':
      return tr(locale, 'Medical', 'Medical', 'طبي');
    case 'douleurs_thoracique':
      return tr(locale, 'Douleurs thoracique', 'Chest pain', 'ألم صدري');
    case 'chirurgical':
      return tr(locale, 'Chirurgical', 'Surgical', 'جراحي');
    default:
      return '-';
  }
}

export function roleLabel(locale, value) {
  const key = String(value || '').trim().toLowerCase();
  switch (key) {
    case 'admin':
      return tr(locale, 'Admin', 'Admin', 'مدير');
    case 'user':
      return tr(locale, 'Utilisateur', 'User', 'مستخدم');
    case 'triage':
      return tr(locale, 'Triage', 'Triage', 'فرز');
    case 'reception':
      return tr(locale, 'Reception', 'Reception', 'استقبال');
    case 'dechocage':
      return tr(locale, 'Dechocage', 'Resuscitation', 'إنعاش');
    default:
      return key || '-';
  }
}

export function triageLabel(locale, level) {
  return tr(locale, `Niveau ${level}`, `Level ${level}`, `المستوى ${level}`);
}
