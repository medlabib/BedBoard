import { triageLevelOf, triageMeta } from '../lib/triage';
import { patientTypeLabel, triageLabel, tr } from '../lib/i18n';

export default function PatientsRows({
  activePatients,
  canViewTriage,
  canViewPatientType,
  authenticated,
  canManageBeds,
  canArchivePatients,
  setScreen,
  setNewPatient,
  setConfirm,
  api,
  showError,
  showSuccess,
  readErrorMessage,
  setPatients,
  escapeText,
  locale,
}) {
  if (!activePatients.length) {
    return (
      <tr>
        <td colSpan={canViewPatientType ? 6 : 5}><div className="empty">{tr(locale, 'Aucun patient enregistre.', 'No patient registered.', 'لا يوجد مرضى مسجلون.')}</div></td>
      </tr>
    );
  }

  return activePatients.map((patient) => {
    const level = triageLevelOf(patient);
    const triage = triageMeta[level] || triageMeta[0];
    return (
      <tr key={patient.registrationNumber} className={`triage-row triage-${level}`}>
        <td>{escapeText(patient.registrationNumber)}</td>
        <td>{escapeText(patient.name)}</td>
        {canViewPatientType ? <td>{patientTypeLabel(locale, patient.patientType)}</td> : null}
        <td>
          {canViewTriage ? <span className="triage-pill" style={{ background: triage.color }}>{triageLabel(locale, level)}</span> : '-'}
        </td>
        <td>{patient.bedNumber ? `${escapeText(patient.roomName || tr(locale, 'Chambre', 'Room', 'غرفة'))} - ${escapeText(patient.bedName || `${tr(locale, 'Lit', 'Bed', 'سرير')} ${patient.bedNumber}`)}` : tr(locale, 'Non assigne', 'Unassigned', 'غير مخصص')}</td>
        <td>
          {authenticated ? (
            <div className="patient-actions">
              {canManageBeds ? <button className="mini-btn" type="button" onClick={() => {
                setScreen('patients');
                setNewPatient((current) => ({ ...current, registrationNumber: patient.registrationNumber }));
              }}>{tr(locale, 'Reassigner', 'Reassign', 'إعادة التخصيص')}</button> : null}
              {canArchivePatients ? <button className="mini-btn action-critical" type="button" onClick={() => {
                setConfirm({ open: true, title: tr(locale, 'Archiver le patient', 'Archive patient', 'أرشفة المريض'), message: `${tr(locale, 'Archiver le patient', 'Archive patient', 'أرشفة المريض')} ${patient.registrationNumber} ?`, onConfirm: async () => {
                  const response = await api('/api/patients/archive', { method: 'POST', body: JSON.stringify({ registrationNumber: patient.registrationNumber, action: 'archive' }) });
                  if (!response.ok) {
                    showError(await readErrorMessage(response, tr(locale, 'Archivage patient impossible.', 'Unable to archive patient.', 'تعذر أرشفة المريض.')));
                    return;
                  }
                  setPatients((current) => current.map((item) => item.registrationNumber === patient.registrationNumber
                    ? { ...item, status: 'archived', bedNumber: null, bedName: '' }
                    : item));
                  showSuccess(`${tr(locale, 'Patient', 'Patient', 'المريض')} ${patient.registrationNumber} ${tr(locale, 'archive.', 'archived.', 'تمت أرشفته.')}`);
                } });
              }}>{tr(locale, 'Archiver', 'Archive', 'أرشفة')}</button> : null}
            </div>
          ) : tr(locale, 'Lecture seule', 'Read only', 'قراءة فقط')}
        </td>
      </tr>
    );
  });
}
