import { useEffect, useMemo, useState } from 'react';

const initialStats = {
  totalBeds: 0,
  freeBeds: 0,
  occupiedBeds: 0,
  cleaningBeds: 0,
  alertBeds: 0,
  totalPatients: 0,
  archivedPatients: 0,
  consultationsByDate: [],
  avgConsultationMinutes: 0,
  totalConsultations: 0,
};

const statusMeta = {
  libre: { label: 'Libre', color: '#7ab893', soft: 'rgba(122, 184, 147, 0.16)' },
  occupé: { label: 'Occupé', color: '#7fa7d4', soft: 'rgba(127, 167, 212, 0.16)' },
  nettoyage: { label: 'Nettoyage', color: '#a58ac9', soft: 'rgba(165, 138, 201, 0.18)' },
  alerte: { label: 'Alerte', color: '#d97a70', soft: 'rgba(217, 122, 112, 0.18)' },
};

function normalizeStatus(value) {
  const key = String(value || 'libre').toLowerCase().trim();
  if (key === 'occupied' || key === 'occupe' || key === 'occupé') return 'occupé';
  if (key === 'cleaning') return 'nettoyage';
  if (key === 'alert') return 'alerte';
  if (key === 'free') return 'libre';
  return key || 'libre';
}

function escapeText(value) {
  return String(value ?? '');
}

