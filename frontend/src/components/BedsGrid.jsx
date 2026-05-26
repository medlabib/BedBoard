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
  bedEdits,
  setBedEdits,
  api,
  showError,
  showSuccess,
  readErrorMessage,
  setConfirm,
}) {
  if (!beds.length) {
    return <div className="empty">Aucun lit disponible.</div>;
  }

  return beds.map((bed) => {
    const statusKey = normalizeStatus(bed.status);
    const meta = statusMeta[statusKey] || statusMeta.libre;
    const hasPatientInfo = Boolean(bed.patientName || bed.patientRegistration || bed.hasPatient);
    const patientLine = hasPatientInfo
      ? `${escapeText(bed.patientName || 'Patient affecte')}${bed.patientRegistration ? ` (${escapeText(bed.patientRegistration)})` : ''}`
      : (statusKey === 'occupé' ? 'Patient affecte' : 'Aucun patient affecte');

    return (
      <article
        key={bed.number}
        className={`card status-${statusKey}`}
        style={{ '--status-color': meta.color, '--status-soft': meta.soft }}
      >
        <div className="card-head">
          <div>
            <h3 className="card-title">{escapeText(bed.room || 'Chambre')} - {escapeText(bed.name || `Lit ${bed.number}`)}</h3>
            <div className="meta">
              <span>Type: {escapeText(bed.type)}</span>
              <span>Heure: {escapeText(bed.time || '-')}</span>
            </div>
          </div>
          <div className="badge"><span className="mini-dot" />{meta.label}</div>
        </div>
        <div className="info-box">
          <strong>Patient</strong>
          <span>{patientLine}</span>
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
                    showError(await readErrorMessage(response, 'Mise a jour etat lit impossible.'));
                    return;
                  }
                  showSuccess(`Lit ${bed.number} mis a jour: ${statusMeta[status].label}.`);
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
                setConfirm({ open: true, title: 'Supprimer le lit', message: `Supprimer le lit ${bed.number} ?`, onConfirm: async () => {
                  const response = await api('/api/beds/delete', {
                    method: 'POST',
                    body: JSON.stringify({ number: bed.number }),
                  });
                  if (!response.ok) {
                    showError(await readErrorMessage(response, 'Suppression du lit impossible.'));
                    return;
                  }
                  showSuccess(`Lit ${bed.number} supprime.`);
                } });
              }}
            >
              Supprimer
            </button>
          ) : null}
          {canManageBeds ? (
            <div className="assign-box">
              <select
                className="form-select"
                value={assignByBed[bed.number] || ''}
                onChange={(e) => setAssignByBed((current) => ({ ...current, [bed.number]: e.target.value }))}
              >
                <option value="">Affecter un patient</option>
                {activePatients.filter((p) => !p.bedNumber).map((p) => (
                  <option key={p.registrationNumber} value={p.registrationNumber}>{p.registrationNumber}</option>
                ))}
              </select>
              <button className="mini-btn" type="button" onClick={async () => {
                const reg = assignByBed[bed.number];
                if (!reg) {
                  showError('Selectionnez un patient avant affectation.');
                  return;
                }
                const selectedPatient = activePatients.find((p) => p.registrationNumber === reg);
                const response = await api('/api/patients', {
                  method: 'POST',
                  body: JSON.stringify({ registrationNumber: reg, name: selectedPatient?.name || '', bedNumber: bed.number }),
                });
                if (!response.ok) {
                  showError(await readErrorMessage(response, 'Affectation patient impossible.'));
                  return;
                }
                setAssignByBed((current) => ({ ...current, [bed.number]: '' }));
                showSuccess(`Patient ${reg} affecte au lit ${bed.number}.`);
              }}>Affecter</button>
            </div>
          ) : null}
          {canManageBeds ? (
            <div className="form-grid compact">
              {(() => {
                const draft = bedEdits[bed.number] || {
                  room: bed.room || '',
                  roomAlt: bed.roomAlt || '',
                  name: bed.name || '',
                  nameAlt: bed.nameAlt || '',
                  type: bed.type,
                };
                return (
                  <>
                    <label>
                      Chambre
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
                      Chambre (autre langue)
                      <input
                        className="form-control"
                        value={draft.roomAlt}
                        onChange={(event) => {
                          const value = event.target.value;
                          setBedEdits((current) => ({
                            ...current,
                            [bed.number]: { ...(current[bed.number] || draft), roomAlt: value },
                          }));
                        }}
                      />
                    </label>
                    <label>
                      Nom
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
                      Lit (autre langue)
                      <input
                        className="form-control"
                        value={draft.nameAlt}
                        onChange={(event) => {
                          const value = event.target.value;
                          setBedEdits((current) => ({ ...current, [bed.number]: { ...(current[bed.number] || draft), nameAlt: value } }));
                        }}
                      />
                    </label>
                    <label>
                      Type
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
                            roomAlt: next.roomAlt,
                            name: next.name,
                            nameAlt: next.nameAlt,
                            type: next.type,
                          }),
                        });
                        if (!response.ok) {
                          showError(await readErrorMessage(response, 'Configuration lit impossible.'));
                          return;
                        }
                        setBedEdits((current) => {
                          const copy = { ...current };
                          delete copy[bed.number];
                          return copy;
                        });
                        showSuccess(`Configuration du lit ${bed.number} enregistree.`);
                      }}
                    >
                      Enregistrer
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
