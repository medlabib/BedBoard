import { useEffect, useMemo, useState } from 'react';
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
import { isRtlLocale, normalizeLocale, patientStatusLabel, patientTypeLabel, roleLabel, tr } from './lib/i18n';

const initialStats = {
  totalBeds: 0,
  freeBeds: 0,
  occupiedBeds: 0,
  cleaningBeds: 0,
  alertBeds: 0,
  totalPatients: 0,
  archivedPatients: 0,
  consultationsByDate: [],
  consultationsByHour: [],
  avgConsultationMinutes: 0,
  avgWaitToTriageMinutes: 0,
  avgWaitToAssignMinutes: 0,
  totalConsultations: 0,
  triageByLevel: { 0: 0, 1: 0, 2: 0, 3: 0, 4: 0 },
  patientsByStatus: {},
  patientsByType: {},
  triageSlaBreaches: 0,
};

const statusMeta = {
  libre: { color: '#7ab893', soft: 'rgba(122, 184, 147, 0.16)' },
  occupé: { color: '#7fa7d4', soft: 'rgba(127, 167, 212, 0.16)' },
  nettoyage: { color: '#a58ac9', soft: 'rgba(165, 138, 201, 0.18)' },
  alerte: { color: '#d97a70', soft: 'rgba(217, 122, 112, 0.18)' },
};

const patientTypeOptions = ['all', 'traumato', 'medical', 'douleurs_thoracique', 'chirurgical', 'urgences_differees'];

const patientTypeSelectableOptions = patientTypeOptions.filter((option) => option !== 'all');

const defaultKeyboardShortcuts = {
  assignHighest: 'Space',
  assignOldest: 'Shift+Space',
  goBeds: 'KeyB',
  goPatients: 'KeyP',
  refresh: 'KeyR',
  help: 'Slash',
};

const allowedShortcutValues = new Set([
  'Space',
  'Shift+Space',
  'KeyB',
  'KeyP',
  'KeyR',
  'Slash',
  'KeyG',
  'KeyH',
  'KeyJ',
]);

function normalizeShortcutValue(value, fallback) {
  const clean = String(value || '').trim();
  return allowedShortcutValues.has(clean) ? clean : fallback;
}

function loadKeyboardShortcuts(username) {
  if (!username) return { ...defaultKeyboardShortcuts };
  try {
    const raw = localStorage.getItem(`bedboard_shortcuts_${username}`);
    const parsed = JSON.parse(raw || '{}');
    return {
      assignHighest: normalizeShortcutValue(parsed.assignHighest, defaultKeyboardShortcuts.assignHighest),
      assignOldest: normalizeShortcutValue(parsed.assignOldest, defaultKeyboardShortcuts.assignOldest),
      goBeds: normalizeShortcutValue(parsed.goBeds, defaultKeyboardShortcuts.goBeds),
      goPatients: normalizeShortcutValue(parsed.goPatients, defaultKeyboardShortcuts.goPatients),
      refresh: normalizeShortcutValue(parsed.refresh, defaultKeyboardShortcuts.refresh),
      help: normalizeShortcutValue(parsed.help, defaultKeyboardShortcuts.help),
    };
  } catch (error) {
    console.error(error);
    return { ...defaultKeyboardShortcuts };
  }
}


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

function normalizePatientTypeValue(value) {
  const key = String(value || '').trim().toLowerCase();
  if (key === 'traumato') return 'traumato';
  if (key === 'chirurgical') return 'chirurgical';
  if (key === 'douleurs_thoracique' || key === 'douleurs thoracique') return 'douleurs_thoracique';
  if (key === 'urgences_differees' || key === 'urgences differees' || key === 'urgence differee') return 'urgences_differees';
  return 'medical';
}

function parsePatientTime(patient) {
  const candidates = [patient?.arrivedAt, patient?.createdAt, patient?.updatedAt, patient?.assignedAt];
  for (const value of candidates) {
    const time = value ? new Date(value).getTime() : NaN;
    if (Number.isFinite(time) && time > 0) return time;
  }
  return Number.MAX_SAFE_INTEGER;
}

function loadPatientTypeFilter() {
  try {
    const raw = localStorage.getItem('bedboard_patient_type_filter');
    const allowed = new Set(['all', 'traumato', 'medical', 'douleurs_thoracique', 'chirurgical', 'urgences_differees']);
    if (allowed.has(raw)) return raw;
  } catch (error) {
    console.error(error);
  }
  return 'all';
}

