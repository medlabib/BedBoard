import { triageLevelOf, triageMeta } from '../lib/triage';
import { patientTypeLabel, triageLabel, tr } from '../lib/i18n';

export default function BedsGrid({
  beds,
  statusMeta,
  normalizeStatus,
  escapeText,
  canManageBeds,
  isAdmin,
  assignByBed,
  setAssignByBed,
  activePatients,
  assignablePatients,
  bedEdits,
  setBedEdits,
  api,
  showError,
  showSuccess,
  readErrorMessage,
  setConfirm,
  locale,
}) {
  if (!beds.length) {
    return <div className="empty">{tr(locale, 'Aucun lit disponible.', 'No beds available.', 'لا توجد أسرة متاحة.')}</div>;
  }

  return beds.map((bed) => {
    const statusKey = normalizeStatus(bed.status);
    const meta = statusMeta[statusKey] || statusMeta.libre;
    const hasPatientInfo = Boolean(bed.patientName || bed.patientRegistration || bed.hasPatient);
    const assignedPatient = activePatients.find((patient) => Number(patient.bedNumber) === Number(bed.number))
      || activePatients.find((patient) => patient.registrationNumber === bed.patientRegistration);
    const assignedType = patientTypeLabel(assignedPatient?.patientType);
    const triageLevel = triageLevelOf(assignedPatient);
    const triage = triageMeta[triageLevel] || triageMeta[0];
    const patientLine = hasPatientInfo
      ? `${escapeText(bed.patientName || tr(locale, 'Patient affecte', 'Assigned patient', 'مريض مخصص'))}${bed.patientRegistration ? ` (${escapeText(bed.patientRegistration)})` : ''}${assignedType ? ` - ${assignedType}` : ''}`
      : (statusKey === 'occupé' ? tr(locale, 'Patient affecte', 'Assigned patient', 'مريض مخصص') : tr(locale, 'Aucun patient affecte', 'No patient assigned', 'لا يوجد مريض مخصص'));

    return (
      <article
        key={bed.number}
        className={`card status-${statusKey}`}
        style={{ '--status-color': meta.color, '--status-soft': meta.soft }}
      >
        <div className="card-head">
          <div>
            <h3 className="card-title">{escapeText(bed.room || tr(locale, 'Chambre', 'Room', 'غرفة'))} - {escapeText(bed.name || `${tr(locale, 'Lit', 'Bed', 'سرير')} ${bed.number}`)}</h3>
            <div className="meta">
              <span>{tr(locale, 'Type', 'Type', 'النوع')}: {escapeText(bed.type)}</span>
              <span>{tr(locale, 'Heure', 'Time', 'الوقت')}: {escapeText(bed.time || '-')}</span>
            </div>
          </div>
          <div className="badge"><span className="mini-dot" />{meta.label}</div>
        </div>
        <div className="info-box">
          <strong>{tr(locale, 'Patient', 'Patient', 'المريض')}</strong>
          <span>{patientLine}</span>
          {assignedPatient ? <span className="triage-pill" style={{ background: triage.color }}>{tr(locale, 'Triage', 'Triage', 'الفرز')} {triageLabel(locale, triageLevel)}</span> : null}
        </div>
        <div className="action-row">
          <div className="status-actions">
            {['libre', 'occupé', 'nettoyage', 'alerte'].map((status) => (
              <button
                key={status}
                className="status-btn"
                data-status={status}
                disabled={!canManageBeds}
                onClick={async () => {
                  if (!canManageBeds) return;
                  const response = await api('/api/status', {
                    method: 'POST',
                    body: JSON.stringify({ number: bed.number, status }),
                  });
                  if (!response.ok) {
                    showError(await readErrorMessage(response, tr(locale, 'Mise a jour etat lit impossible.', 'Unable to update bed status.', 'تعذر تحديث حالة السرير.')));
                    return;
                  }
                  showSuccess(`${tr(locale, 'Lit', 'Bed', 'سرير')} ${bed.number} ${tr(locale, 'mis a jour', 'updated', 'تم تحديثه')}: ${statusMeta[status].label}.`);
                }}
              >
                {statusMeta[status].label}
              </button>
            ))}
          </div>
          {isAdmin ? (
            <button
              className="mini-btn"
              type="button"
              onClick={async () => {
                setConfirm({ open: true, title: tr(locale, 'Supprimer le lit', 'Delete bed', 'حذف السرير'), message: `${tr(locale, 'Supprimer le lit', 'Delete bed', 'حذف السرير')} ${bed.number} ?`, onConfirm: async () => {
                  const response = await api('/api/beds/delete', {
                    method: 'POST',
                    body: JSON.stringify({ number: bed.number }),
                  });
                  if (!response.ok) {
                    showError(await readErrorMessage(response, tr(locale, 'Suppression du lit impossible.', 'Unable to delete bed.', 'تعذر حذف السرير.')));
                    return;
                  }
                  showSuccess(`${tr(locale, 'Lit', 'Bed', 'سرير')} ${bed.number} ${tr(locale, 'supprime.', 'deleted.', 'تم حذفه.')}`);
                } });
              }}
            >
              {tr(locale, 'Supprimer', 'Delete', 'حذف')}
            </button>
          ) : null}
          {canManageBeds ? (
            <div className="assign-box">
              <select
                className="form-select"
                value={assignByBed[bed.number] || ''}
                onChange={(e) => setAssignByBed((current) => ({ ...current, [bed.number]: e.target.value }))}
              >
                <option value="">{tr(locale, 'Affecter un patient', 'Assign a patient', 'تخصيص مريض')}</option>
                {(Array.isArray(assignablePatients) ? assignablePatients : activePatients).filter((p) => !p.bedNumber).map((p) => (
                  <option key={p.registrationNumber} value={p.registrationNumber}>
                    {p.registrationNumber}{patientTypeLabel(p.patientType) ? ` - ${patientTypeLabel(p.patientType)}` : ''}
                  </option>
                ))}
              </select>
              <button className="mini-btn" type="button" onClick={async () => {
                const reg = assignByBed[bed.number];
                if (!reg) {
                  showError(tr(locale, 'Selectionnez un patient avant affectation.', 'Select a patient before assigning.', 'اختر مريضًا قبل التخصيص.'));
                  return;
                }
                const selectedPatient = activePatients.find((p) => p.registrationNumber === reg);
                const response = await api('/api/patients', {
                  method: 'POST',
                  body: JSON.stringify({ registrationNumber: reg, name: selectedPatient?.name || '', bedNumber: bed.number }),
                });
                if (!response.ok) {
                  showError(await readErrorMessage(response, tr(locale, 'Affectation patient impossible.', 'Unable to assign patient.', 'تعذر تخصيص المريض.')));
                  return;
                }
                setAssignByBed((current) => ({ ...current, [bed.number]: '' }));
                showSuccess(`${tr(locale, 'Patient', 'Patient', 'المريض')} ${reg} ${tr(locale, 'affecte au lit', 'assigned to bed', 'تم تخصيصه للسرير')} ${bed.number}.`);
              }}>{tr(locale, 'Affecter', 'Assign', 'تخصيص')}</button>
            </div>
          ) : null}
          {canManageBeds ? (
            <div className="form-grid compact">
              {(() => {
                const draft = bedEdits[bed.number] || {
                  room: bed.room || '',
                  name: bed.name || '',
                  type: bed.type,
                };
                return (
                  <>
                    <label>
                      {tr(locale, 'Chambre', 'Room', 'غرفة')}
                      <input
                        className="form-control"
                        value={draft.room}
                        onChange={(event) => {
                          const value = event.target.value;
                          setBedEdits((current) => ({
                            ...current,
                            [bed.number]: { ...(current[bed.number] || draft), room: value },
                          }));
                        }}
                      />
                    </label>
                    <label>
                      {tr(locale, 'Nom', 'Name', 'الاسم')}
                      <input
                        className="form-control"
                        value={draft.name}
                        onChange={(event) => {
                          const nextName = event.target.value;
                          setBedEdits((current) => ({ ...current, [bed.number]: { ...(current[bed.number] || draft), name: nextName } }));
                        }}
                      />
                    </label>
                    <label>
                      {tr(locale, 'Type', 'Type', 'النوع')}
                      <select
                        className="form-select"
                        value={draft.type}
                        onChange={(event) => {
                          const nextType = event.target.value;
                          setBedEdits((current) => ({ ...current, [bed.number]: { ...(current[bed.number] || { name: bed.name, type: bed.type }), type: nextType } }));
                        }}
                      >
                        <option value="standard">Standard</option>
                        <option value="thoracique">Thoracique</option>
                      </select>
                    </label>
                    <button
                      className="mini-btn"
                      type="button"
                      onClick={async () => {
                        const next = bedEdits[bed.number] || draft;
                        const response = await api('/api/config-bed', {
                          method: 'POST',
                          body: JSON.stringify({
                            number: bed.number,
                            room: next.room,
                            name: next.name,
                            type: next.type,
                          }),
                        });
                        if (!response.ok) {
                          showError(await readErrorMessage(response, tr(locale, 'Configuration lit impossible.', 'Unable to update bed configuration.', 'تعذر تحديث إعدادات السرير.')));
                          return;
                        }
                        setBedEdits((current) => {
                          const copy = { ...current };
                          delete copy[bed.number];
                          return copy;
                        });
                        showSuccess(`${tr(locale, 'Configuration du lit', 'Bed configuration', 'إعدادات السرير')} ${bed.number} ${tr(locale, 'enregistree.', 'saved.', 'تم حفظها.')}`);
                      }}
                    >
                      {tr(locale, 'Enregistrer', 'Save', 'حفظ')}
                    </button>
                  </>
                );
              })()}
            </div>
          ) : null}
        </div>
      </article>
    );
  });
}
