import { useEffect, useMemo, useState } from 'react';
import { callLanguageOptions, getCallTemplate } from './lib/call';
import { normalizeRole } from './lib/roles';
import { triageLevelOf, triageMeta } from './lib/triage';
import AccountScreen from './components/AccountScreen';
import BedsGrid from './components/BedsGrid';
import PatientViewPage from './components/PatientViewPage';
import PatientsRows from './components/PatientsRows';
import ReceptionPage from './components/ReceptionPage';
import SettingsScreen from './components/SettingsScreen';
import SimpleModal from './components/SimpleModal';
import StatsScreen from './components/StatsScreen';

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
  triageByLevel: { 0: 0, 1: 0, 2: 0, 3: 0, 4: 0 },
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

    const safeBeds = (Array.isArray(parsed.beds) ? parsed.beds : []).map((bed) => ({
      ...bed,
      patientName: '',
      patientRegistration: '',
      hasPatient: false,
    }));

    return {
      beds: safeBeds,
      patients: [],
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
  const [user, setUser] = useState({ username: '', admin: false, role: 'user' });
  const [users, setUsers] = useState([]);
  const [screen, setScreen] = useState('beds');
  const [callLanguage, setCallLanguage] = useState('fr-FR');
  const [modalOpen, setModalOpen] = useState(false);
  const [confirm, setConfirm] = useState({ open: false, title: '', message: '', onConfirm: null });
  const [authForm, setAuthForm] = useState({ username: 'admin', password: '' });
  const [authMessage, setAuthMessage] = useState('Connectez-vous pour gérer les lits et les patients sur ce poste.');
  const [connectionState, setConnectionState] = useState('Liaison réseau en attente.');
  const [newBed, setNewBed] = useState({ number: '', room: '', roomAlt: '', name: '', nameAlt: '', type: 'standard' });
  const [newPatient, setNewPatient] = useState({ registrationNumber: '', name: '', bedNumber: '', triageScore: '0' });
  const [newUser, setNewUser] = useState({ username: '', password: '', role: 'user' });
  const [assignByBed, setAssignByBed] = useState({});
  const [passwordForm, setPasswordForm] = useState({ currentPassword: '', newPassword: '', confirmPassword: '' });
  const [resetPasswordForm, setResetPasswordForm] = useState({ username: '', newPassword: '', confirmPassword: '' });
  const [gotifyForm, setGotifyForm] = useState({ enabled: false, url: '', token: '', priority: 8, tokenConfigured: false, clearToken: false });
  const [securityHealth, setSecurityHealth] = useState({ status: 'unknown', checks: [], loaded: false });
  const [bedEdits, setBedEdits] = useState({});
  const [auditLogs, setAuditLogs] = useState([]);
  const [lastBackupFile, setLastBackupFile] = useState('');
  const [notice, setNotice] = useState({ type: '', text: '' });

  const isAdmin = user.admin;
  const role = normalizeRole(user.role);
  const isReception = role === 'reception';
  const isTriage = role === 'triage';
  const isDechocage = role === 'dechocage';
  const canManageBeds = authenticated && (role === 'admin' || role === 'user' || role === 'dechocage');
  const canManagePatients = authenticated && (role === 'admin' || role === 'user' || role === 'triage' || role === 'dechocage');
  const canArchivePatients = authenticated && (role === 'admin' || role === 'user' || role === 'dechocage');
  const canViewTriage = authenticated && !isReception;
  const securityNavStatus = String(securityHealth?.status || 'unknown').toLowerCase();

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

  useEffect(() => {
    if (!notice.text) return undefined;
    const timer = window.setTimeout(() => {
      setNotice({ type: '', text: '' });
    }, 4500);
    return () => window.clearTimeout(timer);
  }, [notice]);

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
    const data = await response.json().catch(() => ({ authenticated: false, username: '', admin: false, role: 'user' }));
    setAuthenticated(Boolean(data.authenticated));
    setUser({ username: data.username || '', admin: Boolean(data.admin), role: normalizeRole(data.role) });
    if (data.authenticated) {
      setModalOpen(false);
    }
    if (data.authenticated && data.username) {
      setConnectionState(`Connecté: ${data.username}`);
    } else {
      setConnectionState('Accès local');
      setBeds([]);
      setPatients([]);
      setStats(initialStats);
      localStorage.removeItem('bedboard_state_cache');
    }
    if (data.authenticated && data.admin) {
      await refreshUsers();
      await refreshAudit(true);
    }
    if (data.authenticated && normalizeRole(data.role) === 'reception' && window.location.pathname !== '/patient-view') {
      window.location.assign('/patient-view');
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

  const refreshGotifySettings = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/integrations/gotify', { method: 'GET' });
    if (!response.ok) {
      showError(await readErrorMessage(response, 'Lecture configuration Gotify impossible.'));
      return;
    }
    const data = await response.json().catch(() => ({}));
    setGotifyForm((current) => ({
      ...current,
      enabled: Boolean(data.enabled),
      url: data.url || '',
      token: '',
      priority: Number(data.priority || 8),
      tokenConfigured: Boolean(data.tokenConfigured),
      clearToken: false,
    }));
  };

  const saveGotifySettings = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/integrations/gotify', {
      method: 'POST',
      body: JSON.stringify({
        enabled: Boolean(gotifyForm.enabled),
        url: gotifyForm.url,
        token: gotifyForm.token,
        priority: Number(gotifyForm.priority || 8),
        clearToken: Boolean(gotifyForm.clearToken),
      }),
    });
    if (!response.ok) {
      showError(await readErrorMessage(response, 'Enregistrement Gotify impossible.'));
      return;
    }
    await refreshGotifySettings();
    showSuccess('Configuration Gotify enregistrée.');
  };

  const refreshSecurityHealth = async () => {
    if (!isAdmin) {
      setSecurityHealth({ status: 'unknown', checks: [], loaded: false });
      return;
    }
    const response = await api('/api/admin/security/health', { method: 'GET' });
    if (!response.ok) {
      const fallback = await readErrorMessage(response, 'Lecture audit sécurité impossible.');
      setSecurityHealth({ status: 'unknown', checks: [], loaded: true, error: fallback });
      return;
    }
    const data = await response.json().catch(() => ({}));
    setSecurityHealth({
      status: data.status || 'unknown',
      checks: Array.isArray(data.checks) ? data.checks : [],
      loaded: true,
      error: '',
    });
  };

  useEffect(() => {
    if (!authenticated || !isAdmin) {
      return;
    }
    refreshSecurityHealth().catch(() => {});
    const timer = window.setInterval(() => {
      refreshSecurityHealth().catch(() => {});
    }, 60000);
    return () => window.clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [authenticated, isAdmin]);

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

    stream.addEventListener('alert.urgent', (event) => {
      try {
        const payload = JSON.parse(event.data || '{}');
        const title = payload.title || 'URGENT';
        const patient = payload.patient || 'Patient';
        const room = payload.room || 'Chambre';
        const bed = payload.bed || 'Lit';
        const timeHM = payload.timeHM || '--:--';
        const sourceUser = payload.sourceUser || 'system';
        showError(`${title} - ${patient} - ${room} / ${bed} - ${timeHM} - ${sourceUser}`);
        const currentRole = normalizeRole(window.localStorage.getItem('bedboard_current_role'));
        if (currentRole === 'dechocage') {
          playUrgentBeep();
        }
      } catch (error) {
        console.error(error);
      }
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
    if (!authenticated) {
      localStorage.removeItem('bedboard_state_cache');
      return;
    }
    localStorage.setItem('bedboard_state_cache', JSON.stringify(next));
  }, [beds, patients, stats, authenticated]);

  useEffect(() => {
    window.localStorage.setItem('bedboard_current_role', role);
  }, [role]);

  const activePatients = useMemo(() => patients.filter((p) => p.status !== 'archived'), [patients]);
  const bedByNumber = useMemo(() => {
    const map = new Map();
    beds.forEach((bed) => map.set(Number(bed.number), bed));
    return map;
  }, [beds]);

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

  const speakPatientCall = (patient) => {
    if (!('speechSynthesis' in window)) {
      showError('Synthèse vocale non disponible sur ce navigateur.');
      return;
    }
    if (!patient?.bedNumber) {
      showError('Patient non assigné, appel vocal indisponible.');
      return;
    }
    const bed = bedByNumber.get(Number(patient.bedNumber));
    const roomName = bed?.roomAlt || bed?.room || patient.roomNameAlt || patient.roomName || 'Chambre';
    const bedName = bed?.nameAlt || bed?.name || patient.bedNameAlt || patient.bedName || `Lit ${patient.bedNumber}`;
    const text = getCallTemplate(callLanguage, patient.name || patient.registrationNumber, roomName, bedName);
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = callLanguage;
    window.speechSynthesis.cancel();
    window.speechSynthesis.speak(utterance);
    showSuccess(`Appel vocal lancé pour ${patient.registrationNumber}.`);
  };

  const playUrgentBeep = () => {
    try {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (!AudioCtx) return;
      const ctx = new AudioCtx();
      const oscillator = ctx.createOscillator();
      const gain = ctx.createGain();
      oscillator.type = 'square';
      oscillator.frequency.value = 880;
      gain.gain.setValueAtTime(0.0001, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.2, ctx.currentTime + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.45);
      oscillator.connect(gain);
      gain.connect(ctx.destination);
      oscillator.start();
      oscillator.stop(ctx.currentTime + 0.45);
    } catch (error) {
      console.error(error);
    }
  };

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
        <td>{escapeText(item.role || (item.admin ? 'admin' : 'user'))}</td>
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
        room: newBed.room,
        roomAlt: newBed.roomAlt,
        name: newBed.name,
        nameAlt: newBed.nameAlt,
        type: newBed.type,
      }),
    });
    if (response.ok) {
      setNewBed({ number: '', room: '', roomAlt: '', name: '', nameAlt: '', type: 'standard' });
      showSuccess('Lit créé.');
      return;
    }
    showError(await readErrorMessage(response, 'Création lit impossible.'));
  };

  const savePatient = async () => {
    if (!canManagePatients) {
      showError('Action non autorisée pour votre profil.');
      return;
    }
    if (isTriage && Number(newPatient.bedNumber) > 0) {
      showError('Le profil triage ne peut pas affecter un lit.');
      return;
    }
    const response = await api('/api/patients', {
      method: 'POST',
      body: JSON.stringify({
        registrationNumber: newPatient.registrationNumber,
        name: newPatient.name,
        bedNumber: Number(newPatient.bedNumber),
        triageScore: Number(newPatient.triageScore),
      }),
    });
    if (response.ok) {
      setNewPatient({ registrationNumber: '', name: '', bedNumber: '', triageScore: '0' });
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
      const nextRole = normalizeRole(data.role);
      setUser({ username: data.username || authForm.username || 'admin', admin: Boolean(data.admin), role: nextRole });
      setModalOpen(false);
      setAuthForm((current) => ({ ...current, password: '' }));
      setConnectionState(data.username ? `Connecté: ${data.username}` : 'Connecté.');
      showSuccess('Connexion réussie.');
      if (data.admin) {
        await refreshUsers();
      }
      if (nextRole === 'reception') {
        window.location.assign('/patient-view');
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
    setUser({ username: '', admin: false, role: 'user' });
    setUsers([]);
    setConnectionState('Accès local');
    setBeds([]);
    setPatients([]);
    setStats(initialStats);
    localStorage.removeItem('bedboard_state_cache');
    localStorage.removeItem('bedboard_current_role');
    showSuccess('Déconnexion effectuée.');
  };

  const createUser = async () => {
    const response = await api('/api/users', {
      method: 'POST',
      body: JSON.stringify(newUser),
    });
    if (response.ok) {
      setNewUser({ username: '', password: '', role: 'user' });
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
    await Promise.all([refreshUsers(), refreshAudit(true), refreshGotifySettings(), refreshSecurityHealth()]);
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

  const currentPatient = useMemo(() => {
    const assigned = activePatients.filter(p => p.status === 'assigned' && p.bedNumber).sort((a, b) => {
      const ta = a.assignedAt ? new Date(a.assignedAt).getTime() : 0;
      const tb = b.assignedAt ? new Date(b.assignedAt).getTime() : 0;
      return tb - ta;
    });
    const unassigned = activePatients.filter((p) => !p.bedNumber).sort((a, b) => {
      const ta = a.assignedAt ? new Date(a.assignedAt).getTime() : 0;
      const tb = b.assignedAt ? new Date(b.assignedAt).getTime() : 0;
      return tb - ta;
    });
    return assigned[0] || unassigned[0] || null;
  }, [activePatients]);

  const patientPanel = useMemo(() => {
    const current = currentPatient;
    if (!current) return <div className="empty">Aucun patient.</div>;
    const level = triageLevelOf(current);
    const triage = triageMeta[level] || triageMeta[0];
    return (
      <div className="patient-center">
        {canViewTriage ? <div className="triage-pill patient-triage" style={{ background: triage.color }}>{triage.label}</div> : null}
        <div className="patient-line">Patient {escapeText(current.registrationNumber)} — {current.bedNumber ? `${escapeText(current.roomName || 'Chambre')} - ${escapeText(current.bedName || `Lit ${current.bedNumber}`)}` : 'Non assigné'}</div>
      </div>
    );
  }, [currentPatient, canViewTriage]);

  if (isPatientPage) {
    if (!authenticated) {
      window.location.assign('/');
      return null;
    }
    return (
      <PatientViewPage
        callLanguage={callLanguage}
        setCallLanguage={setCallLanguage}
        callLanguageOptions={callLanguageOptions}
        currentPatient={currentPatient}
        speakPatientCall={speakPatientCall}
        openMainPage={openMainPage}
        patientPanel={patientPanel}
      />
    );
  }

  if (authenticated && isReception) {
    return (
      <ReceptionPage logout={logout} openPatientPage={openPatientPage} />
    );
  }

  return (
    <div className="app">
      <div className="navbar">
        <div className="nav-left">
          <div className="brand-mark"><img src="/logo.svg" alt="Logo de l'hôpital" /></div>
          <div className="nav-title">
            <strong>BedBoard</strong>
            <span>{connectionState}</span>
          </div>
        </div>
        <div className="nav-actions">
          <select className="form-select" value={callLanguage} onChange={(event) => setCallLanguage(event.target.value)}>
            {callLanguageOptions.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
          {authenticated && isAdmin ? (
            <button className={`security-chip inline ${securityNavStatus}`} type="button" onClick={openSettings}>
              Securite: {securityNavStatus.toUpperCase()}
            </button>
          ) : null}
          {authenticated && user.username ? <div className="user-chip">{user.username} ({role})</div> : null}
          <button className="btn" type="button" onClick={openPatientPage}>Page patient</button>
          {authenticated && isAdmin ? <button className="btn" type="button" onClick={openSettings}>Paramètres</button> : null}
          {!authenticated ? <button className="btn" type="button" onClick={() => setModalOpen(true)}>Se connecter</button> : null}
          {authenticated ? <button className="btn primary" type="button" onClick={logout}>Déconnexion</button> : null}
        </div>
      </div>

      {notice.text ? (
        <div className={`notice ${notice.type === 'error' ? 'error' : 'success'}`} role="status" aria-live="polite">
          <span>{notice.text}</span>
          <button
            className="notice-close"
            type="button"
            aria-label="Fermer la notification"
            onClick={() => setNotice({ type: '', text: '' })}
          >
            Fermer
          </button>
        </div>
      ) : null}

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
            {canManageBeds ? <button className={`tab-btn ${screen === 'beds' ? 'active' : ''}`} type="button" onClick={() => setScreen('beds')}>Lits</button> : null}
            {canManagePatients ? <button className={`tab-btn ${screen === 'patients' ? 'active' : ''}`} type="button" onClick={() => setScreen('patients')}>Patients</button> : null}
            {authenticated && isAdmin ? <button className={`tab-btn ${screen === 'stats' ? 'active' : ''}`} type="button" onClick={() => setScreen('stats')}>Statistiques</button> : null}
            {authenticated ? <button className={`tab-btn ${screen === 'account' ? 'active' : ''}`} type="button" onClick={openAccount}>Mon compte</button> : null}
            {authenticated && isAdmin ? <button className={`tab-btn ${screen === 'settings' ? 'active' : ''}`} type="button" onClick={openSettings}>Paramètres</button> : null}
          </div>

          {screen === 'beds' && canManageBeds ? (
            <div className="screen active">
              <div className="controls-grid">
                {isAdmin ? (
                  <div className="form-card">
                    <h2>Ajouter un lit</h2>
                    <div className="form-grid">
                      <label>
                        Numéro
                        <input className="form-control" value={newBed.number} type="number" min="1" onChange={(event) => setNewBed((current) => ({ ...current, number: event.target.value }))} />
                      </label>
                      <label>
                        Nom
                        <input className="form-control" value={newBed.name} type="text" onChange={(event) => setNewBed((current) => ({ ...current, name: event.target.value }))} />
                      </label>
                      <label>
                        Nom (autre langue)
                        <input className="form-control" value={newBed.nameAlt} type="text" onChange={(event) => setNewBed((current) => ({ ...current, nameAlt: event.target.value }))} />
                      </label>
                      <label>
                        Chambre
                        <input className="form-control" value={newBed.room} type="text" onChange={(event) => setNewBed((current) => ({ ...current, room: event.target.value }))} />
                      </label>
                      <label>
                        Chambre (autre langue)
                        <input className="form-control" value={newBed.roomAlt} type="text" onChange={(event) => setNewBed((current) => ({ ...current, roomAlt: event.target.value }))} />
                      </label>
                      <label>
                        Type
                        <select className="form-select" value={newBed.type} onChange={(event) => setNewBed((current) => ({ ...current, type: event.target.value }))}>
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
              <div className="grid">
                <BedsGrid
                  beds={beds}
                  statusMeta={statusMeta}
                  normalizeStatus={normalizeStatus}
                  escapeText={escapeText}
                  canManageBeds={canManageBeds}
                  isAdmin={isAdmin}
                  assignByBed={assignByBed}
                  setAssignByBed={setAssignByBed}
                  activePatients={activePatients}
                  bedEdits={bedEdits}
                  setBedEdits={setBedEdits}
                  api={api}
                  showError={showError}
                  showSuccess={showSuccess}
                  readErrorMessage={readErrorMessage}
                  setConfirm={setConfirm}
                />
              </div>
              <div className="foot">Liaison réseau en attente.</div>
            </div>
          ) : null}

          {screen === 'patients' && canManagePatients ? (
            <div className="screen active">
              <div className="patients-header-grid">
                <div className="form-card patient-command-card">
                  <div className="patient-command-head">
                    <h2>Poste de commande patients</h2>
                    <p className="small-note">Saisie prioritaire pour l'ajout, le triage et l'assignation.</p>
                  </div>
                  <div className="patient-priority-row">
                    <span className="patient-priority-chip">Actifs: {activePatients.length}</span>
                    <span className="patient-priority-chip warn">Non assignes: {activePatients.filter((patient) => !patient.bedNumber).length}</span>
                    <span className="patient-priority-chip danger">Triage critique: {activePatients.filter((patient) => triageLevelOf(patient) >= 3).length}</span>
                  </div>
                  <div className="form-grid patient-command-grid">
                    <label>
                      Numero
                      <input className="form-control" value={newPatient.registrationNumber} type="text" onChange={(event) => setNewPatient((current) => ({ ...current, registrationNumber: event.target.value }))} />
                    </label>
                    <label>
                      Nom
                      <input className="form-control" value={newPatient.name} type="text" onChange={(event) => setNewPatient((current) => ({ ...current, name: event.target.value }))} />
                    </label>
                    <label>
                      Lit
                      <input className="form-control" value={newPatient.bedNumber} type="number" min="1" disabled={isTriage} onChange={(event) => setNewPatient((current) => ({ ...current, bedNumber: event.target.value }))} />
                    </label>
                    <label>
                      Score triage (0-4)
                      <select className="form-select" value={newPatient.triageScore} onChange={(event) => setNewPatient((current) => ({ ...current, triageScore: event.target.value }))}>
                        <option value="0">0</option>
                        <option value="1">1</option>
                        <option value="2">2</option>
                        <option value="3">3</option>
                        <option value="4">4</option>
                      </select>
                    </label>
                    <button className="btn primary" type="button" onClick={savePatient}>Enregistrer patient</button>
                  </div>
                  {isTriage ? <p className="small-note">Le profil triage enregistre uniquement des patients non assignes.</p> : null}
                </div>
                <div className="form-card">
                  <h2>Actions critiques</h2>
                  <p className="small-note">Prioriser les patients avec triage 3-4 puis traiter les non assignes.</p>
                  <div className="patient-legend">
                    <span className="patient-legend-item"><span className="legend-dot triage-critical-dot" /> Triage eleve</span>
                    <span className="patient-legend-item"><span className="legend-dot triage-medium-dot" /> Triage modere</span>
                    <span className="patient-legend-item"><span className="legend-dot triage-low-dot" /> Triage bas</span>
                  </div>
                </div>
              </div>
              <div className="table-wrap patients-table-wrap">
                <table className="table table-sm align-middle mb-0 patients-table">
                  <thead>
                    <tr>
                      <th>Numéro d'inscription</th>
                      <th>Nom</th>
                      <th>Triage</th>
                      <th>Chambre / Lit</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    <PatientsRows
                      activePatients={activePatients}
                      canViewTriage={canViewTriage}
                      authenticated={authenticated}
                      canManageBeds={canManageBeds}
                      canArchivePatients={canArchivePatients}
                      setScreen={setScreen}
                      setNewPatient={setNewPatient}
                      setConfirm={setConfirm}
                      api={api}
                      showError={showError}
                      showSuccess={showSuccess}
                      readErrorMessage={readErrorMessage}
                      setPatients={setPatients}
                      speakPatientCall={speakPatientCall}
                      escapeText={escapeText}
                    />
                  </tbody>
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

          {screen === 'stats' && authenticated && isAdmin ? (
            <StatsScreen stats={stats} />
          ) : null}

          {screen === 'account' && authenticated ? (
            <AccountScreen
              passwordForm={passwordForm}
              setPasswordForm={setPasswordForm}
              changeOwnPassword={changeOwnPassword}
              user={user}
            />
          ) : null}

          {screen === 'settings' && authenticated && isAdmin ? (
            <SettingsScreen
              newUser={newUser}
              setNewUser={setNewUser}
              createUser={createUser}
              resetPasswordForm={resetPasswordForm}
              setResetPasswordForm={setResetPasswordForm}
              resetUserPassword={resetUserPassword}
              createBackup={createBackup}
              restoreLatestBackup={restoreLatestBackup}
              lastBackupFile={lastBackupFile}
              gotifyForm={gotifyForm}
              setGotifyForm={setGotifyForm}
              saveGotifySettings={saveGotifySettings}
              securityHealth={securityHealth}
              refreshSecurityHealth={refreshSecurityHealth}
              renderUsers={renderUsers}
              auditLogs={auditLogs}
              escapeText={escapeText}
            />
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
              <input className="form-control" value={authForm.username} type="text" onChange={(event) => setAuthForm((current) => ({ ...current, username: event.target.value }))} />
            </label>
            <label>
              Mot de passe
              <input className="form-control" value={authForm.password} type="password" onChange={(event) => setAuthForm((current) => ({ ...current, password: event.target.value }))} />
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
      <SimpleModal
        title={confirm.title}
        text={confirm.message}
        onCancel={closeConfirm}
        onConfirm={handleConfirmAction}
      />
      ) : null}
    </div>
  );
}

export default App;