function App() {
  const [beds, setBeds] = useState([]);
  const [patients, setPatients] = useState([]);
  const [stats, setStats] = useState(initialStats);
  const [authenticated, setAuthenticated] = useState(false);
  const [user, setUser] = useState({ username: '', admin: false });
  const [users, setUsers] = useState([]);
  const [screen, setScreen] = useState('beds');
  const [modalOpen, setModalOpen] = useState(false);
  const [authForm, setAuthForm] = useState({ username: 'admin', password: '' });
  const [authMessage, setAuthMessage] = useState('Connectez-vous pour gérer les lits et les patients sur ce poste.');
  const [connectionState, setConnectionState] = useState('Liaison réseau en attente.');
  const [newBed, setNewBed] = useState({ number: '', name: '', type: 'standard' });
  const [newPatient, setNewPatient] = useState({ registrationNumber: '', name: '', bedNumber: '' });
  const [newUser, setNewUser] = useState({ username: '', password: '' });

  const isAdmin = user.admin;

  const api = async (url, options = {}) => {
    const response = await fetch(url, {
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
      ...options,
    });
    return response;
  };

  const refreshUsers = async () => {
    if (!isAdmin) {
      setUsers([]);
      return;
    }
    const response = await api('/api/users');
    const data = await response.json().catch(() => ({ users: [] }));
    setUsers(Array.isArray(data.users) ? data.users : []);
  };

  const syncMe = async () => {
    const response = await fetch('/api/me', { credentials: 'include' });
    const data = await response.json().catch(() => ({ authenticated: false, username: '', admin: false }));
    setAuthenticated(Boolean(data.authenticated));
    setUser({ username: data.username || '', admin: Boolean(data.admin) });
    if (data.authenticated) {
      setModalOpen(false);
    }
    if (data.authenticated && data.username) {
      setConnectionState(`Connecté: ${data.username}`);
    } else {
      setConnectionState('Accès local');
    }
    if (data.authenticated && data.admin) {
      await refreshUsers();
    }
  };

  const connectStream = () => {
    const stream = new EventSource('/api/stream');
    setConnectionState('Connexion locale...');

    stream.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        setBeds(Array.isArray(data.beds) ? data.beds : []);
        setPatients(Array.isArray(data.patients) ? data.patients : []);
        setStats(data.stats || initialStats);
        setConnectionState('Connecté.');
      } catch (error) {
        console.error(error);
      }
    };

    stream.onerror = () => {
      setConnectionState('Connexion interrompue.');
    };

    return stream;
  };

  useEffect(() => {
    let stream;
    syncMe().finally(() => {
      stream = connectStream();
    });
    return () => {
      if (stream) {
        stream.close();
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const renderBeds = useMemo(() => {
    if (!beds.length) {
      return <div className="empty">Aucun lit disponible.</div>;
    }

    return beds.map((bed) => {
      const statusKey = normalizeStatus(bed.status);
      const meta = statusMeta[statusKey] || statusMeta.libre;
      const hasPatientInfo = Boolean(bed.patientName || bed.patientRegistration || bed.hasPatient);
      const patientLine = hasPatientInfo
        ? `${escapeText(bed.patientName || 'Patient affecté')}${bed.patientRegistration ? ` (${escapeText(bed.patientRegistration)})` : ''}`
        : (statusKey === 'occupé' ? 'Patient affecté' : 'Aucun patient affecté');

      return (
        <article
          key={bed.number}
          className={`card status-${statusKey}`}
          style={{ '--status-color': meta.color, '--status-soft': meta.soft }}
        >
          <div className="card-head">
            <div>
              <h3 className="card-title">Lit {bed.number} - {escapeText(bed.name)}</h3>
              <div className="meta">
                <span>Type: {escapeText(bed.type)}</span>
                <span>Heure: {escapeText(bed.time || '—')}</span>
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
                  disabled={!authenticated}
                  onClick={async () => {
                    if (!authenticated) return;
                    await api('/api/status', {
                      method: 'POST',
                      body: JSON.stringify({ number: bed.number, status }),
                    });
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
                  await api('/api/beds/delete', {
                    method: 'POST',
                    body: JSON.stringify({ number: bed.number }),
                  });
                }}
              >
                Supprimer
              </button>
            ) : null}
            {authenticated ? (
              <div className="assign-box">
                <select defaultValue="" onChange={(e) => e.target.setAttribute('data-selected', e.target.value)}>
                  <option value="">Affecter un patient</option>
                  {patients.filter(p => !p.bedNumber).map(p => (
                    <option key={p.registrationNumber} value={p.registrationNumber}>{p.registrationNumber}</option>
                  ))}
                </select>
                <button className="mini-btn" type="button" onClick={async (e) => {
                  const select = e.currentTarget.previousElementSibling;
                  const reg = select && select.getAttribute('data-selected');
                  if (!reg) return;
                  await api('/api/patients', {
                    method: 'POST',
                    body: JSON.stringify({ registrationNumber: reg, name: '', bedNumber: bed.number }),
                  });
                }}>Affecter</button>
              </div>
            ) : null}
            {authenticated ? (
              <div className="form-grid compact">
                <label>
                  Nom
                  <input
                    value={bed.name}
                    onChange={async (event) => {
                      const nextName = event.target.value;
                      await api('/api/config-bed', {
                        method: 'POST',
                        body: JSON.stringify({ number: bed.number, name: nextName, type: bed.type }),
                      });
                    }}
                  />
                </label>
                <label>
                  Type
                  <select
                    value={bed.type}
                    onChange={async (event) => {
                      await api('/api/config-bed', {
                        method: 'POST',
                        body: JSON.stringify({ number: bed.number, name: bed.name, type: event.target.value }),
                      });
                    }}
                  >
                    <option value="standard">Standard</option>
                    <option value="thoracique">Thoracique</option>
                  </select>
                </label>
              </div>
            ) : null}
          </div>
        </article>
      );
    });
  }, [beds, authenticated, isAdmin]);

  const renderPatients = useMemo(() => {
    if (!patients.length) {
      return (
        <tr>
          <td colSpan="4"><div className="empty">Aucun patient enregistré.</div></td>
        </tr>
      );
    }

    return patients.map((patient) => (
      <tr key={patient.registrationNumber}>
        <td>{escapeText(patient.registrationNumber)}</td>
        <td>{escapeText(patient.name)}</td>
        <td>{patient.bedNumber ? `Lit ${patient.bedNumber}` : 'Non assigné'}</td>
        <td>
          {authenticated ? (
            <>
              <button className="mini-btn" type="button" onClick={() => {
                setScreen('patients');
                setNewPatient((current) => ({ ...current, registrationNumber: patient.registrationNumber }));
              }}>Réassigner</button>
              <button className="mini-btn" type="button" onClick={async () => {
                await api('/api/patients/archive', { method: 'POST', body: JSON.stringify({ registrationNumber: patient.registrationNumber, action: 'archive' }) });
              }}>Archiver</button>
            </>
          ) : 'Lecture seule'}
        </td>
      </tr>
    ));
  }, [patients, authenticated]);

  const renderUsers = useMemo(() => {
    if (!isAdmin) {
      return (
        <tr>
          <td colSpan="2"><div className="empty">Réservé à l’admin.</div></td>
        </tr>
      );
    }
    if (!users.length) {
      return (
        <tr>
          <td colSpan="2"><div className="empty">Aucun utilisateur.</div></td>
        </tr>
      );
    }
    return users.map((item) => (
      <tr key={item.username}>
        <td>{escapeText(item.username)}</td>
        <td>{item.admin ? 'Admin' : 'Utilisateur'}</td>
      </tr>
    ));
  }, [users, isAdmin]);

  const createBed = async () => {
    const response = await api('/api/beds', {
      method: 'POST',
      body: JSON.stringify({
        number: Number(newBed.number),
        name: newBed.name,
        type: newBed.type,
      }),
    });
    if (response.ok) {
      setNewBed({ number: '', name: '', type: 'standard' });
    }
  };

  const savePatient = async () => {
    const response = await api('/api/patients', {
      method: 'POST',
      body: JSON.stringify({
        registrationNumber: newPatient.registrationNumber,
        name: newPatient.name,
        bedNumber: Number(newPatient.bedNumber),
      }),
    });
    if (response.ok) {
      setNewPatient({ registrationNumber: '', name: '', bedNumber: '' });
    }
  };

  const authenticate = async () => {
    const response = await fetch('/api/auth', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(authForm),
    });
    if (response.ok) {
      const data = await response.json().catch(() => ({}));
      setAuthenticated(true);
      setUser({ username: data.username || authForm.username || 'admin', admin: Boolean(data.admin) });
      setModalOpen(false);
      setConnectionState(data.username ? `Connecté: ${data.username}` : 'Connecté.');
      if (data.admin) {
        await refreshUsers();
      }
      return;
    }
    setAuthMessage('Identifiants invalides.');
  };

  const logout = async () => {
    await fetch('/api/logout', { method: 'POST', credentials: 'include' });
    setAuthenticated(false);
    setUser({ username: '', admin: false });
    setUsers([]);
    setConnectionState('Accès local');
  };

  const createUser = async () => {
    const response = await api('/api/users', {
      method: 'POST',
      body: JSON.stringify(newUser),
    });
    if (response.ok) {
      setNewUser({ username: '', password: '' });
      await refreshUsers();
      return;
    }
    setConnectionState('Création utilisateur impossible.');
  };

  const openSettings = async () => {
    setScreen('settings');
    await refreshUsers();
  };

  return (
    <div className="app">
      <div className="navbar">
        <div className="nav-left">
          <div className="brand-mark"><img src="/logo.png" alt="Logo de l'hôpital" /></div>
          <div className="nav-title">
            <strong>BedBoard</strong>
            <span>{connectionState}</span>
          </div>
        </div>
        <div className="nav-actions">
          {authenticated && user.username ? <div className="user-chip">{user.username}</div> : null}
          {authenticated && isAdmin ? <button className="btn" type="button" onClick={openSettings}>Paramètres</button> : null}
          {!authenticated ? <button className="btn" type="button" onClick={() => setModalOpen(true)}>Se connecter</button> : null}
          {authenticated ? <button className="btn primary" type="button" onClick={logout}>Déconnexion</button> : null}
        </div>
      </div>

      <div className="shell">
        <div className="hero">
          <div className="brand-card">
            <p className="eyebrow">BedBoard</p>
            <h1>Gestion des lits et des patients</h1>
            
            <div className="status-row">
              <span className="pill"><span className="dot" style={{ background: 'var(--green)' }} /> Libre</span>
              <span className="pill"><span className="dot" style={{ background: 'var(--blue)' }} /> Occupé</span>
              <span className="pill"><span className="dot" style={{ background: 'var(--violet)' }} /> Nettoyage</span>
              <span className="pill"><span className="dot" style={{ background: 'var(--red)' }} /> Alerte</span>
            </div>
          </div>
          <div className="side-card">
            <div className="stats-grid">
              <div className="stat"><span>Total lits</span><strong>{stats.totalBeds || 0}</strong></div>
              <div className="stat"><span>Occupés</span><strong>{stats.occupiedBeds || 0}</strong></div>
              <div className="stat"><span>Libres</span><strong>{stats.freeBeds || 0}</strong></div>
              <div className="stat"><span>Patients</span><strong>{stats.totalPatients || 0}</strong></div>
            </div>
          </div>
        </div>

        <div className="section-card">
          <div className="tab-strip">
            <button className={`tab-btn ${screen === 'beds' ? 'active' : ''}`} type="button" onClick={() => setScreen('beds')}>Lits</button>
            <button className={`tab-btn ${screen === 'patients' ? 'active' : ''}`} type="button" onClick={() => setScreen('patients')}>Patients</button>
            <button className={`tab-btn ${screen === 'patientview' ? 'active' : ''}`} type="button" onClick={() => setScreen('patientview')}>Vue patient</button>
            {authenticated ? <button className={`tab-btn ${screen === 'stats' ? 'active' : ''}`} type="button" onClick={() => setScreen('stats')}>Statistiques</button> : null}
            {authenticated && isAdmin ? <button className={`tab-btn ${screen === 'settings' ? 'active' : ''}`} type="button" onClick={openSettings}>Paramètres</button> : null}
          </div>

          {screen === 'beds' ? (
            <div className="screen active">
              <div className="controls-grid">
                {isAdmin ? (
                  <div className="form-card">
                    <h2>Ajouter un lit</h2>
                    <div className="form-grid">
                      <label>
                        Numéro
                        <input value={newBed.number} type="number" min="1" onChange={(event) => setNewBed((current) => ({ ...current, number: event.target.value }))} />
                      </label>
                      <label>
                        Nom
                        <input value={newBed.name} type="text" onChange={(event) => setNewBed((current) => ({ ...current, name: event.target.value }))} />
                      </label>
                      <label>
                        Type
                        <select value={newBed.type} onChange={(event) => setNewBed((current) => ({ ...current, type: event.target.value }))}>
                          <option value="standard">Standard</option>
                          <option value="thoracique">Thoracique</option>
                        </select>
                      </label>
                      <button className="btn primary" type="button" onClick={createBed}>Créer</button>
                    </div>
                  </div>
                ) : (
                  <div className="form-card">
                    <h2>Actions rapides</h2>
                    <p className="small-note">Les utilisateurs authentifiés peuvent modifier l’état des lits et gérer les patients. La création et la suppression de lits sont réservées à l’admin.</p>
                  </div>
                )}
                <div className="form-card">
                  <h2>Actions rapides</h2>
                  <p className="small-note">Les changements se synchronisent automatiquement sur les postes connectés.</p>
                </div>
              </div>
              <div className="grid">{renderBeds}</div>
              <div className="foot">Liaison réseau en attente.</div>
            </div>
          ) : null}

          {screen === 'patients' ? (
            <div className="screen active">
              <div className="controls-grid">
                {authenticated ? (
                  <div className="form-card">
                    <h2>Ajouter / assigner</h2>
                    <div className="form-grid">
                        <label>
                          Numéro
                          <input value={newPatient.registrationNumber} type="text" onChange={(event) => setNewPatient((current) => ({ ...current, registrationNumber: event.target.value }))} />
                        </label>
                        <label>
                          Nom
                          <input value={newPatient.name} type="text" onChange={(event) => setNewPatient((current) => ({ ...current, name: event.target.value }))} />
                        </label>
                        <label>
                          Lit
                          <input value={newPatient.bedNumber} type="number" min="1" onChange={(event) => setNewPatient((current) => ({ ...current, bedNumber: event.target.value }))} />
                        </label>
                        <button className="btn primary" type="button" onClick={savePatient}>Enregistrer</button>
                      </div>
                    </div>
                ) : (
                  <div className="form-card">
                    <h2>Liste des patients</h2>
                    <p className="small-note">Lecture seule tant que vous n’êtes pas connecté.</p>
                  </div>
                )}
                <div className="form-card">
                  <h2>Patients</h2>
                </div>
              </div>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Numéro d'inscription</th>
                      <th>Nom</th>
                      <th>Lit</th>
                      <th>Action</th>
                    </tr>
                  </thead>
                  <tbody>{renderPatients}</tbody>
                </table>
              </div>
            </div>
          ) : null}

          {screen === 'patientview' ? (
            <div className="screen active">
              <div className="controls-grid">
              </div>
              <div className="patient-full">
                    {(() => {
                      // next patient: prefer assigned (earliest assignedAt), otherwise first unassigned
                      const assigned = patients.filter(p => p.status === 'assigned' && p.bedNumber).sort((a,b)=> {
                        const ta = a.assignedAt ? new Date(a.assignedAt).getTime() : 0;
                        const tb = b.assignedAt ? new Date(b.assignedAt).getTime() : 0;
                        return ta - tb;
                      });
                      let next = null;
                      if (assigned.length) next = assigned[0];
                      else {
                        const unassigned = patients.filter(p => !p.bedNumber).sort((a,b)=> (a.registrationNumber||'').localeCompare(b.registrationNumber||''));
                        if (unassigned.length) next = unassigned[0];
                      }
                      if (!next) return <div className="empty">Aucun patient.</div>;
                      return (
                        <div className="patient-center">
                          <div className="patient-line">Patient {escapeText(next.registrationNumber)} — Lit {next.bedNumber || '—'}</div>
                        </div>
                      );
                    })()}
                  </div>
            </div>
          ) : null}

          {screen === 'stats' && authenticated ? (
            <div className="screen active">
              <div className="controls-grid">
                <div className="form-card">
                  <h2>Statistiques</h2>
                  <p className="small-note">Consultations par date, patients archivés, durée moyenne des consultations.</p>
                </div>
              </div>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Date</th>
                      <th>Consultations</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(stats.consultationsByDate || []).length ? (stats.consultationsByDate || []).map(item => (
                      <tr key={item.date}><td>{item.date}</td><td>{item.count}</td></tr>
                    )) : (
                      <tr><td colSpan="2"><div className="empty">Aucune consultation enregistrée.</div></td></tr>
                    )}
                  </tbody>
                </table>
                <div className="stats-grid" style={{marginTop: 16}}>
                  <div className="stat"><span>Patients archivés</span><strong>{stats.archivedPatients || 0}</strong></div>
                  <div className="stat"><span>Total consultations</span><strong>{stats.totalConsultations || 0}</strong></div>
                  <div className="stat"><span>Durée moyenne (min)</span><strong>{Math.round(stats.avgConsultationMinutes) || 0}</strong></div>
                </div>
              </div>
            </div>
          ) : null}

          {screen === 'settings' && authenticated && isAdmin ? (
            <div className="screen active">
              <div className="controls-grid">
                <div className="form-card">
                  <h2>Ajouter un utilisateur</h2>
                  <div className="form-grid">
                    <label>
                      Identifiant
                      <input value={newUser.username} type="text" onChange={(event) => setNewUser((current) => ({ ...current, username: event.target.value }))} />
                    </label>
                    <label>
                      Mot de passe
                      <input value={newUser.password} type="password" onChange={(event) => setNewUser((current) => ({ ...current, password: event.target.value }))} />
                    </label>
                    <button className="btn primary" type="button" onClick={createUser}>Créer</button>
                  </div>
                </div>
                <div className="form-card">
                  <h2>Utilisateurs</h2>
                  <p className="small-note">Les comptes créés peuvent se connecter directement depuis la barre du haut.</p>
                </div>
              </div>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Identifiant</th>
                      <th>Rôle</th>
                    </tr>
                  </thead>
                  <tbody>{renderUsers}</tbody>
                </table>
              </div>
            </div>
          ) : null}
        </div>
      </div>

      <div className={`modal-backdrop ${modalOpen ? 'open' : ''}`} aria-hidden={!modalOpen}>
          <div className="modal section-card">
          <h2>Connexion</h2>
          <p className="small-note">{authMessage}</p>
          <div className="modal-grid">
            <label>
              Identifiant
              <input value={authForm.username} type="text" onChange={(event) => setAuthForm((current) => ({ ...current, username: event.target.value }))} />
            </label>
            <label>
              Mot de passe
              <input value={authForm.password} type="password" onChange={(event) => setAuthForm((current) => ({ ...current, password: event.target.value }))} />
            </label>
          </div>
          <div className="modal-actions">
            <button className="btn" type="button" onClick={() => setModalOpen(false)}>Fermer</button>
            <button className="btn primary" type="button" onClick={authenticate}>Valider</button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default App;
