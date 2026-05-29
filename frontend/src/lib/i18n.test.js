import { describe, expect, it } from 'vitest';
import { isRtlLocale, normalizeLocale, patientTypeLabel, roleLabel, tr, triageLabel } from './i18n';
import { normalizeRole } from './roles';
import { triageLevelOf } from './triage';

describe('i18n and role helpers', () => {
  it('normalizes locales and translations', () => {
    expect(normalizeLocale('EN')).toBe('en');
    expect(normalizeLocale('ar')).toBe('ar');
    expect(normalizeLocale('unknown')).toBe('fr');
    expect(isRtlLocale('ar')).toBe(true);
    expect(tr('en', 'FR', 'EN', 'AR')).toBe('EN');
    expect(tr('ar', 'FR', 'EN', 'AR')).toBe('AR');
  });

  it('maps labels and triage level safely', () => {
    expect(patientTypeLabel('en', 'medical')).toBe('Medical');
    expect(roleLabel('en', 'dechocage')).toBe('Resuscitation');
    expect(triageLabel('en', 4)).toBe('Level 4');
    expect(normalizeRole(' TRIAGE ')).toBe('triage');
    expect(triageLevelOf({ triageScore: 8 })).toBe(4);
    expect(triageLevelOf({ triageScore: -5 })).toBe(0);
  });
});
