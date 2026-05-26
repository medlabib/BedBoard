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

function loadCachedState() {
  try {
    const raw = localStorage.getItem('bedboard_state_cache');
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    return {
      beds: Array.isArray(parsed.beds) ? parsed.beds : [],
      patients: Array.isArray(parsed.patients) ? parsed.patients : [],
      stats: parsed.stats || initialStats,
    };
  } catch (error) {
    console.error(error);
    return null;
  }
}

function App() {
  const cached = loadCachedState();
  const [isPatientPage, setIsPatientPage] = useState(() => window.location.pathname === '/patient-view');
  const [beds, setBeds] = useState(cached?.beds || []);
  const [patients, setPatients] = useState(cached?.patients || []);
  const [stats, setStats] = useState(cached?.stats || initialStats);
  const [authenticated, setAuthenticated] = useState(false);
  const [user, setUser] = useState({ username: '', admin: false });
  const [users, setUsers] = useState([]);
  const [screen, setScreen] = useState('beds');
  const [pvIndex, setPvIndex] = useState(0);
  const [modalOpen, setModalOpen] = useState(false);
  const [confirm, setConfirm] = useState({ open: false, title: '', message: '', onConfirm: null });
  const [authForm, setAuthForm] = useState({ username: 'admin', password: '' });
  const [authMessage, setAuthMessage] = useState('Connectez-vous pour gérer les lits et les patients sur ce poste.');
  const [connectionState, setConnectionState] = useState('Liaison réseau en attente.');
  const [newBed, setNewBed] = useState({ number: '', name: '', type: 'standard' });
  const [newPatient, setNewPatient] = useState({ registrationNumber: '', name: '', bedNumber: '' });
  const [newUser, setNewUser] = useState({ username: '', password: '' });
  const [assignByBed, setAssignByBed] = useState({});
  const [passwordForm, setPasswordForm] = useState({ currentPassword: '', newPassword: '', confirmPassword: '' });
  const [resetPasswordForm, setResetPasswordForm] = useState({ username: '', newPassword: '', confirmPassword: '' });
  const [bedEdits, setBedEdits] = useState({});
  const [auditLogs, setAuditLogs] = useState([]);
  const [lastBackupFile, setLastBackupFile] = useState('');
  const [notice, setNotice] = useState({ type: '', text: '' });

  const isAdmin = user.admin;

  const api = async (url, options = {}) => {
    const response = await fetch(url, {
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
      ...options,
    });
    return response;
  };

  const readErrorMessage = async (response, fallback) => {
    const message = await response.text().catch(() => '');
    const clean = String(message || '').trim();
    return clean || fallback;
  };

  const showError = (text) => setNotice({ type: 'error', text });
  const showSuccess = (text) => setNotice({ type: 'success', text });

  const closeConfirm = () => {
    setConfirm({ open: false, title: '', message: '', onConfirm: null });
  };

  const handleConfirmAction = async () => {
    try {
      if (confirm.onConfirm) {
        await confirm.onConfirm();
      }
    } catch (error) {
      console.error(error);
      showError('Action impossible pour le moment.');
    } finally {
      closeConfirm();
    }
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
      await refreshAudit(true);
    }
  };

  const refreshState = async () => {
    const response = await fetch('/api/state', { credentials: 'include' });
    const data = await response.json().catch(() => ({}));
    setBeds(Array.isArray(data.beds) ? data.beds : []);
    setPatients(Array.isArray(data.patients) ? data.patients : []);
    setStats(data.stats || initialStats);
  };

  const refreshAudit = async (force = false) => {
    if (!force && !isAdmin) {
      setAuditLogs([]);
      return;
    }
    const response = await api('/api/audit');
    const data = await response.json().catch(() => ({ logs: [] }));
    setAuditLogs(Array.isArray(data.logs) ? data.logs : []);
  };

  const connectStream = () => {
    const stream = new EventSource('/api/stream');
    setConnectionState('Connexion locale...');

    let refreshTimer = null;
    const scheduleRefresh = () => {
      if (refreshTimer) return;
      refreshTimer = setTimeout(() => {
        refreshTimer = null;
        refreshState().catch(() => {
          setConnectionState('Synchronisation locale interrompue.');
        });
      }, 120);
    };

    stream.addEventListener('state.snapshot', (event) => {
      try {
        const data = JSON.parse(event.data);
        setBeds(Array.isArray(data.beds) ? data.beds : []);
        setPatients(Array.isArray(data.patients) ? data.patients : []);
        setStats(data.stats || initialStats);
        setConnectionState('Connecté.');
      } catch (error) {
        console.error(error);
      }
    });

    ['state.changed', 'bed.updated', 'bed.created', 'bed.deleted', 'patient.updated', 'patient.archived', 'user.updated', 'system.backup', 'system.restore'].forEach((eventName) => {
      stream.addEventListener(eventName, () => {
        scheduleRefresh();
        if (eventName === 'user.updated') {
          refreshUsers().catch(() => {});
        }
        if (eventName === 'system.restore' || eventName === 'bed.updated') {
          refreshAudit().catch(() => {});
        }
      });
    });

    stream.onmessage = (event) => {
      try {
        if (!event.data) return;
        const data = JSON.parse(event.data);
        if (Array.isArray(data.beds) || Array.isArray(data.patients) || data.stats) {
          setBeds(Array.isArray(data.beds) ? data.beds : []);
          setPatients(Array.isArray(data.patients) ? data.patients : []);
          setStats(data.stats || initialStats);
          setConnectionState('Connecté.');
        }
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

  useEffect(() => {
    const onPopState = () => {
      setIsPatientPage(window.location.pathname === '/patient-view');
    };
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  useEffect(() => {
    const next = { beds, patients, stats, savedAt: new Date().toISOString() };
    localStorage.setItem('bedboard_state_cache', JSON.stringify(next));
  }, [beds, patients, stats]);

  useEffect(() => {
    setBedEdits((current) => {
      const valid = new Set(beds.map((b) => b.number));
      const next = {};
      Object.entries(current).forEach(([key, value]) => {
        const number = Number(key);
        if (valid.has(number)) {
          next[key] = value;
        }
      });
      return next;
    });
  }, [beds]);

  // auto-rotate patientview entries when visible
  useEffect(() => {
    if (screen !== 'patientview' && !isPatientPage) return;
    const rot = setInterval(() => {
      const list = patients.filter(p => p.status === 'assigned' || !p.bedNumber);
      if (!list.length) return;
      setPvIndex((i) => (i + 1) % list.length);
    }, 8000);
    return () => clearInterval(rot);
  }, [screen, patients, isPatientPage]);

  // jump to newest assigned patient whenever a new assignment appears
  useEffect(() => {
    if (screen !== 'patientview' && !isPatientPage) return;
    const assigned = patients.filter((p) => p.status === 'assigned' && p.bedNumber);
    if (!assigned.length) return;
    setPvIndex(0);
  }, [patients, screen, isPatientPage]);

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
                    const response = await api('/api/status', {
                      method: 'POST',
                      body: JSON.stringify({ number: bed.number, status }),
                    });
                    if (!response.ok) {
                      showError(await readErrorMessage(response, 'Mise à jour état lit impossible.'));
                      return;
                    }
                    showSuccess(`Lit ${bed.number} mis à jour: ${statusMeta[status].label}.`);
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
                    showSuccess(`Lit ${bed.number} supprimé.`);
                  } });
                }}
              >
                Supprimer
              </button>
            ) : null}
            {authenticated ? (
              <div className="assign-box">
                <select
                  value={assignByBed[bed.number] || ''}
                  onChange={(e) => setAssignByBed((current) => ({ ...current, [bed.number]: e.target.value }))}
                >
                  <option value="">Affecter un patient</option>
                  {patients.filter(p => !p.bedNumber).map(p => (
                    <option key={p.registrationNumber} value={p.registrationNumber}>{p.registrationNumber}</option>
                  ))}
                </select>
                <button className="mini-btn" type="button" onClick={async () => {
                  const reg = assignByBed[bed.number];
                  if (!reg) {
                    showError('Sélectionnez un patient avant affectation.');
                    return;
                  }
                  const selectedPatient = patients.find((p) => p.registrationNumber === reg);
                  const response = await api('/api/patients', {
                    method: 'POST',
                    body: JSON.stringify({ registrationNumber: reg, name: selectedPatient?.name || '', bedNumber: bed.number }),
                  });
                  if (!response.ok) {
                    showError(await readErrorMessage(response, 'Affectation patient impossible.'));
                    return;
                  }
                  setAssignByBed((current) => ({ ...current, [bed.number]: '' }));
                  showSuccess(`Patient ${reg} affecté au lit ${bed.number}.`);
                }}>Affecter</button>
              </div>
            ) : null}
            {authenticated ? (
              <div className="form-grid compact">
                {(() => {
                  const draft = bedEdits[bed.number] || { name: bed.name, type: bed.type };
                  return (
                    <>
                <label>
                  Nom
                  <input
                    value={draft.name}
                    onChange={(event) => {
                      const nextName = event.target.value;
                      setBedEdits((current) => ({ ...current, [bed.number]: { ...(current[bed.number] || { name: bed.name, type: bed.type }), name: nextName } }));
                    }}
                  />
                </label>
                <label>
                  Type
                  <select
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
                    const next = bedEdits[bed.number] || { name: bed.name, type: bed.type };
                    const response = await api('/api/config-bed', {
                      method: 'POST',
                      body: JSON.stringify({ number: bed.number, name: next.name, type: next.type }),
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
                    showSuccess(`Configuration du lit ${bed.number} enregistrée.`);
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
  }, [beds, authenticated, isAdmin, assignByBed, patients]);

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
              <button className="mini-btn" type="button" onClick={() => {
                setConfirm({ open: true, title: 'Archiver le patient', message: `Archiver le patient ${patient.registrationNumber} ?`, onConfirm: async () => {
                  const response = await api('/api/patients/archive', { method: 'POST', body: JSON.stringify({ registrationNumber: patient.registrationNumber, action: 'archive' }) });
                  if (!response.ok) {
                    showError(await readErrorMessage(response, 'Archivage patient impossible.'));
                    return;
                  }
                  setPatients((current) => current.map((item) => item.registrationNumber === patient.registrationNumber
                    ? { ...item, status: 'archived', bedNumber: null, bedName: '' }
                    : item));
                  showSuccess(`Patient ${patient.registrationNumber} archivé.`);
                } });
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
          <td colSpan="3"><div className="empty">Réservé à l’admin.</div></td>
        </tr>
      );
    }
    if (!users.length) {
      return (
        <tr>
          <td colSpan="3"><div className="empty">Aucun utilisateur.</div></td>
        </tr>
      );
    }
    return users.map((item) => (
      <tr key={item.username}>
        <td>{escapeText(item.username)}</td>
        <td>{item.admin ? 'Admin' : 'Utilisateur'}</td>
        <td>
          <button
            className="mini-btn"
            type="button"
            onClick={() => setResetPasswordForm((current) => ({ ...current, username: item.username }))}
          >
            Changer mot de passe
          </button>
        </td>
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
      showSuccess('Lit créé.');
      return;
    }
    showError(await readErrorMessage(response, 'Création lit impossible.'));
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
      showSuccess('Patient enregistré.');
      return;
    }
    showError(await readErrorMessage(response, 'Enregistrement patient impossible.'));
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
      setAuthForm((current) => ({ ...current, password: '' }));
      setConnectionState(data.username ? `Connecté: ${data.username}` : 'Connecté.');
      showSuccess('Connexion réussie.');
      if (data.admin) {
        await refreshUsers();
      }
      return;
    }
    const message = await readErrorMessage(response, 'Identifiants invalides.');
    setAuthMessage(message);
    showError(message);
  };

  const logout = async () => {
    await fetch('/api/logout', { method: 'POST', credentials: 'include' });
    setAuthenticated(false);
    setUser({ username: '', admin: false });
    setUsers([]);
    setConnectionState('Accès local');
    showSuccess('Déconnexion effectuée.');
  };

  const createUser = async () => {
    const response = await api('/api/users', {
      method: 'POST',
      body: JSON.stringify(newUser),
    });
    if (response.ok) {
      setNewUser({ username: '', password: '' });
      await refreshUsers();
      showSuccess('Utilisateur créé.');
      return;
    }
    showError(await readErrorMessage(response, 'Création utilisateur impossible.'));
  };

  const changeOwnPassword = async () => {
    if (!passwordForm.currentPassword || !passwordForm.newPassword) {
      showError('Mot de passe actuel et nouveau requis.');
      return;
    }
    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      showError('Confirmation du nouveau mot de passe invalide.');
      return;
    }
    const response = await api('/api/users/password', {
      method: 'POST',
      body: JSON.stringify({
        currentPassword: passwordForm.currentPassword,
        newPassword: passwordForm.newPassword,
      }),
    });
    if (response.ok) {
      setPasswordForm({ currentPassword: '', newPassword: '', confirmPassword: '' });
      showSuccess('Mot de passe mis à jour.');
      return;
    }
    const message = await response.text().catch(() => 'Changement de mot de passe impossible.');
    showError(message || 'Changement de mot de passe impossible.');
  };

  const resetUserPassword = async () => {
    if (!isAdmin) return;
    if (!resetPasswordForm.username || !resetPasswordForm.newPassword) {
      showError('Utilisateur et nouveau mot de passe requis.');
      return;
    }
    if (resetPasswordForm.newPassword !== resetPasswordForm.confirmPassword) {
      showError('Confirmation du nouveau mot de passe invalide.');
      return;
    }
    const response = await api('/api/users/password', {
      method: 'POST',
      body: JSON.stringify({
        username: resetPasswordForm.username,
        newPassword: resetPasswordForm.newPassword,
      }),
    });
    if (response.ok) {
      setResetPasswordForm({ username: '', newPassword: '', confirmPassword: '' });
      showSuccess('Mot de passe utilisateur mis à jour.');
      return;
    }
    const message = await response.text().catch(() => 'Réinitialisation mot de passe impossible.');
    showError(message || 'Réinitialisation mot de passe impossible.');
  };

  const openSettings = async () => {
    setScreen('settings');
    await refreshUsers();
    await refreshAudit(true);
  };

  const openAccount = () => {
    setScreen('account');
  };

  const openPatientPage = () => {
    window.location.assign('/patient-view');
  };

  const openMainPage = () => {
    window.location.assign('/');
  };

  const createBackup = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/backup', { method: 'POST', body: JSON.stringify({}) });
    if (!response.ok) {
      showError(await readErrorMessage(response, 'Sauvegarde impossible.'));
      return;
    }
    const data = await response.json().catch(() => ({}));
    setLastBackupFile(data.file || '');
    showSuccess('Sauvegarde SQLite créée.');
    await refreshAudit();
  };

  const restoreLatestBackup = async () => {
    if (!isAdmin) return;
    setConfirm({
      open: true,
      title: 'Restaurer la dernière sauvegarde',
      message: 'Cette action remplace la base SQLite actuelle. Continuer ?',
      onConfirm: async () => {
        const response = await api('/api/admin/restore', { method: 'POST', body: JSON.stringify({}) });
        if (!response.ok) {
          showError(await readErrorMessage(response, 'Restauration impossible.'));
          return;
        }
        const data = await response.json().catch(() => ({}));
        setLastBackupFile(data.file || '');
        await refreshState();
        await refreshAudit();
        showSuccess('Restauration SQLite terminée.');
      },
    });
  };

  const patientPanel = useMemo(() => {
    const assigned = patients.filter(p => p.status === 'assigned' && p.bedNumber).sort((a, b) => {
      const ta = a.assignedAt ? new Date(a.assignedAt).getTime() : 0;
      const tb = b.assignedAt ? new Date(b.assignedAt).getTime() : 0;
      return tb - ta;
    });
    const list = assigned.length
      ? assigned
      : (patients.filter(p => !p.bedNumber).length ? patients.filter(p => !p.bedNumber) : patients);
    if (!list.length) return <div className="empty">Aucun patient.</div>;
    const current = list[pvIndex % list.length];
    return (
      <div className="patient-center">
        <div className="patient-line">Patient {escapeText(current.registrationNumber)} — Lit {current.bedNumber || '—'}</div>
      </div>
    );
  }, [patients, pvIndex]);

  if (isPatientPage) {
    return (
      <div className="app patient-page-shell">
        <div className="navbar">
          <div className="nav-left">
            <div className="brand-mark"><img src="/logo.png" alt="Logo de l'hôpital" /></div>
            <div className="nav-title">
              <strong>BedBoard - Vue patient</strong>
              <span>Affichage salle d'attente</span>
            </div>
          </div>
          <div className="nav-actions">
            <button className="btn" type="button" onClick={openMainPage}>Retour tableau</button>
          </div>
        </div>
        <div className="section-card patient-page-card">
          <div className="patient-full">
            {patientPanel}
          </div>
        </div>
      </div>
    );
  }

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
          <button className="btn" type="button" onClick={openPatientPage}>Page patient</button>
          {authenticated && isAdmin ? <button className="btn" type="button" onClick={openSettings}>Paramètres</button> : null}
          {!authenticated ? <button className="btn" type="button" onClick={() => setModalOpen(true)}>Se connecter</button> : null}
          {authenticated ? <button className="btn primary" type="button" onClick={logout}>Déconnexion</button> : null}
        </div>
      </div>

      {notice.text ? <div className={`notice ${notice.type === 'error' ? 'error' : 'success'}`}>{notice.text}</div> : null}

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
            {authenticated ? <button className={`tab-btn ${screen === 'stats' ? 'active' : ''}`} type="button" onClick={() => setScreen('stats')}>Statistiques</button> : null}
            {authenticated ? <button className={`tab-btn ${screen === 'account' ? 'active' : ''}`} type="button" onClick={openAccount}>Mon compte</button> : null}
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
                {patientPanel}
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

          {screen === 'account' && authenticated ? (
            <div className="screen active">
              <div className="controls-grid">
                <div className="form-card">
                  <h2>Changer mon mot de passe</h2>
                  <div className="form-grid">
                    <label>
                      Mot de passe actuel
                      <input value={passwordForm.currentPassword} type="password" onChange={(event) => setPasswordForm((current) => ({ ...current, currentPassword: event.target.value }))} />
                    </label>
                    <label>
                      Nouveau mot de passe
                      <input value={passwordForm.newPassword} type="password" onChange={(event) => setPasswordForm((current) => ({ ...current, newPassword: event.target.value }))} />
                    </label>
                    <label>
                      Confirmer
                      <input value={passwordForm.confirmPassword} type="password" onChange={(event) => setPasswordForm((current) => ({ ...current, confirmPassword: event.target.value }))} />
                    </label>
                    <button className="btn primary" type="button" onClick={changeOwnPassword}>Mettre à jour</button>
                  </div>
                </div>
                <div className="form-card">
                  <h2>Compte</h2>
                  <p className="small-note">Connecté en tant que {escapeText(user.username)}.</p>
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
                  <h2>Changer mot de passe utilisateur</h2>
                  <div className="form-grid">
                    <label>
                      Utilisateur
                      <input value={resetPasswordForm.username} type="text" onChange={(event) => setResetPasswordForm((current) => ({ ...current, username: event.target.value }))} />
                    </label>
                    <label>
                      Nouveau mot de passe
                      <input value={resetPasswordForm.newPassword} type="password" onChange={(event) => setResetPasswordForm((current) => ({ ...current, newPassword: event.target.value }))} />
                    </label>
                    <label>
                      Confirmer
                      <input value={resetPasswordForm.confirmPassword} type="password" onChange={(event) => setResetPasswordForm((current) => ({ ...current, confirmPassword: event.target.value }))} />
                    </label>
                    <button className="btn primary" type="button" onClick={resetUserPassword}>Mettre à jour</button>
                  </div>
                </div>
                <div className="form-card">
                  <h2>Utilisateurs</h2>
                  <p className="small-note">Les comptes créés peuvent se connecter directement depuis la barre du haut.</p>
                </div>
                <div className="form-card">
                  <h2>Sauvegarde / restauration</h2>
                  <div className="form-grid">
                    <button className="btn primary" type="button" onClick={createBackup}>Sauvegarde 1 clic</button>
                    <button className="btn" type="button" onClick={restoreLatestBackup}>Restaurer dernière sauvegarde</button>
                  </div>
                  <p className="small-note">Dernière sauvegarde: {lastBackupFile ? escapeText(lastBackupFile) : 'Aucune'}</p>
                </div>
              </div>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Identifiant</th>
                      <th>Rôle</th>
                      <th>Action</th>
                    </tr>
                  </thead>
                  <tbody>{renderUsers}</tbody>
                </table>
              </div>
              <div className="table-wrap" style={{ marginTop: 16 }}>
                <table>
                  <thead>
                    <tr>
                      <th>Heure</th>
                      <th>Utilisateur</th>
                      <th>Action</th>
                      <th>Objet</th>
                    </tr>
                  </thead>
                  <tbody>
                    {auditLogs.length ? auditLogs.map((entry) => (
                      <tr key={entry.id || `${entry.createdAt}-${entry.action}-${entry.entityKey}`}>
                        <td>{entry.createdAt ? new Date(entry.createdAt).toLocaleString() : '—'}</td>
                        <td>{escapeText(entry.username || 'system')}</td>
                        <td>{escapeText(entry.action)}</td>
                        <td>{escapeText(entry.entityKey || entry.entity)}</td>
                      </tr>
                    )) : (
                      <tr><td colSpan="4"><div className="empty">Aucune action journalisée.</div></td></tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          ) : null}
        </div>
      </div>

      {modalOpen ? (
      <div className="modal-backdrop open" role="dialog" aria-modal="true">
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
      ) : null}

      {confirm.open ? (
      <div className="modal-backdrop open" role="dialog" aria-modal="true">
        <div className="modal section-card">
          <h2>{confirm.title}</h2>
          <p className="small-note">{confirm.message}</p>
          <div className="modal-actions">
            <button className="btn" type="button" onClick={closeConfirm}>Annuler</button>
            <button className="btn primary" type="button" onClick={handleConfirmAction}>Confirmer</button>
          </div>
        </div>
      </div>
      ) : null}
    </div>
  );
}

export default App;