function loadVisiblePatientTypes(username) {
  if (!username) return patientTypeSelectableOptions.map((option) => option);
  try {
    const raw = localStorage.getItem(`bedboard_visible_patient_types_${username}`);
    const parsed = JSON.parse(raw || '[]');
    if (!Array.isArray(parsed)) return patientTypeSelectableOptions.map((option) => option);
    const allowed = new Set(patientTypeSelectableOptions.map((option) => option));
    const clean = parsed
      .map((value) => normalizePatientTypeValue(value))
      .filter((value) => allowed.has(value));
    return clean.length ? Array.from(new Set(clean)) : patientTypeSelectableOptions.map((option) => option);
  } catch (error) {
    console.error(error);
    return patientTypeSelectableOptions.map((option) => option);
  }
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
  const [authResolved, setAuthResolved] = useState(false);
  const [user, setUser] = useState({ username: '', admin: false, role: 'user' });
  const [users, setUsers] = useState([]);
  const [screen, setScreen] = useState('beds');
  const [confirm, setConfirm] = useState({ open: false, title: '', message: '', onConfirm: null });
  const [authForm, setAuthForm] = useState({ username: 'admin', password: '' });
  const [authMessage, setAuthMessage] = useState('');
  const [connectionState, setConnectionState] = useState('');
  const [uiConfigForm, setUiConfigForm] = useState({ appName: 'BedBoard', logoDataUrl: '', locale: 'fr', patientViewIdentityMode: 'name', clearLogo: false });
  const [publicUiConfig, setPublicUiConfig] = useState({ appName: 'BedBoard', logoDataUrl: '', locale: 'fr', patientViewIdentityMode: 'name' });
  const [newBed, setNewBed] = useState({ number: '', room: '', name: '', type: 'standard' });
  const [newPatient, setNewPatient] = useState({ registrationNumber: '', name: '', patientType: 'medical', bedNumber: '', triageScore: '0', status: 'arrived', reason: '', destination: '', outcome: '' });
  const [patientTypeFilter, setPatientTypeFilter] = useState(() => loadPatientTypeFilter());
  const [visiblePatientTypes, setVisiblePatientTypes] = useState(() => patientTypeSelectableOptions.map((option) => option));
  const [newUser, setNewUser] = useState({ username: '', password: '', role: 'user' });
  const [assignByBed, setAssignByBed] = useState({});
  const [passwordForm, setPasswordForm] = useState({ currentPassword: '', newPassword: '', confirmPassword: '' });
  const [resetPasswordForm, setResetPasswordForm] = useState({ username: '', newPassword: '', confirmPassword: '' });
  const [gotifyForm, setGotifyForm] = useState({ enabled: false, url: '', token: '', priority: 8, tokenConfigured: false, clearToken: false });
  const [alertChannelsForm, setAlertChannelsForm] = useState({
    sms: { enabled: false, webhookUrl: '', recipient: '' },
    whatsapp: { enabled: false, webhookUrl: '', recipient: '' },
  });
  const [alertNotifications, setAlertNotifications] = useState([]);
  const [securityConfigForm, setSecurityConfigForm] = useState({
    adminInitUsername: 'admin',
    adminInitPassword: '',
    forceSecureCookie: true,
    trustProxyHeaders: true,
    enableHsts: true,
    hstsMaxAge: 31536000,
    hstsIncludeSubdomains: true,
    hstsPreload: false,
    gotifyTokenEncKey: '',
    proxyEnabled: false,
    proxyUrl: '',
    proxyUsername: '',
    proxyPassword: '',
    adminInitPasswordConfigured: false,
    gotifyTokenEncKeyConfigured: false,
    proxyPasswordConfigured: false,
    alertCallbackSignatureRequired: true,
    alertCallbackSecret: '',
    alertCallbackSecretConfigured: false,
    alertCallbackIpAllowlist: '',
    triageSlaMinutes: 15,
    clearAdminInitPassword: false,
    clearGotifyTokenEncKey: false,
    clearProxyPassword: false,
    clearAlertCallbackSecret: false,
  });
  const [securityHealth, setSecurityHealth] = useState({ status: 'unknown', checks: [], loaded: false });
  const [bedEdits, setBedEdits] = useState({});
  const [auditLogs, setAuditLogs] = useState([]);
  const [patientEvents, setPatientEvents] = useState([]);
  const [selectedPatientReg, setSelectedPatientReg] = useState('');
  const [patientImportForm, setPatientImportForm] = useState({ source: 'manual', json: '' });
  const [lastBackupFile, setLastBackupFile] = useState('');
  const [notice, setNotice] = useState({ type: '', text: '' });
  const [commandPaletteOpen, setCommandPaletteOpen] = useState(false);
  const [commandQuery, setCommandQuery] = useState('');
  const [keyboardShortcuts, setKeyboardShortcuts] = useState({ ...defaultKeyboardShortcuts });

  const isAdmin = user.admin;
  const locale = normalizeLocale(publicUiConfig.locale);
  const brandName = String(publicUiConfig.appName || 'BedBoard');
  const brandLogo = String(publicUiConfig.logoDataUrl || '/logo.svg');
  const patientViewIdentityMode = String(publicUiConfig.patientViewIdentityMode || 'name') === 'number' ? 'number' : 'name';
  const role = normalizeRole(user.role);
  const isReception = role === 'reception';
  const isTriage = role === 'triage';
  const isDechocage = role === 'dechocage';
  const canManageBeds = authenticated && (role === 'admin' || role === 'user' || role === 'dechocage');
  const canManagePatients = authenticated && (role === 'admin' || role === 'user' || role === 'triage' || role === 'dechocage');
  const canArchivePatients = authenticated && (role === 'admin' || role === 'user' || role === 'dechocage');
  const canCustomizePatientTypeVisibility = authenticated && (role === 'admin' || role === 'user' || role === 'dechocage');
  const canViewTriage = authenticated && !isReception;
  const canViewPatientType = authenticated && !isReception && !isPatientPage;
  const securityNavStatus = String(securityHealth?.status || 'unknown').toLowerCase();
  const localizedStatusMeta = useMemo(() => ({
    libre: { ...statusMeta.libre, label: tr(locale, 'Libre', 'Free', 'شاغر') },
    occupé: { ...statusMeta.occupé, label: tr(locale, 'Occupe', 'Occupied', 'مشغول') },
    nettoyage: { ...statusMeta.nettoyage, label: tr(locale, 'Nettoyage', 'Cleaning', 'تنظيف') },
    alerte: { ...statusMeta.alerte, label: tr(locale, 'Alerte', 'Alert', 'إنذار') },
  }), [locale]);

  useEffect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dir = isRtlLocale(locale) ? 'rtl' : 'ltr';
  }, [locale]);

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
      showError(tr(locale, 'Action impossible pour le moment.', 'Action unavailable right now.', 'الإجراء غير متاح حاليًا.'));
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

  const refreshPublicUIConfig = async () => {
    const response = await fetch('/api/public/ui-config', { credentials: 'include' });
    if (!response.ok) return;
    const data = await response.json().catch(() => ({}));
    setPublicUiConfig({
      appName: data.appName || 'BedBoard',
      logoDataUrl: data.logoDataUrl || '',
      locale: normalizeLocale(data.locale),
      patientViewIdentityMode: String(data.patientViewIdentityMode || 'name') === 'number' ? 'number' : 'name',
    });
  };

  const refreshUIConfig = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/ui/config', { method: 'GET' });
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Lecture configuration interface impossible.', 'Unable to load UI configuration.', 'تعذر قراءة إعدادات الواجهة.')));
      return;
    }
    const data = await response.json().catch(() => ({}));
    setUiConfigForm({
      appName: data.appName || 'BedBoard',
      logoDataUrl: data.logoDataUrl || '',
      locale: normalizeLocale(data.locale),
      patientViewIdentityMode: String(data.patientViewIdentityMode || 'name') === 'number' ? 'number' : 'name',
      clearLogo: false,
    });
  };

  const saveUiConfig = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/ui/config', {
      method: 'POST',
      body: JSON.stringify({
        appName: uiConfigForm.appName,
        logoDataUrl: uiConfigForm.logoDataUrl,
        locale: uiConfigForm.locale,
        patientViewIdentityMode: uiConfigForm.patientViewIdentityMode,
        clearLogo: Boolean(uiConfigForm.clearLogo),
      }),
    });
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Enregistrement configuration interface impossible.', 'Unable to save UI configuration.', 'تعذر حفظ إعدادات الواجهة.')));
      return;
    }
    await refreshPublicUIConfig();
    await refreshUIConfig();
    showSuccess(tr(locale, 'Configuration interface enregistree.', 'UI configuration saved.', 'تم حفظ إعدادات الواجهة.'));
  };

  const syncMe = async () => {
    const response = await fetch('/api/me', { credentials: 'include' });
    const data = await response.json().catch(() => ({ authenticated: false, username: '', admin: false, role: 'user' }));
    setAuthenticated(Boolean(data.authenticated));
    setUser({ username: data.username || '', admin: Boolean(data.admin), role: normalizeRole(data.role) });
    if (data.authenticated && data.username) {
      setConnectionState(`${tr(locale, 'Connecte', 'Connected', 'متصل')}: ${data.username}`);
    } else {
      setConnectionState(tr(locale, 'Acces local', 'Local access', 'وصول محلي'));
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
    setAuthResolved(true);
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

  const exportAuditCsv = () => {
    window.location.assign('/api/admin/audit/export');
  };

  const exportFHIRBundle = () => {
    if (!isAdmin) return;
    window.location.assign('/api/admin/integrations/fhir/export');
  };

  const refreshPatientEvents = async (registrationNumber) => {
    const reg = String(registrationNumber || '').trim();
    if (!reg) {
      setPatientEvents([]);
      return;
    }
    const response = await api(`/api/patients/events?registrationNumber=${encodeURIComponent(reg)}`);
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Lecture timeline patient impossible.', 'Unable to load patient timeline.', 'تعذر قراءة التسلسل الزمني للمريض.')));
      return;
    }
    const data = await response.json().catch(() => ({ events: [] }));
    setPatientEvents(Array.isArray(data.events) ? data.events : []);
  };

  const importPatients = async () => {
    if (!isAdmin) return;
    let parsed;
    try {
      parsed = JSON.parse(patientImportForm.json || '{}');
    } catch (error) {
      showError(tr(locale, 'JSON import invalide.', 'Invalid import JSON.', 'تنسيق JSON غير صالح للاستيراد.'));
      return;
    }
    const body = {
      source: patientImportForm.source || 'manual',
      patients: Array.isArray(parsed?.patients) ? parsed.patients : [],
    };
    if (!Array.isArray(body.patients) || !body.patients.length) {
      showError(tr(locale, 'Le JSON doit contenir un tableau patients non vide.', 'JSON must include a non-empty patients array.', 'يجب أن يحتوي JSON على مصفوفة مرضى غير فارغة.'));
      return;
    }
    const response = await api('/api/admin/integrations/patients/import', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
      showError((data && data.error) || tr(locale, 'Import patients impossible.', 'Unable to import patients.', 'تعذر استيراد المرضى.'));
      return;
    }
    await refreshState();
    showSuccess(`${tr(locale, 'Import termine', 'Import completed', 'اكتمل الاستيراد')} (${patientImportForm.source || 'manual'}): ${data.processed || 0} ${tr(locale, 'reussis', 'successful', 'ناجح')}, ${data.failed || 0} ${tr(locale, 'echecs', 'failed', 'فاشل')}.`);
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
      showError(await readErrorMessage(response, tr(locale, 'Enregistrement Gotify impossible.', 'Unable to save Gotify settings.', 'تعذر حفظ إعدادات Gotify.')));
      return;
    }
    await refreshGotifySettings();
    showSuccess(tr(locale, 'Configuration Gotify enregistree.', 'Gotify settings saved.', 'تم حفظ إعدادات Gotify.'));
  };

  const testGotifySettings = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/integrations/gotify/test', {
      method: 'POST',
      body: JSON.stringify({}),
    });
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Test Gotify impossible.', 'Unable to send Gotify test.', 'تعذر إرسال اختبار Gotify.')));
      return;
    }
    showSuccess(tr(locale, 'Notification test Gotify envoyee.', 'Gotify test notification sent.', 'تم إرسال إشعار اختبار Gotify.'));
  };

  const refreshAlertChannelsSettings = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/integrations/alerts/channels', { method: 'GET' });
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Lecture configuration canaux impossible.', 'Unable to load channel settings.', 'تعذر تحميل إعدادات القنوات.')));
      return;
    }
    const data = await response.json().catch(() => ({}));
    setAlertChannelsForm({
      sms: {
        enabled: Boolean(data?.sms?.enabled),
        webhookUrl: data?.sms?.webhookUrl || '',
        recipient: data?.sms?.recipient || '',
      },
      whatsapp: {
        enabled: Boolean(data?.whatsapp?.enabled),
        webhookUrl: data?.whatsapp?.webhookUrl || '',
        recipient: data?.whatsapp?.recipient || '',
      },
    });
  };

  const saveAlertChannelsSettings = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/integrations/alerts/channels', {
      method: 'POST',
      body: JSON.stringify(alertChannelsForm),
    });
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Enregistrement canaux impossible.', 'Unable to save channel settings.', 'تعذر حفظ إعدادات القنوات.')));
      return;
    }
    await refreshAlertChannelsSettings();
    await refreshAlertNotifications({ announce: true });
    showSuccess(tr(locale, 'Canaux SMS/WhatsApp enregistres.', 'SMS/WhatsApp channels saved.', 'تم حفظ قنوات SMS/WhatsApp.'));
  };

  const testAlertChannelsSettings = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/integrations/alerts/channels/test', {
      method: 'POST',
      body: JSON.stringify({}),
    });
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Test canaux impossible.', 'Unable to test channels.', 'تعذر اختبار القنوات.')));
      return;
    }
    await refreshAlertNotifications({ announce: true });
    showSuccess(tr(locale, 'Test SMS/WhatsApp envoye.', 'SMS/WhatsApp test sent.', 'تم إرسال اختبار SMS/WhatsApp.'));
  };

  const refreshAlertNotifications = async (options = {}) => {
    const announce = Boolean(options.announce);
    if (!isAdmin) {
      setAlertNotifications([]);
      return;
    }
    const response = await api('/api/admin/integrations/alerts/notifications', { method: 'GET' });
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Lecture notifications impossible.', 'Unable to load notifications.', 'تعذر تحميل الإشعارات.')));
      return;
    }
    const data = await response.json().catch(() => ({}));
    const items = Array.isArray(data.items) ? data.items : [];
    setAlertNotifications(items);
    if (announce) {
      const failed = items.filter((item) => String(item.status || '').toLowerCase() === 'failed').length;
      const pending = items.filter((item) => String(item.status || '').toLowerCase() === 'pending').length;
      const acknowledged = items.filter((item) => String(item.status || '').toLowerCase() === 'acknowledged').length;
      if (failed > 0) {
        showError(tr(locale, `Canaux alertes: ${failed} en echec, ${pending} en attente, ${acknowledged} accuses.`, `Alert channels: ${failed} failed, ${pending} pending, ${acknowledged} acknowledged.`, `قنوات التنبيه: ${failed} فاشلة، ${pending} قيد الانتظار، ${acknowledged} مؤكدة.`));
      } else {
        showSuccess(tr(locale, `Canaux alertes: ${pending} en attente, ${acknowledged} accuses.`, `Alert channels: ${pending} pending, ${acknowledged} acknowledged.`, `قنوات التنبيه: ${pending} قيد الانتظار، ${acknowledged} مؤكدة.`));
      }
    }
  };

  const acknowledgeAlertNotification = async (id) => {
    if (!isAdmin) return;
    const response = await api('/api/admin/integrations/alerts/notifications/ack', {
      method: 'POST',
      body: JSON.stringify({ id: Number(id) }),
    });
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Accuse reception impossible.', 'Unable to acknowledge notification.', 'تعذر تأكيد استلام الإشعار.')));
      return;
    }
    await refreshAlertNotifications({ announce: false });
    showSuccess(tr(locale, 'Notification accusee.', 'Notification acknowledged.', 'تم تأكيد الإشعار.'));
  };

  const acknowledgeAllAlertNotifications = async () => {
    if (!isAdmin) return;
    const pending = (Array.isArray(alertNotifications) ? alertNotifications : [])
      .filter((entry) => String(entry.status || '').toLowerCase() !== 'acknowledged');
    if (!pending.length) {
      showSuccess(tr(locale, 'Aucune notification en attente.', 'No pending notifications.', 'لا توجد إشعارات معلقة.'));
      return;
    }
    let failed = 0;
    for (const entry of pending) {
      const response = await api('/api/admin/integrations/alerts/notifications/ack', {
        method: 'POST',
        body: JSON.stringify({ id: Number(entry.id) }),
      });
      if (!response.ok) {
        failed += 1;
      }
    }
    await refreshAlertNotifications({ announce: false });
    if (failed > 0) {
      showError(tr(locale, `${failed} accusés ont échoué.`, `${failed} acknowledgments failed.`, `فشل ${failed} من عمليات التأكيد.`));
      return;
    }
    showSuccess(tr(locale, `${pending.length} notifications accusées.`, `${pending.length} notifications acknowledged.`, `تم تأكيد ${pending.length} إشعارًا.`));
  };

  const refreshSecurityConfig = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/security/config', { method: 'GET' });
    if (!response.ok) {
      showError(await readErrorMessage(response, 'Lecture configuration sécurité impossible.'));
      return;
    }
    const data = await response.json().catch(() => ({}));
    setSecurityConfigForm((current) => ({
      ...current,
      adminInitUsername: data.adminInitUsername || 'admin',
      adminInitPassword: '',
      forceSecureCookie: Boolean(data.forceSecureCookie),
      trustProxyHeaders: Boolean(data.trustProxyHeaders),
      enableHsts: Boolean(data.enableHsts),
      hstsMaxAge: Number(data.hstsMaxAge || 31536000),
      hstsIncludeSubdomains: Boolean(data.hstsIncludeSubdomains),
      hstsPreload: Boolean(data.hstsPreload),
      gotifyTokenEncKey: '',
      proxyEnabled: Boolean(data.proxyEnabled),
      proxyUrl: data.proxyUrl || '',
      proxyUsername: data.proxyUsername || '',
      proxyPassword: '',
      adminInitPasswordConfigured: Boolean(data.adminInitPasswordConfigured),
      gotifyTokenEncKeyConfigured: Boolean(data.gotifyTokenEncKeyConfigured),
      proxyPasswordConfigured: Boolean(data.proxyPasswordConfigured),
      alertCallbackSignatureRequired: Boolean(data.alertCallbackSignatureRequired),
      alertCallbackSecret: '',
      alertCallbackSecretConfigured: Boolean(data.alertCallbackSecretConfigured),
      alertCallbackIpAllowlist: data.alertCallbackIpAllowlist || '',
      triageSlaMinutes: Number(data.triageSlaMinutes || 15),
      clearAdminInitPassword: false,
      clearGotifyTokenEncKey: false,
      clearProxyPassword: false,
      clearAlertCallbackSecret: false,
    }));
  };

  const saveSecurityConfig = async () => {
    if (!isAdmin) return;
    const response = await api('/api/admin/security/config', {
      method: 'POST',
      body: JSON.stringify({
        adminInitUsername: securityConfigForm.adminInitUsername,
        adminInitPassword: securityConfigForm.adminInitPassword,
        forceSecureCookie: Boolean(securityConfigForm.forceSecureCookie),
        trustProxyHeaders: Boolean(securityConfigForm.trustProxyHeaders),
        enableHsts: Boolean(securityConfigForm.enableHsts),
        hstsMaxAge: Number(securityConfigForm.hstsMaxAge || 0),
        hstsIncludeSubdomains: Boolean(securityConfigForm.hstsIncludeSubdomains),
        hstsPreload: Boolean(securityConfigForm.hstsPreload),
        gotifyTokenEncKey: securityConfigForm.gotifyTokenEncKey,
        proxyEnabled: Boolean(securityConfigForm.proxyEnabled),
        proxyUrl: securityConfigForm.proxyUrl,
        proxyUsername: securityConfigForm.proxyUsername,
        proxyPassword: securityConfigForm.proxyPassword,
        alertCallbackSignatureRequired: Boolean(securityConfigForm.alertCallbackSignatureRequired),
        alertCallbackSecret: securityConfigForm.alertCallbackSecret,
        alertCallbackIpAllowlist: securityConfigForm.alertCallbackIpAllowlist,
        triageSlaMinutes: Number(securityConfigForm.triageSlaMinutes || 15),
        clearAdminInitPassword: Boolean(securityConfigForm.clearAdminInitPassword),
        clearGotifyTokenEncKey: Boolean(securityConfigForm.clearGotifyTokenEncKey),
        clearProxyPassword: Boolean(securityConfigForm.clearProxyPassword),
        clearAlertCallbackSecret: Boolean(securityConfigForm.clearAlertCallbackSecret),
      }),
    });
    if (!response.ok) {
      showError(await readErrorMessage(response, 'Enregistrement configuration sécurité impossible.'));
      return;
    }
    await refreshSecurityConfig();
    await refreshSecurityHealth();
    showSuccess('Configuration sécurité enregistrée.');
  };

  const refreshSecurityHealth = async () => {
    if (!isAdmin) {
      setSecurityHealth({ status: 'unknown', checks: [], loaded: false });
      return;
    }
    const response = await api('/api/admin/security/health', { method: 'GET' });
    if (!response.ok) {
      const fallback = await readErrorMessage(response, tr(locale, 'Lecture audit securite impossible.', 'Unable to load security audit.', 'تعذر قراءة تدقيق الأمان.'));
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
    setConnectionState(tr(locale, 'Connexion locale...', 'Connecting locally...', 'جار الاتصال محليًا...'));

    let refreshTimer = null;
    const scheduleRefresh = () => {
      if (refreshTimer) return;
      refreshTimer = setTimeout(() => {
        refreshTimer = null;
        refreshState().catch(() => {
          setConnectionState(tr(locale, 'Synchronisation locale interrompue.', 'Local synchronization interrupted.', 'توقفت المزامنة المحلية.'));
        });
      }, 120);
    };

    stream.addEventListener('state.snapshot', (event) => {
      try {
        const data = JSON.parse(event.data);
        setBeds(Array.isArray(data.beds) ? data.beds : []);
        setPatients(Array.isArray(data.patients) ? data.patients : []);
        setStats(data.stats || initialStats);
        setConnectionState(tr(locale, 'Connecte.', 'Connected.', 'متصل.'));
      } catch (error) {
        console.error(error);
      }
    });

    ['state.changed', 'bed.updated', 'bed.created', 'bed.deleted', 'patient.updated', 'patient.archived', 'user.updated', 'system.backup', 'system.restore', 'system.settings'].forEach((eventName) => {
      stream.addEventListener(eventName, (event) => {
        scheduleRefresh();
        if (eventName === 'user.updated') {
          refreshUsers().catch(() => {});
        }
        if (eventName === 'system.settings') {
          refreshPublicUIConfig().catch(() => {});
          try {
            const payload = JSON.parse(event?.data || '{}');
            const scope = String(payload.scope || '').trim();
            if (scope === 'integrations.alert_channels') {
              refreshAlertNotifications({ announce: true }).catch(() => {});
              showSuccess(tr(locale, 'Canaux alertes mis a jour.', 'Alert channels updated.', 'تم تحديث قنوات التنبيه.'));
            }
          } catch (error) {
            console.error(error);
          }
        }
        if (eventName === 'system.restore' || eventName === 'bed.updated') {
          refreshAudit().catch(() => {});
        }
        if (eventName === 'system.backup') {
          try {
            const payload = JSON.parse(event?.data || '{}');
            const file = String(payload.file || '').trim();
            if (file) {
              showSuccess(tr(locale, `Sauvegarde terminee: ${file}`, `Backup completed: ${file}`, `اكتمل النسخ الاحتياطي: ${file}`));
            }
          } catch (error) {
            console.error(error);
          }
        }
        if (eventName === 'system.restore') {
          try {
            const payload = JSON.parse(event?.data || '{}');
            const file = String(payload.file || '').trim();
            if (file) {
              showSuccess(tr(locale, `Restauration appliquee: ${file}`, `Restore applied: ${file}`, `تم تطبيق الاستعادة: ${file}`));
            }
          } catch (error) {
            console.error(error);
          }
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
          setConnectionState(tr(locale, 'Connecte.', 'Connected.', 'متصل.'));
        }
      } catch (error) {
        console.error(error);
      }
    };

    stream.onerror = () => {
      setConnectionState(tr(locale, 'Connexion interrompue.', 'Connection interrupted.', 'انقطع الاتصال.'));
    };

    return stream;
  };

  useEffect(() => {
    let stream;
    refreshPublicUIConfig().finally(() => {
      syncMe().finally(() => {
        stream = connectStream();
      });
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

  useEffect(() => {
    localStorage.setItem('bedboard_patient_type_filter', patientTypeFilter);
  }, [patientTypeFilter]);

  useEffect(() => {
    setAuthMessage(tr(locale, 'Connectez-vous pour gerer les lits et les patients sur ce poste.', 'Sign in to manage beds and patients on this station.', 'سجّل الدخول لإدارة الأسرة والمرضى في هذه المحطة.'));
    if (!authenticated) {
      setConnectionState(tr(locale, 'Acces local', 'Local access', 'وصول محلي'));
    }
  }, [locale, authenticated]);

  useEffect(() => {
    if (!canCustomizePatientTypeVisibility || !user.username) {
      setVisiblePatientTypes(patientTypeSelectableOptions.map((option) => option));
      return;
    }
    setVisiblePatientTypes(loadVisiblePatientTypes(user.username));
  }, [canCustomizePatientTypeVisibility, user.username]);

  useEffect(() => {
    if (!canCustomizePatientTypeVisibility || !user.username) return;
    localStorage.setItem(`bedboard_visible_patient_types_${user.username}`, JSON.stringify(visiblePatientTypes));
  }, [canCustomizePatientTypeVisibility, user.username, visiblePatientTypes]);

  useEffect(() => {
    if (!authenticated || !user.username) {
      setKeyboardShortcuts({ ...defaultKeyboardShortcuts });
      return;
    }
    setKeyboardShortcuts(loadKeyboardShortcuts(user.username));
  }, [authenticated, user.username]);

  useEffect(() => {
    if (!authenticated || !user.username) return;
    localStorage.setItem(`bedboard_shortcuts_${user.username}`, JSON.stringify(keyboardShortcuts));
  }, [authenticated, user.username, keyboardShortcuts]);

  const activePatients = useMemo(() => patients.filter((p) => p.status !== 'archived'), [patients]);
  const visiblePatientTypeSet = useMemo(() => new Set(visiblePatientTypes), [visiblePatientTypes]);
  const visiblePatients = useMemo(() => {
    if (!canCustomizePatientTypeVisibility) return activePatients;
    return activePatients.filter((patient) => visiblePatientTypeSet.has(normalizePatientTypeValue(patient.patientType)));
  }, [activePatients, canCustomizePatientTypeVisibility, visiblePatientTypeSet]);
  const filteredPatients = useMemo(() => {
    if (!canCustomizePatientTypeVisibility || patientTypeFilter === 'all') return visiblePatients;
    return visiblePatients.filter((patient) => normalizePatientTypeValue(patient.patientType) === patientTypeFilter);
  }, [canCustomizePatientTypeVisibility, visiblePatients, patientTypeFilter]);

  const keyboardCategory = useMemo(() => (patientTypeFilter === 'all' ? 'all' : normalizePatientTypeValue(patientTypeFilter)), [patientTypeFilter]);

  const assignmentCandidates = useMemo(() => visiblePatients
    .filter((patient) => !patient.bedNumber)
    .filter((patient) => keyboardCategory === 'all' || normalizePatientTypeValue(patient.patientType) === keyboardCategory), [visiblePatients, keyboardCategory]);

  const highestTriageCandidate = useMemo(() => {
    if (!assignmentCandidates.length) return null;
    return [...assignmentCandidates].sort((a, b) => {
      const triageDiff = triageLevelOf(b) - triageLevelOf(a);
      if (triageDiff !== 0) return triageDiff;
      return parsePatientTime(a) - parsePatientTime(b);
    })[0];
  }, [assignmentCandidates]);

  const oldestUnseenCandidate = useMemo(() => {
    if (!assignmentCandidates.length) return null;
    const unseen = assignmentCandidates.filter((patient) => {
      const status = String(patient.status || '').toLowerCase();
      return status === 'arrived' || status === 'triaged' || status === 'waiting' || !status;
    });
    const pool = unseen.length ? unseen : assignmentCandidates;
    return [...pool].sort((a, b) => parsePatientTime(a) - parsePatientTime(b))[0] || null;
  }, [assignmentCandidates]);

  const triageSlaMinutes = useMemo(() => {
    const raw = Number(securityConfigForm?.triageSlaMinutes);
    return Number.isFinite(raw) && raw > 0 ? raw : 15;
  }, [securityConfigForm?.triageSlaMinutes]);

  const waitingMinutes = (patient) => {
    const now = Date.now();
    const started = parsePatientTime(patient);
    if (!Number.isFinite(started) || started === Number.MAX_SAFE_INTEGER) return 0;
    return Math.max(0, Math.round((now - started) / 60000));
  };

  const smartPriorityList = useMemo(() => {
    const candidates = visiblePatients.filter((patient) => String(patient.status || '').toLowerCase() !== 'archived');
    return candidates.map((patient) => {
      const triage = triageLevelOf(patient);
      const wait = waitingMinutes(patient);
      const notAssignedBoost = patient.bedNumber ? 0 : 6;
      const slaBoost = triage >= 4 && wait > triageSlaMinutes ? 12 : 0;
      const score = triage * 10 + Math.min(wait, 45) + notAssignedBoost + slaBoost;
      return {
        patient,
        score,
        wait,
        slaRisk: triage >= 4 && wait > triageSlaMinutes,
      };
    }).sort((a, b) => b.score - a.score);
  }, [visiblePatients, triageSlaMinutes]);

  const nextToCall = smartPriorityList[0] || null;

  const firstFreeBed = useMemo(() => {
    const freeBeds = beds.filter((bed) => normalizeStatus(bed.status) === 'libre');
    if (!freeBeds.length) return null;
    return [...freeBeds].sort((a, b) => Number(a.number) - Number(b.number))[0];
  }, [beds]);

  const toggleVisiblePatientType = (patientType) => {
    setVisiblePatientTypes((current) => {
      if (current.includes(patientType)) {
        if (current.length <= 1) {
          showError(tr(locale, 'Selectionnez au moins un type patient.', 'Select at least one patient type.', 'اختر نوع مريض واحدًا على الأقل.'));
          return current;
        }
        return current.filter((value) => value !== patientType);
      }
      return [...current, patientType];
    });
  };

  const assignPatientToBed = async (registrationNumber, bedNumber) => {
    if (!canManageBeds || !canManagePatients) return false;
    const reg = String(registrationNumber || '').trim();
    const bedNum = Number(bedNumber);
    if (!reg || !bedNum) return false;
    const selectedPatient = activePatients.find((p) => p.registrationNumber === reg);
    const response = await api('/api/patients', {
      method: 'POST',
      body: JSON.stringify({ registrationNumber: reg, name: selectedPatient?.name || '', bedNumber: bedNum }),
    });
    if (!response.ok) {
      showError(await readErrorMessage(response, tr(locale, 'Affectation impossible.', 'Unable to assign.', 'تعذر التخصيص.')));
      return false;
    }
    setAssignByBed((current) => ({ ...current, [bedNum]: '' }));
    showSuccess(`${tr(locale, 'Affectation terminee', 'Assignment completed', 'اكتمل التخصيص')}: ${reg} -> ${tr(locale, 'lit', 'bed', 'سرير')} ${bedNum}.`);
    return true;
  };

  const commandItems = useMemo(() => {
    const items = [
      {
        id: 'nav-beds',
        label: tr(locale, 'Aller a Lits', 'Go to Beds', 'الانتقال إلى الأسرة'),
        keywords: 'beds lits',
        action: () => setScreen('beds'),
      },
      {
        id: 'nav-patients',
        label: tr(locale, 'Aller a Patients', 'Go to Patients', 'الانتقال إلى المرضى'),
        keywords: 'patients triage waiting',
        action: () => setScreen('patients'),
      },
      {
        id: 'refresh-state',
        label: tr(locale, 'Rafraichir donnees', 'Refresh data', 'تحديث البيانات'),
        keywords: 'refresh reload',
        action: async () => {
          await refreshState();
          showSuccess(tr(locale, 'Donnees rafraichies.', 'Data refreshed.', 'تم تحديث البيانات.'));
        },
      },
      {
        id: 'assign-highest',
        label: tr(locale, 'Affecter triage le plus eleve', 'Assign highest triage', 'تخصيص أعلى فرز'),
        keywords: 'assign triage highest urgent',
        action: async () => {
          if (!firstFreeBed || !highestTriageCandidate?.registrationNumber) return;
          await assignPatientToBed(highestTriageCandidate.registrationNumber, firstFreeBed.number);
        },
      },
      {
        id: 'assign-oldest',
        label: tr(locale, 'Affecter plus ancien non vu', 'Assign oldest unseen', 'تخصيص الأقدم غير المرئي'),
        keywords: 'assign oldest unseen waiting',
        action: async () => {
          if (!firstFreeBed || !oldestUnseenCandidate?.registrationNumber) return;
          await assignPatientToBed(oldestUnseenCandidate.registrationNumber, firstFreeBed.number);
        },
      },
    ];

    if (isAdmin) {
      items.push({
        id: 'nav-settings',
        label: tr(locale, 'Aller a Parametres', 'Go to Settings', 'الانتقال إلى الإعدادات'),
        keywords: 'settings admin',
        action: async () => {
          setScreen('settings');
          await Promise.all([
            refreshUsers(),
            refreshAudit(true),
            refreshGotifySettings(),
            refreshAlertChannelsSettings(),
            refreshAlertNotifications({ announce: false }),
            refreshSecurityConfig(),
            refreshSecurityHealth(),
            refreshUIConfig(),
          ]);
        },
      });
    }

    const patientItems = visiblePatients.slice(0, 50).map((patient) => ({
      id: `patient-${patient.registrationNumber}`,
      label: `${tr(locale, 'Dossier', 'Case', 'حالة')}: ${patient.registrationNumber} ${patient.name ? `- ${patient.name}` : ''}`,
      keywords: `${patient.registrationNumber} ${patient.name || ''} ${patient.patientType || ''}`,
      action: () => {
        setScreen('patients');
        setSelectedPatientReg(patient.registrationNumber);
        refreshPatientEvents(patient.registrationNumber).catch(() => {});
      },
    }));

    const bedItems = beds.slice(0, 50).map((bed) => ({
      id: `bed-${bed.number}`,
      label: `${tr(locale, 'Lit', 'Bed', 'سرير')} ${bed.number} - ${bed.room || '-'} ${bed.name || ''}`,
      keywords: `${bed.number} ${bed.room || ''} ${bed.name || ''} ${bed.status || ''}`,
      action: () => setScreen('beds'),
    }));

    return [...items, ...patientItems, ...bedItems];
  }, [
    locale,
    isAdmin,
    visiblePatients,
    beds,
    highestTriageCandidate,
    oldestUnseenCandidate,
    firstFreeBed,
  ]);

  const filteredCommandItems = useMemo(() => {
    const query = String(commandQuery || '').toLowerCase().trim();
    if (!query) return commandItems.slice(0, 20);
    return commandItems.filter((item) => {
      const haystack = `${item.label} ${item.keywords || ''}`.toLowerCase();
      return haystack.includes(query);
    }).slice(0, 20);
  }, [commandItems, commandQuery]);

  useEffect(() => {
    const shortcutMatchesEvent = (event, shortcut) => {
      if (event.ctrlKey || event.metaKey || event.altKey) return false;
      switch (shortcut) {
        case 'Space':
          return event.code === 'Space' && !event.shiftKey;
        case 'Shift+Space':
          return event.code === 'Space' && event.shiftKey;
        default:
          return event.code === shortcut;
      }
    };

    const isEditableTarget = (target) => {
      if (!target) return false;
      const tag = String(target.tagName || '').toLowerCase();
      return tag === 'input' || tag === 'textarea' || tag === 'select' || Boolean(target.isContentEditable);
    };

    const onKeyDown = (event) => {
      if (!authenticated) return;
      if (isEditableTarget(event.target)) return;
      if (event.repeat) return;

      if ((event.ctrlKey || event.metaKey) && event.code === 'KeyK') {
        event.preventDefault();
        setCommandPaletteOpen((current) => !current);
        if (!commandPaletteOpen) {
          setCommandQuery('');
        }
        return;
      }

      if (commandPaletteOpen) {
        if (event.code === 'Escape') {
          event.preventDefault();
          setCommandPaletteOpen(false);
          return;
        }
        if (event.code === 'Enter' && filteredCommandItems.length) {
          event.preventDefault();
          const action = filteredCommandItems[0]?.action;
          setCommandPaletteOpen(false);
          if (action) {
            Promise.resolve(action()).catch(() => {});
          }
          return;
        }
      }

      if (isPatientPage || !canManageBeds || !canManagePatients) return;

      if (shortcutMatchesEvent(event, keyboardShortcuts.assignHighest)) {
        event.preventDefault();
        if (!firstFreeBed) {
          showError(tr(locale, 'Aucun lit libre pour affectation rapide.', 'No free bed for quick assignment.', 'لا يوجد سرير شاغر للتخصيص السريع.'));
          return;
        }
        if (!highestTriageCandidate?.registrationNumber) {
          showError(tr(locale, 'Aucun dossier disponible dans la categorie selectionnee.', 'No record available in selected category.', 'لا توجد حالة متاحة في الفئة المحددة.'));
          return;
        }
        const modeLabel = tr(locale, 'triage le plus eleve', 'highest triage', 'أعلى فرز');
        assignPatientToBed(highestTriageCandidate.registrationNumber, firstFreeBed.number).then((ok) => {
          if (!ok) return;
          showSuccess(`${tr(locale, 'Raccourci applique', 'Shortcut applied', 'تم تطبيق الاختصار')} (${modeLabel})`);
        }).catch(() => {});
        return;
      }

      if (shortcutMatchesEvent(event, keyboardShortcuts.assignOldest)) {
        event.preventDefault();
        if (!firstFreeBed) {
          showError(tr(locale, 'Aucun lit libre pour affectation rapide.', 'No free bed for quick assignment.', 'لا يوجد سرير شاغر للتخصيص السريع.'));
          return;
        }
        if (!oldestUnseenCandidate?.registrationNumber) {
          showError(tr(locale, 'Aucun dossier disponible dans la categorie selectionnee.', 'No record available in selected category.', 'لا توجد حالة متاحة في الفئة المحددة.'));
          return;
        }
        const modeLabel = tr(locale, 'plus ancien non vu', 'oldest unseen', 'الأقدم غير المرئي');
        assignPatientToBed(oldestUnseenCandidate.registrationNumber, firstFreeBed.number).then((ok) => {
          if (!ok) return;
          showSuccess(`${tr(locale, 'Raccourci applique', 'Shortcut applied', 'تم تطبيق الاختصار')} (${modeLabel})`);
        }).catch(() => {});
        return;
      }

      if (shortcutMatchesEvent(event, keyboardShortcuts.goBeds)) {
        event.preventDefault();
        setScreen('beds');
        return;
      }

      if (shortcutMatchesEvent(event, keyboardShortcuts.goPatients)) {
        event.preventDefault();
        setScreen('patients');
        return;
      }

      if (shortcutMatchesEvent(event, keyboardShortcuts.refresh)) {
        event.preventDefault();
        refreshState().then(() => {
          showSuccess(tr(locale, 'Donnees rafraichies.', 'Data refreshed.', 'تم تحديث البيانات.'));
        }).catch(() => {});
        return;
      }

      if (shortcutMatchesEvent(event, keyboardShortcuts.help)) {
        event.preventDefault();
        showSuccess(tr(
          locale,
          'Raccourcis actifs: triage max, plus ancien non vu, navigation Lits/Patients, rafraichir, palette Cmd/Ctrl+K.',
          'Active shortcuts: highest triage, oldest unseen, Beds/Patients navigation, refresh, palette Cmd/Ctrl+K.',
          'الاختصارات النشطة: أعلى فرز، الأقدم غير المرئي، تنقل الأسرة/المرضى، تحديث، لوحة الأوامر Ctrl/Cmd+K.'
        ));
      }
    };

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [
    authenticated,
    isPatientPage,
    canManageBeds,
    canManagePatients,
    firstFreeBed,
    highestTriageCandidate,
    oldestUnseenCandidate,
    commandPaletteOpen,
    filteredCommandItems,
    keyboardShortcuts,
    locale,
  ]);

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
          <td colSpan="3"><div className="empty">{tr(locale, 'Reserve a l admin.', 'Admin only.', 'للمدير فقط.')}</div></td>
        </tr>
      );
    }
    if (!users.length) {
      return (
        <tr>
          <td colSpan="3"><div className="empty">{tr(locale, 'Aucun utilisateur.', 'No users.', 'لا يوجد مستخدمون.')}</div></td>
        </tr>
      );
    }
    return users.map((item) => (
      <tr key={item.username}>
        <td>{escapeText(item.username)}</td>
        <td>{roleLabel(locale, item.role || (item.admin ? 'admin' : 'user'))}</td>
        <td>
          <button
            className="mini-btn"
            type="button"
            onClick={() => setResetPasswordForm((current) => ({ ...current, username: item.username }))}
          >
            {tr(locale, 'Changer mot de passe', 'Change password', 'تغيير كلمة المرور')}
          </button>
        </td>
      </tr>
    ));
  }, [users, isAdmin, locale]);

  const createBed = async () => {
    const response = await api('/api/beds', {
      method: 'POST',
      body: JSON.stringify({
        number: Number(newBed.number),
        room: newBed.room,
        name: newBed.name,
        type: newBed.type,
      }),
    });
    if (response.ok) {
      setNewBed({ number: '', room: '', name: '', type: 'standard' });
      showSuccess(tr(locale, 'Lit cree.', 'Bed created.', 'تم إنشاء السرير.'));
      return;
    }
    showError(await readErrorMessage(response, tr(locale, 'Creation lit impossible.', 'Unable to create bed.', 'تعذر إنشاء السرير.')));
  };

  const savePatient = async () => {
    if (!canManagePatients) {
      showError(tr(locale, 'Action non autorisee pour votre profil.', 'Action not allowed for your role.', 'الإجراء غير مسموح لدورك.'));
      return;
    }
    if (isTriage && Number(newPatient.bedNumber) > 0) {
      showError(tr(locale, 'Le profil triage ne peut pas affecter un lit.', 'Triage role cannot assign beds.', 'دور الفرز لا يمكنه تخصيص سرير.'));
      return;
    }
    const response = await api('/api/patients', {
      method: 'POST',
      body: JSON.stringify({
        registrationNumber: newPatient.registrationNumber,
        name: newPatient.name,
        patientType: newPatient.patientType,
        bedNumber: Number(newPatient.bedNumber),
        triageScore: Number(newPatient.triageScore),
        status: newPatient.status,
        reason: newPatient.reason,
        destination: newPatient.destination,
        outcome: newPatient.outcome,
      }),
    });
    if (response.ok) {
      const data = await response.json().catch(() => ({}));
      setNewPatient({ registrationNumber: '', name: '', patientType: 'medical', bedNumber: '', triageScore: '0', status: 'arrived', reason: '', destination: '', outcome: '' });
      if (data?.patient?.registrationNumber) {
        setSelectedPatientReg(data.patient.registrationNumber);
        refreshPatientEvents(data.patient.registrationNumber).catch(() => {});
      }
      showSuccess(tr(locale, 'Patient enregistre.', 'Patient saved.', 'تم حفظ المريض.'));
      return;
    }
    showError(await readErrorMessage(response, tr(locale, 'Enregistrement patient impossible.', 'Unable to save patient.', 'تعذر حفظ المريض.')));
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
      setAuthForm((current) => ({ ...current, password: '' }));
      setConnectionState(data.username ? `${tr(locale, 'Connecte', 'Connected', 'متصل')}: ${data.username}` : tr(locale, 'Connecte.', 'Connected.', 'متصل.'));
      showSuccess(tr(locale, 'Connexion reussie.', 'Login successful.', 'تم تسجيل الدخول بنجاح.'));
      await refreshState().catch(() => {});
      if (data.admin) {
        await refreshUsers();
      }
      if (nextRole === 'reception') {
        window.location.assign('/patient-view');
      }
      return;
    }
    const message = await readErrorMessage(response, tr(locale, 'Identifiants invalides.', 'Invalid credentials.', 'بيانات اعتماد غير صالحة.'));
    setAuthMessage(message);
    showError(message);
  };

  const logout = async () => {
    await fetch('/api/logout', { method: 'POST', credentials: 'include' });
    setAuthenticated(false);
    setUser({ username: '', admin: false, role: 'user' });
    setUsers([]);
    setConnectionState(tr(locale, 'Acces local', 'Local access', 'وصول محلي'));
    setBeds([]);
    setPatients([]);
    setStats(initialStats);
    localStorage.removeItem('bedboard_state_cache');
    localStorage.removeItem('bedboard_current_role');
    localStorage.removeItem('bedboard_patient_type_filter');
    setPatientTypeFilter('all');
    showSuccess(tr(locale, 'Deconnexion effectuee.', 'Logged out.', 'تم تسجيل الخروج.'));
  };

  const createUser = async () => {
    const response = await api('/api/users', {
      method: 'POST',
      body: JSON.stringify(newUser),
    });
    if (response.ok) {
      setNewUser({ username: '', password: '', role: 'user' });
      await refreshUsers();
      showSuccess(tr(locale, 'Utilisateur cree.', 'User created.', 'تم إنشاء المستخدم.'));
      return;
    }
    showError(await readErrorMessage(response, tr(locale, 'Creation utilisateur impossible.', 'Unable to create user.', 'تعذر إنشاء المستخدم.')));
  };

  const changeOwnPassword = async () => {
    if (!passwordForm.currentPassword || !passwordForm.newPassword) {
      showError(tr(locale, 'Mot de passe actuel et nouveau requis.', 'Current and new password are required.', 'كلمة المرور الحالية والجديدة مطلوبة.'));
      return;
    }
    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      showError(tr(locale, 'Confirmation du nouveau mot de passe invalide.', 'Password confirmation does not match.', 'تأكيد كلمة المرور غير مطابق.'));
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
      showSuccess(tr(locale, 'Mot de passe mis a jour.', 'Password updated.', 'تم تحديث كلمة المرور.'));
      return;
    }
    const message = await response.text().catch(() => tr(locale, 'Changement de mot de passe impossible.', 'Unable to change password.', 'تعذر تغيير كلمة المرور.'));
    showError(message || tr(locale, 'Changement de mot de passe impossible.', 'Unable to change password.', 'تعذر تغيير كلمة المرور.'));
  };

  const resetUserPassword = async () => {
    if (!isAdmin) return;
    if (!resetPasswordForm.username || !resetPasswordForm.newPassword) {
      showError(tr(locale, 'Utilisateur et nouveau mot de passe requis.', 'Username and new password are required.', 'اسم المستخدم وكلمة المرور الجديدة مطلوبان.'));
      return;
    }
    if (resetPasswordForm.newPassword !== resetPasswordForm.confirmPassword) {
      showError(tr(locale, 'Confirmation du nouveau mot de passe invalide.', 'Password confirmation does not match.', 'تأكيد كلمة المرور غير مطابق.'));
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
      showSuccess(tr(locale, 'Mot de passe utilisateur mis a jour.', 'User password updated.', 'تم تحديث كلمة مرور المستخدم.'));
      return;
    }
    const message = await response.text().catch(() => tr(locale, 'Reinitialisation mot de passe impossible.', 'Unable to reset password.', 'تعذر إعادة تعيين كلمة المرور.'));
    showError(message || tr(locale, 'Reinitialisation mot de passe impossible.', 'Unable to reset password.', 'تعذر إعادة تعيين كلمة المرور.'));
  };

  const openSettings = async () => {
    setScreen('settings');
    await Promise.all([
      refreshUsers(),
      refreshAudit(true),
      refreshGotifySettings(),
      refreshAlertChannelsSettings(),
      refreshAlertNotifications({ announce: false }),
      refreshSecurityConfig(),
      refreshSecurityHealth(),
      refreshUIConfig(),
    ]);
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
      showError(await readErrorMessage(response, tr(locale, 'Sauvegarde impossible.', 'Backup failed.', 'فشل النسخ الاحتياطي.')));
      return;
    }
    const data = await response.json().catch(() => ({}));
    setLastBackupFile(data.file || '');
    const backupFile = String(data.file || '').trim();
    showSuccess(backupFile
      ? tr(locale, `Sauvegarde SQLite creee: ${backupFile}`, `SQLite backup created: ${backupFile}`, `تم إنشاء نسخة SQLite الاحتياطية: ${backupFile}`)
      : tr(locale, 'Sauvegarde SQLite creee.', 'SQLite backup created.', 'تم إنشاء نسخة SQLite الاحتياطية.'));
    await refreshAudit();
  };

  const restoreLatestBackup = async () => {
    if (!isAdmin) return;
    setConfirm({
      open: true,
      title: tr(locale, 'Restaurer la derniere sauvegarde', 'Restore latest backup', 'استعادة آخر نسخة احتياطية'),
      message: tr(locale, 'Cette action remplace la base SQLite actuelle. Continuer ?', 'This action replaces current SQLite database. Continue?', 'هذا الإجراء يستبدل قاعدة SQLite الحالية. المتابعة؟'),
      onConfirm: async () => {
        const response = await api('/api/admin/restore', { method: 'POST', body: JSON.stringify({}) });
        if (!response.ok) {
          showError(await readErrorMessage(response, tr(locale, 'Restauration impossible.', 'Restore failed.', 'فشلت الاستعادة.')));
          return;
        }
        const data = await response.json().catch(() => ({}));
        setLastBackupFile(data.file || '');
        await refreshState();
        await refreshAudit();
        const restoredFile = String(data.file || '').trim();
        showSuccess(restoredFile
          ? tr(locale, `Restauration SQLite terminee depuis ${restoredFile}.`, `SQLite restore completed from ${restoredFile}.`, `اكتملت استعادة SQLite من ${restoredFile}.`)
          : tr(locale, 'Restauration SQLite terminee.', 'SQLite restore completed.', 'اكتملت استعادة SQLite.'));
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
    if (!current) return <div className="empty">{tr(locale, 'Aucun dossier.', 'No case.', 'لا توجد حالة.')}</div>;
    const currentName = String(current.name || '').trim();
    const currentReg = String(current.registrationNumber || '').trim();
    const identity = patientViewIdentityMode === 'number'
      ? (currentReg || currentName || tr(locale, 'Sans identifiant', 'Without identifier', 'بدون معرف'))
      : (currentName || currentReg || tr(locale, 'Sans identifiant', 'Without identifier', 'بدون معرف'));
    const wish = tr(
      locale,
      'Bon retablissement et courage.',
      'Wishing you a quick and smooth recovery.',
      'نتمنى لك الشفاء العاجل والتعافي السريع.'
    );
    return (
      <div className="patient-center">
        <div className="patient-line">{escapeText(identity)} - {current.bedNumber ? `${escapeText(current.roomName || tr(locale, 'Chambre', 'Room', 'غرفة'))} - ${escapeText(current.bedName || `${tr(locale, 'Lit', 'Bed', 'سرير')} ${current.bedNumber}`)}` : tr(locale, 'Non assigne', 'Unassigned', 'غير مخصص')}</div>
        <div className="small-note" style={{ marginTop: 12, fontSize: '1.05rem' }}>{wish}</div>
      </div>
    );
  }, [currentPatient, locale, patientViewIdentityMode]);

  if (isPatientPage) {
    if (!authResolved) {
      return (
        <div className="app patient-page-shell">
          <div className="section-card patient-page-card">
            <div className="empty">{tr(locale, 'Verification de session en cours...', 'Checking session...', 'جار التحقق من الجلسة...')}</div>
          </div>
        </div>
      );
    }
    if (!authenticated) {
      window.location.assign('/');
      return null;
    }
    return (
      <PatientViewPage
        openMainPage={openMainPage}
        patientPanel={patientPanel}
        locale={locale}
        brandName={brandName}
        brandLogo={brandLogo}
      />
    );
  }

  if (authenticated && isReception) {
    return (
      <ReceptionPage logout={logout} openPatientPage={openPatientPage} locale={locale} brandName={brandName} brandLogo={brandLogo} />
    );
  }

  if (!authResolved) {
    return (
      <div className="app login-shell">
        <div className="section-card login-card">
          <div className="empty">{tr(locale, 'Verification de session en cours...', 'Checking session...', 'جار التحقق من الجلسة...')}</div>
        </div>
      </div>
    );
  }

  if (!authenticated) {
    return (
      <div className="app login-shell">
        {notice.text ? (
          <div className={`notice ${notice.type === 'error' ? 'error' : 'success'}`} role="status" aria-live="polite">
            <span>{notice.text}</span>
            <button className="notice-close" type="button" aria-label={tr(locale, 'Fermer la notification', 'Close notification', 'إغلاق الإشعار')} onClick={() => setNotice({ type: '', text: '' })}>
              {tr(locale, 'Fermer', 'Close', 'إغلاق')}
            </button>
          </div>
        ) : null}
        <div className="section-card login-card">
          <div className="login-brand">
            <div className="brand-mark login-brand-mark"><img src={brandLogo} alt={tr(locale, "Logo de l'hopital", 'Hospital logo', 'شعار المستشفى')} /></div>
            <h1 className="login-title">{brandName}</h1>
            <p className="small-note">{authMessage}</p>
          </div>
          <form
            className="login-form"
            onSubmit={(event) => {
              event.preventDefault();
              authenticate();
            }}
          >
            <label>
              {tr(locale, 'Identifiant', 'Username', 'اسم المستخدم')}
              <input className="form-control" value={authForm.username} type="text" onChange={(event) => setAuthForm((current) => ({ ...current, username: event.target.value }))} />
            </label>
            <label>
              {tr(locale, 'Mot de passe', 'Password', 'كلمة المرور')}
              <input className="form-control" value={authForm.password} type="password" onChange={(event) => setAuthForm((current) => ({ ...current, password: event.target.value }))} />
            </label>
            <button className="btn primary" type="submit">{tr(locale, 'Se connecter', 'Sign in', 'تسجيل الدخول')}</button>
            <p className="small-note login-state">{connectionState}</p>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="app">
      <div className="navbar">
        <div className="nav-left">
          <div className="brand-mark"><img src={brandLogo} alt={tr(locale, "Logo de l'hopital", 'Hospital logo', 'شعار المستشفى')} /></div>
          <div className="nav-title">
            <strong>{brandName}</strong>
            <span>{connectionState}</span>
          </div>
        </div>
        <div className="nav-actions">
          {authenticated && isAdmin ? (
            <button className={`security-chip inline ${securityNavStatus}`} type="button" onClick={openSettings}>
              {tr(locale, 'Securite', 'Security', 'الأمان')}: {securityNavStatus.toUpperCase()}
            </button>
          ) : null}
          {authenticated && user.username ? <div className="user-chip">{user.username} ({roleLabel(locale, role)})</div> : null}
          <button className="btn" type="button" onClick={openPatientPage}>{tr(locale, 'Page patient', 'Patient page', 'صفحة المرضى')}</button>
          {authenticated && isAdmin ? <button className="btn" type="button" onClick={openSettings}>{tr(locale, 'Parametres', 'Settings', 'الإعدادات')}</button> : null}
          {authenticated ? <button className="btn primary" type="button" onClick={logout}>{tr(locale, 'Deconnexion', 'Logout', 'تسجيل الخروج')}</button> : null}
        </div>
      </div>

      {notice.text ? (
        <div className={`notice ${notice.type === 'error' ? 'error' : 'success'}`} role="status" aria-live="polite">
          <span>{notice.text}</span>
          <button
            className="notice-close"
            type="button"
            aria-label={tr(locale, 'Fermer la notification', 'Close notification', 'إغلاق الإشعار')}
            onClick={() => setNotice({ type: '', text: '' })}
          >
            {tr(locale, 'Fermer', 'Close', 'إغلاق')}
          </button>
        </div>
      ) : null}

      {commandPaletteOpen ? (
        <div className="modal-backdrop open" role="dialog" aria-modal="true">
          <div className="modal section-card command-palette-modal">
            <h2>{tr(locale, 'Palette de commandes', 'Command palette', 'لوحة الأوامر')}</h2>
            <input
              className="form-control"
              autoFocus
              value={commandQuery}
              placeholder={tr(locale, 'Rechercher dossier, lit, action...', 'Search case, bed, action...', 'ابحث عن حالة أو سرير أو إجراء...')}
              onChange={(event) => setCommandQuery(event.target.value)}
            />
            <div className="command-palette-list">
              {filteredCommandItems.length ? filteredCommandItems.map((item, index) => (
                <button
                  key={item.id}
                  className={`command-palette-item ${index === 0 ? 'primary' : ''}`}
                  type="button"
                  onClick={() => {
                    setCommandPaletteOpen(false);
                    Promise.resolve(item.action()).catch(() => {});
                  }}
                >
                  {item.label}
                </button>
              )) : (
                <div className="empty">{tr(locale, 'Aucun resultat.', 'No results.', 'لا توجد نتائج.')}</div>
              )}
            </div>
            <div className="modal-actions">
              <button className="btn" type="button" onClick={() => setCommandPaletteOpen(false)}>{tr(locale, 'Fermer', 'Close', 'إغلاق')}</button>
            </div>
          </div>
        </div>
      ) : null}

      <div className="shell">
        <div className="hero">
          <div className="brand-card">
            <p className="eyebrow">{brandName}</p>
            <h1>{tr(locale, 'Gestion des lits et des patients', 'Bed and patient management', 'إدارة الأسرة والمرضى')}</h1>
            
            <div className="status-row">
              <span className="pill"><span className="dot" style={{ background: 'var(--green)' }} /> {tr(locale, 'Libre', 'Free', 'شاغر')}</span>
              <span className="pill"><span className="dot" style={{ background: 'var(--blue)' }} /> {tr(locale, 'Occupe', 'Occupied', 'مشغول')}</span>
              <span className="pill"><span className="dot" style={{ background: 'var(--violet)' }} /> {tr(locale, 'Nettoyage', 'Cleaning', 'تنظيف')}</span>
              <span className="pill"><span className="dot" style={{ background: 'var(--red)' }} /> {tr(locale, 'Alerte', 'Alert', 'إنذار')}</span>
            </div>
          </div>
          <div className="side-card">
            <div className="stats-grid">
              <div className="stat"><span>{tr(locale, 'Total lits', 'Total beds', 'إجمالي الأسرة')}</span><strong>{stats.totalBeds || 0}</strong></div>
              <div className="stat"><span>{tr(locale, 'Occupes', 'Occupied', 'مشغولة')}</span><strong>{stats.occupiedBeds || 0}</strong></div>
              <div className="stat"><span>{tr(locale, 'Libres', 'Free', 'شاغرة')}</span><strong>{stats.freeBeds || 0}</strong></div>
              <div className="stat"><span>{tr(locale, 'Patients', 'Patients', 'المرضى')}</span><strong>{stats.totalPatients || 0}</strong></div>
            </div>
          </div>
        </div>

        <div className="section-card">
          <div className="tab-strip">
            {canManageBeds ? <button className={`tab-btn ${screen === 'beds' ? 'active' : ''}`} type="button" onClick={() => setScreen('beds')}>{tr(locale, 'Lits', 'Beds', 'الأسرة')}</button> : null}
            {canManagePatients ? <button className={`tab-btn ${screen === 'patients' ? 'active' : ''}`} type="button" onClick={() => setScreen('patients')}>{tr(locale, 'Patients', 'Patients', 'المرضى')}</button> : null}
            {authenticated && isAdmin ? <button className={`tab-btn ${screen === 'stats' ? 'active' : ''}`} type="button" onClick={() => setScreen('stats')}>{tr(locale, 'Statistiques', 'Statistics', 'الإحصاءات')}</button> : null}
            {authenticated ? <button className={`tab-btn ${screen === 'account' ? 'active' : ''}`} type="button" onClick={openAccount}>{tr(locale, 'Mon compte', 'My account', 'حسابي')}</button> : null}
            {authenticated && isAdmin ? <button className={`tab-btn ${screen === 'settings' ? 'active' : ''}`} type="button" onClick={openSettings}>{tr(locale, 'Parametres', 'Settings', 'الإعدادات')}</button> : null}
          </div>

          {screen === 'beds' && canManageBeds ? (
            <div className="screen active">
              <div className="controls-grid">
                {isAdmin ? (
                  <div className="form-card">
                    <h2>{tr(locale, 'Ajouter un lit', 'Add bed', 'إضافة سرير')}</h2>
                    <div className="form-grid">
                      <label>
                        {tr(locale, 'Numero', 'Number', 'الرقم')}
                        <input className="form-control" value={newBed.number} type="number" min="1" onChange={(event) => setNewBed((current) => ({ ...current, number: event.target.value }))} />
                      </label>
                      <label>
                        {tr(locale, 'Nom', 'Name', 'الاسم')}
                        <input className="form-control" value={newBed.name} type="text" onChange={(event) => setNewBed((current) => ({ ...current, name: event.target.value }))} />
                      </label>
                      <label>
                        {tr(locale, 'Chambre', 'Room', 'الغرفة')}
                        <input className="form-control" value={newBed.room} type="text" onChange={(event) => setNewBed((current) => ({ ...current, room: event.target.value }))} />
                      </label>
                      <label>
                        {tr(locale, 'Type', 'Type', 'النوع')}
                        <select className="form-select" value={newBed.type} onChange={(event) => setNewBed((current) => ({ ...current, type: event.target.value }))}>
                          <option value="standard">Standard</option>
                          <option value="thoracique">{tr(locale, 'Thoracique', 'Thoracic', 'صدري')}</option>
                        </select>
                      </label>
                      <button className="btn primary" type="button" onClick={createBed}>{tr(locale, 'Creer', 'Create', 'إنشاء')}</button>
                    </div>
                  </div>
                ) : (
                  <div className="form-card">
                    <h2>{tr(locale, 'Actions rapides', 'Quick actions', 'إجراءات سريعة')}</h2>
                    <p className="small-note">{tr(locale, "Les utilisateurs authentifies peuvent modifier l'etat des lits et gerer les patients. La creation et la suppression de lits sont reservees a l admin.", 'Authenticated users can update bed status and manage patients. Bed create/delete is admin-only.', 'يمكن للمستخدمين الموثقين تعديل حالة الأسرة وإدارة المرضى. إنشاء/حذف الأسرة للمدير فقط.')}</p>
                  </div>
                )}
                <div className="form-card">
                  <h2>{tr(locale, 'Actions rapides', 'Quick actions', 'إجراءات سريعة')}</h2>
                  <p className="small-note">{tr(locale, 'Les changements se synchronisent automatiquement sur les postes connectes.', 'Changes sync automatically across connected stations.', 'تتم مزامنة التغييرات تلقائيًا عبر المحطات المتصلة.')}</p>
                </div>
                {canCustomizePatientTypeVisibility ? (
                  <div className="form-card">
                    <h2>{tr(locale, 'Types visibles (affectation)', 'Visible types (assignment)', 'الأنواع الظاهرة (التخصيص)')}</h2>
                    <p className="small-note">{tr(locale, "Choisissez les types de patients affiches pour votre utilisateur dans l'affectation Beds et la rubrique Patients.", 'Choose patient types visible for your user in bed assignment and patients tab.', 'اختر أنواع المرضى الظاهرة لمستخدمك في تخصيص الأسرة وقسم المرضى.')}</p>
                    <div className="patient-priority-row" style={{ marginTop: 8 }}>
                      {patientTypeSelectableOptions.map((option) => (
                        <label key={option} className="patient-priority-chip" style={{ display: 'inline-flex', gap: 6, alignItems: 'center' }}>
                          <input
                            type="checkbox"
                            checked={visiblePatientTypes.includes(option)}
                            onChange={() => toggleVisiblePatientType(option)}
                          />
                          {patientTypeLabel(locale, option)}
                        </label>
                      ))}
                    </div>
                    <p className="small-note">{tr(locale, 'Patients visibles maintenant', 'Visible patients now', 'المرضى الظاهرون الآن')}: {visiblePatients.length} / {activePatients.length}</p>
                  </div>
                ) : null}
              </div>
              <div className="grid">
                <BedsGrid
                  beds={beds}
                  statusMeta={localizedStatusMeta}
                  normalizeStatus={normalizeStatus}
                  escapeText={escapeText}
                  canManageBeds={canManageBeds}
                  isAdmin={isAdmin}
                  assignByBed={assignByBed}
                  setAssignByBed={setAssignByBed}
                  activePatients={activePatients}
                  assignablePatients={visiblePatients}
                  bedEdits={bedEdits}
                  setBedEdits={setBedEdits}
                  api={api}
                  showError={showError}
                  showSuccess={showSuccess}
                  readErrorMessage={readErrorMessage}
                  setConfirm={setConfirm}
                  assignPatientToBed={assignPatientToBed}
                  locale={locale}
                />
              </div>
              <div className="foot">{connectionState}</div>
            </div>
          ) : null}

          {screen === 'patients' && canManagePatients ? (
            <div className="screen active">
              <div className="patients-header-grid">
                <div className="form-card patient-command-card">
                  <div className="patient-command-head">
                    <h2>{tr(locale, 'Poste de commande patients', 'Patient command desk', 'وحدة أوامر المرضى')}</h2>
                    <p className="small-note">{tr(locale, "Saisie prioritaire pour l'ajout, le triage et l'assignation.", 'Priority workflow for registration, triage, and assignment.', 'تدفق أولوية للتسجيل والفرز والتخصيص.')}</p>
                  </div>
                  <div className="patient-priority-row">
                    <span className="patient-priority-chip">{tr(locale, 'Affiches', 'Displayed', 'المعروض')}: {filteredPatients.length} / {visiblePatients.length}</span>
                    <span className="patient-priority-chip warn">{tr(locale, 'Non assignes', 'Unassigned', 'غير مخصصين')}: {filteredPatients.filter((patient) => !patient.bedNumber).length}</span>
                    <span className="patient-priority-chip danger">{tr(locale, 'Triage critique', 'Critical triage', 'فرز حرج')}: {filteredPatients.filter((patient) => triageLevelOf(patient) >= 3).length}</span>
                  </div>
                  <div className="form-card" style={{ marginTop: 8 }}>
                    <h3>{tr(locale, 'Priorisation intelligente', 'Smart prioritization', 'الأولوية الذكية')}</h3>
                    {nextToCall ? (
                      <>
                        <p className="small-note">
                          {tr(locale, 'Prochain a appeler', 'Next to call', 'التالي للنداء')}: {nextToCall.patient.registrationNumber} - {escapeText(nextToCall.patient.name || '-')}
                          {' | '}{tr(locale, 'Score', 'Score', 'النقاط')}: {nextToCall.score}
                          {' | '}{tr(locale, 'Attente', 'Waiting', 'الانتظار')}: {nextToCall.wait} min
                          {nextToCall.slaRisk ? ` | ${tr(locale, 'Risque SLA', 'SLA risk', 'خطر SLA')}` : ''}
                        </p>
                        <div className="settings-action-row">
                          <button className="btn" type="button" onClick={() => {
                            setScreen('patients');
                            setSelectedPatientReg(nextToCall.patient.registrationNumber);
                            setNewPatient((current) => ({
                              ...current,
                              registrationNumber: nextToCall.patient.registrationNumber,
                              name: nextToCall.patient.name || current.name,
                              status: nextToCall.patient.status || current.status,
                            }));
                            refreshPatientEvents(nextToCall.patient.registrationNumber).catch(() => {});
                          }}>{tr(locale, 'Charger ce dossier', 'Load this case', 'تحميل هذه الحالة')}</button>
                        </div>
                      </>
                    ) : (
                      <p className="small-note">{tr(locale, 'Aucune priorite disponible.', 'No priority candidate available.', 'لا توجد أولوية متاحة.')}</p>
                    )}
                  </div>
                  {canCustomizePatientTypeVisibility ? (
                    <div className="patient-priority-row" style={{ marginTop: 8 }}>
                      {patientTypeSelectableOptions.map((option) => (
                        <label key={option} className="patient-priority-chip" style={{ display: 'inline-flex', gap: 6, alignItems: 'center' }}>
                          <input
                            type="checkbox"
                            checked={visiblePatientTypes.includes(option)}
                            onChange={() => toggleVisiblePatientType(option)}
                          />
                          {patientTypeLabel(locale, option)}
                        </label>
                      ))}
                    </div>
                  ) : null}
                  {canCustomizePatientTypeVisibility ? (
                    <div className="form-grid compact">
                      <label>
                        {tr(locale, 'Filtre type patient', 'Patient type filter', 'تصفية نوع المريض')}
                        <select className="form-select" value={patientTypeFilter} onChange={(event) => setPatientTypeFilter(event.target.value)}>
                          {patientTypeOptions.map((option) => (
                            <option key={option} value={option}>{option === 'all' ? tr(locale, 'Tous', 'All', 'الكل') : patientTypeLabel(locale, option)}</option>
                          ))}
                        </select>
                      </label>
                    </div>
                  ) : null}
                  <div className="form-grid patient-command-grid">
                    <label>
                      {tr(locale, 'Numero', 'Number', 'الرقم')}
                      <input className="form-control" value={newPatient.registrationNumber} type="text" onChange={(event) => setNewPatient((current) => ({ ...current, registrationNumber: event.target.value }))} />
                    </label>
                    <label>
                      {tr(locale, 'Nom', 'Name', 'الاسم')}
                      <input className="form-control" value={newPatient.name} type="text" onChange={(event) => setNewPatient((current) => ({ ...current, name: event.target.value }))} />
                    </label>
                    <label>
                      {tr(locale, 'Type patient', 'Patient type', 'نوع المريض')}
                      <select className="form-select" value={newPatient.patientType} onChange={(event) => setNewPatient((current) => ({ ...current, patientType: event.target.value }))}>
                        <option value="traumato">{patientTypeLabel(locale, 'traumato')}</option>
                        <option value="medical">{patientTypeLabel(locale, 'medical')}</option>
                        <option value="douleurs_thoracique">{patientTypeLabel(locale, 'douleurs_thoracique')}</option>
                        <option value="chirurgical">{patientTypeLabel(locale, 'chirurgical')}</option>
                        <option value="urgences_differees">{patientTypeLabel(locale, 'urgences_differees')}</option>
                      </select>
                    </label>
                    <label>
                      {tr(locale, 'Lit', 'Bed', 'السرير')}
                      <input className="form-control" value={newPatient.bedNumber} type="number" min="1" disabled={isTriage} onChange={(event) => setNewPatient((current) => ({ ...current, bedNumber: event.target.value }))} />
                    </label>
                    <label>
                      {tr(locale, 'Score triage (0-4)', 'Triage score (0-4)', 'درجة الفرز (0-4)')}
                      <select className="form-select" value={newPatient.triageScore} onChange={(event) => setNewPatient((current) => ({ ...current, triageScore: event.target.value }))}>
                        <option value="0">0</option>
                        <option value="1">1</option>
                        <option value="2">2</option>
                        <option value="3">3</option>
                        <option value="4">4</option>
                      </select>
                    </label>
                    <label>
                      {tr(locale, 'Statut patient', 'Patient status', 'حالة المريض')}
                      <select className="form-select" value={newPatient.status} onChange={(event) => setNewPatient((current) => ({ ...current, status: event.target.value }))}>
                        <option value="arrived">{patientStatusLabel(locale, 'arrived')}</option>
                        <option value="triaged">{patientStatusLabel(locale, 'triaged')}</option>
                        <option value="waiting">{patientStatusLabel(locale, 'waiting')}</option>
                        <option value="assigned">{patientStatusLabel(locale, 'assigned')}</option>
                        <option value="in_exam">{patientStatusLabel(locale, 'in_exam')}</option>
                        <option value="imaging">{patientStatusLabel(locale, 'imaging')}</option>
                        <option value="waiting_results">{patientStatusLabel(locale, 'waiting_results')}</option>
                        <option value="discharge_ready">{patientStatusLabel(locale, 'discharge_ready')}</option>
                        <option value="consulted">{patientStatusLabel(locale, 'consulted')}</option>
                        <option value="transferred">{patientStatusLabel(locale, 'transferred')}</option>
                        <option value="deceased">{patientStatusLabel(locale, 'deceased')}</option>
                      </select>
                    </label>
                    <label>
                      {tr(locale, 'Motif', 'Reason', 'السبب')}
                      <input className="form-control" value={newPatient.reason} type="text" onChange={(event) => setNewPatient((current) => ({ ...current, reason: event.target.value }))} />
                    </label>
                    <label>
                      {tr(locale, 'Destination', 'Destination', 'الوجهة')}
                      <input className="form-control" value={newPatient.destination} type="text" onChange={(event) => setNewPatient((current) => ({ ...current, destination: event.target.value }))} />
                    </label>
                    <label>
                      {tr(locale, 'Issue clinique', 'Outcome', 'النتيجة')}
                      <input className="form-control" value={newPatient.outcome} type="text" onChange={(event) => setNewPatient((current) => ({ ...current, outcome: event.target.value }))} />
                    </label>
                    <button className="btn primary" type="button" onClick={savePatient}>{tr(locale, 'Enregistrer patient', 'Save patient', 'حفظ المريض')}</button>
                  </div>
                  {isTriage ? <p className="small-note">{tr(locale, 'Le profil triage enregistre uniquement des patients non assignes.', 'Triage role only registers unassigned patients.', 'دور الفرز يسجل المرضى غير المخصصين فقط.')}</p> : null}
                </div>
                <div className="form-card">
                  <h2>{tr(locale, 'Actions critiques', 'Critical actions', 'إجراءات حرجة')}</h2>
                  <p className="small-note">{tr(locale, 'Prioriser les patients avec triage 3-4 puis traiter les non assignes.', 'Prioritize triage level 3-4, then handle unassigned patients.', 'أعطِ أولوية لفرز 3-4 ثم عالج غير المخصصين.')}</p>
                  <div className="patient-legend">
                    <span className="patient-legend-item"><span className="legend-dot triage-critical-dot" /> {tr(locale, 'Triage eleve', 'High triage', 'فرز مرتفع')}</span>
                    <span className="patient-legend-item"><span className="legend-dot triage-medium-dot" /> {tr(locale, 'Triage modere', 'Medium triage', 'فرز متوسط')}</span>
                    <span className="patient-legend-item"><span className="legend-dot triage-low-dot" /> {tr(locale, 'Triage bas', 'Low triage', 'فرز منخفض')}</span>
                  </div>
                </div>
              </div>
              <div className="table-wrap patients-table-wrap">
                <table className="table table-sm align-middle mb-0 patients-table">
                  <thead>
                    <tr>
                      <th>{tr(locale, 'Numero inscription', 'Registration number', 'رقم التسجيل')}</th>
                      <th>{tr(locale, 'Nom', 'Name', 'الاسم')}</th>
                      {canViewPatientType ? <th>{tr(locale, 'Type', 'Type', 'النوع')}</th> : null}
                      <th>{tr(locale, 'Triage', 'Triage', 'الفرز')}</th>
                      <th>{tr(locale, 'Statut', 'Status', 'الحالة')}</th>
                      <th>{tr(locale, 'Chambre / Lit', 'Room / Bed', 'الغرفة / السرير')}</th>
                      <th>{tr(locale, 'Actions', 'Actions', 'الإجراءات')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <PatientsRows
                      activePatients={filteredPatients}
                      canViewTriage={canViewTriage}
                      canViewPatientType={canViewPatientType}
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
                      setSelectedPatientReg={setSelectedPatientReg}
                      refreshPatientEvents={refreshPatientEvents}
                      escapeText={escapeText}
                      locale={locale}
                    />
                  </tbody>
                </table>
              </div>
              <div className="form-card" style={{ marginTop: 14 }}>
                <h2>{tr(locale, 'Timeline patient', 'Patient timeline', 'التسلسل الزمني للمريض')}</h2>
                <p className="small-note">{selectedPatientReg ? `${tr(locale, 'Patient', 'Patient', 'المريض')}: ${escapeText(selectedPatientReg)}` : tr(locale, 'Selectionnez un patient pour afficher ses evenements.', 'Select a patient to view timeline events.', 'اختر مريضًا لعرض الأحداث الزمنية.')}</p>
                <div className="table-wrap">
                  <table className="table table-sm align-middle mb-0">
                    <thead>
                      <tr>
                        <th>{tr(locale, 'Heure', 'Time', 'الوقت')}</th>
                        <th>{tr(locale, 'Evenement', 'Event', 'الحدث')}</th>
                        <th>{tr(locale, 'Utilisateur', 'User', 'المستخدم')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {patientEvents.length ? patientEvents.map((entry) => (
                        <tr key={entry.id || `${entry.createdAt}-${entry.event}`}>
                          <td>{entry.createdAt ? new Date(entry.createdAt).toLocaleString(locale) : '-'}</td>
                          <td>{escapeText(entry.event)}</td>
                          <td>{escapeText(entry.username || 'system')}</td>
                        </tr>
                      )) : (
                        <tr><td colSpan="3"><div className="empty">{tr(locale, 'Aucun evenement timeline.', 'No timeline events.', 'لا توجد أحداث زمنية.')}</div></td></tr>
                      )}
                    </tbody>
                  </table>
                </div>
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
            <StatsScreen stats={stats} locale={locale} />
          ) : null}

          {screen === 'account' && authenticated ? (
            <AccountScreen
              passwordForm={passwordForm}
              setPasswordForm={setPasswordForm}
              changeOwnPassword={changeOwnPassword}
              user={user}
              keyboardShortcuts={keyboardShortcuts}
              setKeyboardShortcuts={setKeyboardShortcuts}
              locale={locale}
            />
          ) : null}

          {screen === 'settings' && authenticated && isAdmin ? (
            <SettingsScreen
              newUser={newUser}
              setNewUser={setNewUser}
              createUser={createUser}
              refreshUsers={refreshUsers}
              resetPasswordForm={resetPasswordForm}
              setResetPasswordForm={setResetPasswordForm}
              resetUserPassword={resetUserPassword}
              usersCount={users.length}
              createBackup={createBackup}
              restoreLatestBackup={restoreLatestBackup}
              lastBackupFile={lastBackupFile}
              refreshAuditLogs={() => refreshAudit(true)}
              gotifyForm={gotifyForm}
              setGotifyForm={setGotifyForm}
              saveGotifySettings={saveGotifySettings}
              testGotifySettings={testGotifySettings}
              refreshGotifySettings={refreshGotifySettings}
              alertChannelsForm={alertChannelsForm}
              setAlertChannelsForm={setAlertChannelsForm}
              saveAlertChannelsSettings={saveAlertChannelsSettings}
              testAlertChannelsSettings={testAlertChannelsSettings}
              refreshAlertChannelsSettings={refreshAlertChannelsSettings}
              alertNotifications={alertNotifications}
              refreshAlertNotifications={refreshAlertNotifications}
              acknowledgeAlertNotification={acknowledgeAlertNotification}
              acknowledgeAllAlertNotifications={acknowledgeAllAlertNotifications}
              securityConfigForm={securityConfigForm}
              setSecurityConfigForm={setSecurityConfigForm}
              saveSecurityConfig={saveSecurityConfig}
              refreshSecurityConfig={refreshSecurityConfig}
              securityHealth={securityHealth}
              refreshSecurityHealth={refreshSecurityHealth}
              exportAuditCsv={exportAuditCsv}
              exportFHIRBundle={exportFHIRBundle}
              patientImportForm={patientImportForm}
              setPatientImportForm={setPatientImportForm}
              importPatients={importPatients}
              uiConfigForm={uiConfigForm}
              setUiConfigForm={setUiConfigForm}
              saveUiConfig={saveUiConfig}
              refreshUIConfig={refreshUIConfig}
              locale={locale}
              renderUsers={renderUsers}
              auditLogs={auditLogs}
              escapeText={escapeText}
            />
          ) : null}
        </div>
      </div>

      {confirm.open ? (
      <SimpleModal
        title={confirm.title}
        text={confirm.message}
        onCancel={closeConfirm}
        onConfirm={handleConfirmAction}
        locale={locale}
      />
      ) : null}
    </div>
  );
}

export default App;
