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
    case 'urgences_differees':
      return tr(locale, 'Urgences differees', 'Deferred emergencies', 'حالات طارئة مؤجلة');
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

export function patientStatusLabel(locale, value) {
  const key = String(value || '').trim().toLowerCase();
  switch (key) {
    case 'arrived':
      return tr(locale, 'Arrive', 'Arrived', 'وصل');
    case 'triaged':
      return tr(locale, 'Trie', 'Triaged', 'تم الفرز');
    case 'waiting':
      return tr(locale, 'En attente', 'Waiting', 'قيد الانتظار');
    case 'assigned':
      return tr(locale, 'Assigne', 'Assigned', 'مخصص');
    case 'in_exam':
      return tr(locale, 'En examen', 'In exam', 'قيد الفحص');
    case 'imaging':
      return tr(locale, 'Imagerie', 'Imaging', 'تصوير');
    case 'waiting_results':
      return tr(locale, 'Attente resultats', 'Waiting results', 'بانتظار النتائج');
    case 'discharge_ready':
      return tr(locale, 'Pret sortie', 'Discharge ready', 'جاهز للخروج');
    case 'consulted':
      return tr(locale, 'Consulte', 'Consulted', 'تمت المعاينة');
    case 'transferred':
      return tr(locale, 'Transfere', 'Transferred', 'تم التحويل');
    case 'deceased':
      return tr(locale, 'Decede', 'Deceased', 'متوفى');
    case 'archived':
      return tr(locale, 'Archive', 'Archived', 'مؤرشف');
    default:
      return key || '-';
  }
}
