import { triageLevelOf, triageMeta } from '../lib/triage';

export default function PatientsRows({
  activePatients,
  canViewTriage,
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
  speakPatientCall,
  escapeText,
}) {
  if (!activePatients.length) {
    return (
      <tr>
        <td colSpan="5"><div className="empty">Aucun patient enregistre.</div></td>
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
        <td>
          {canViewTriage ? <span className="triage-pill" style={{ background: triage.color }}>{triage.label}</span> : '-'}
        </td>
        <td>{patient.bedNumber ? `${escapeText(patient.roomName || 'Chambre')} - ${escapeText(patient.bedName || `Lit ${patient.bedNumber}`)}` : 'Non assigne'}</td>
        <td>
          {authenticated ? (
            <div className="patient-actions">
              {canManageBeds ? <button className="mini-btn" type="button" onClick={() => {
                setScreen('patients');
                setNewPatient((current) => ({ ...current, registrationNumber: patient.registrationNumber }));
              }}>Reassigner</button> : null}
              <button className="mini-btn action-secondary" type="button" onClick={() => speakPatientCall(patient)}>Appel vocal</button>
              {canArchivePatients ? <button className="mini-btn action-critical" type="button" onClick={() => {
                setConfirm({ open: true, title: 'Archiver le patient', message: `Archiver le patient ${patient.registrationNumber} ?`, onConfirm: async () => {
                  const response = await api('/api/patients/archive', { method: 'POST', body: JSON.stringify({ registrationNumber: patient.registrationNumber, action: 'archive' }) });
                  if (!response.ok) {
                    showError(await readErrorMessage(response, 'Archivage patient impossible.'));
                    return;
                  }
                  setPatients((current) => current.map((item) => item.registrationNumber === patient.registrationNumber
                    ? { ...item, status: 'archived', bedNumber: null, bedName: '' }
                    : item));
                  showSuccess(`Patient ${patient.registrationNumber} archive.`);
                } });
              }}>Archiver</button> : null}
            </div>
          ) : 'Lecture seule'}
        </td>
      </tr>
    );
  });
}
