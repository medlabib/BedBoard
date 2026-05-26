export const callLanguageOptions = [
  { value: 'fr-FR', label: 'Français' },
  { value: 'en-US', label: 'English' },
  { value: 'es-ES', label: 'Español' },
  { value: 'ar-MA', label: 'العربية' },
];

export function getCallTemplate(lang, patientName, roomName, bedName) {
  const room = roomName || 'Chambre';
  const bed = bedName || 'Lit';
  if (lang === 'en-US') return `Patient ${patientName}, ${room}, ${bed}, please.`;
  if (lang === 'es-ES') return `Paciente ${patientName}, ${room}, ${bed}, por favor.`;
  if (lang === 'ar-MA') return `المريض ${patientName}، ${room}، ${bed}، من فضلكم.`;
  return `Patient ${patientName}, ${room}, ${bed}, s'il vous plaît.`;
}
