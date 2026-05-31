import { useMemo, useState } from 'react';
import { roleLabel, tr } from '../lib/i18n';

const sections = ['parameters', 'security', 'integrations', 'operations'];

export default function SettingsScreen({
  newUser,
  setNewUser,
  createUser,
  refreshUsers,
  resetPasswordForm,
  setResetPasswordForm,
  resetUserPassword,
  usersCount,
  createBackup,
  restoreLatestBackup,
  lastBackupFile,
  refreshAuditLogs,
  gotifyForm,
  setGotifyForm,
  saveGotifySettings,
  testGotifySettings,
  refreshGotifySettings,
  alertChannelsForm,
  setAlertChannelsForm,
  saveAlertChannelsSettings,
  testAlertChannelsSettings,
  refreshAlertChannelsSettings,
  alertNotifications,
  refreshAlertNotifications,
  acknowledgeAlertNotification,
  acknowledgeAllAlertNotifications,
  securityConfigForm,
  setSecurityConfigForm,
  saveSecurityConfig,
  refreshSecurityConfig,
  securityHealth,
  refreshSecurityHealth,
  exportAuditCsv,
  exportFHIRBundle,
  patientImportForm,
  setPatientImportForm,
  importPatients,
  uiConfigForm,
  setUiConfigForm,
  saveUiConfig,
  refreshUIConfig,
  locale,
  renderUsers,
  auditLogs,
  escapeText,
}) {
  const [activeSection, setActiveSection] = useState('parameters');
  const healthStatus = String(securityHealth?.status || 'unknown').toLowerCase();
  const checks = Array.isArray(securityHealth?.checks) ? securityHealth.checks : [];
  const notifications = Array.isArray(alertNotifications) ? alertNotifications : [];
  const pendingNotifications = useMemo(
    () => notifications.filter((entry) => String(entry.status || '').toLowerCase() !== 'acknowledged'),
    [notifications],
  );
  const failedNotifications = useMemo(
    () => notifications.filter((entry) => String(entry.status || '').toLowerCase() === 'failed'),
    [notifications],
  );

  const buildStrongPassword = () => {
    const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%&*?';
    if (window.crypto?.getRandomValues) {
      const bytes = new Uint8Array(16);
      window.crypto.getRandomValues(bytes);
      return Array.from(bytes, (byte) => alphabet[byte % alphabet.length]).join('');
    }
    return Array.from({ length: 16 }, () => alphabet[Math.floor(Math.random() * alphabet.length)]).join('');
  };

  const sectionLabel = (key) => {
    switch (key) {
      case 'parameters':
        return tr(locale, 'Parametres', 'Parameters', 'المعلمات');
      case 'security':
        return tr(locale, 'Securite', 'Security', 'الأمان');
      case 'integrations':
        return tr(locale, 'Integrations', 'Integrations', 'التكاملات');
      case 'operations':
        return tr(locale, 'Operations', 'Operations', 'العمليات');
      default:
        return key;
    }
  };

  return (
    <div className="screen active settings-shell">
      <div className="settings-toolbar form-card">
        <div className="settings-toolbar-row">
          <h2>{tr(locale, 'Console administration', 'Administration console', 'وحدة الإدارة')}</h2>
          <div className="settings-action-row">
            <button className="btn" type="button" onClick={() => setActiveSection('parameters')}>{sectionLabel('parameters')}</button>
            <button className="btn" type="button" onClick={() => setActiveSection('security')}>{sectionLabel('security')}</button>
            <button className="btn" type="button" onClick={() => setActiveSection('integrations')}>{sectionLabel('integrations')}</button>
            <button className="btn" type="button" onClick={() => setActiveSection('operations')}>{sectionLabel('operations')}</button>
          </div>
        </div>
        <div className="settings-kpi-grid">
          <div className="settings-kpi">
            <span>{tr(locale, 'Utilisateurs', 'Users', 'المستخدمون')}</span>
            <strong>{Number(usersCount || 0)}</strong>
          </div>
          <div className="settings-kpi">
            <span>{tr(locale, 'Securite', 'Security', 'الأمان')}</span>
            <strong className={`security-chip inline ${healthStatus}`}>{healthStatus.toUpperCase()}</strong>
          </div>
          <div className="settings-kpi">
            <span>{tr(locale, 'Alertes en attente', 'Pending alerts', 'تنبيهات معلقة')}</span>
            <strong>{pendingNotifications.length}</strong>
          </div>
          <div className="settings-kpi">
            <span>{tr(locale, 'Echecs canaux', 'Channel failures', 'فشل القنوات')}</span>
            <strong>{failedNotifications.length}</strong>
          </div>
        </div>
      </div>

      <div className="tab-strip">
        {sections.map((section) => (
          <button
            key={section}
            className={`tab-btn ${activeSection === section ? 'active' : ''}`}
            type="button"
            onClick={() => setActiveSection(section)}
          >
            {sectionLabel(section)}
          </button>
        ))}
      </div>

      {activeSection === 'parameters' ? (
        <div className="settings-panel-grid">
          <div className="form-card settings-wide">
            <h2>{tr(locale, 'White-label et langue', 'White-label and language', 'الهوية البصرية واللغة')}</h2>
            <p className="small-note">{tr(locale, 'Nom application, logo et langue globale de l interface.', 'App name, logo and global interface language.', 'اسم التطبيق والشعار ولغة الواجهة العامة.')}</p>
            <div className="form-grid compact">
              <label>
                {tr(locale, 'Nom application', 'Application name', 'اسم التطبيق')}
                <input className="form-control" value={uiConfigForm.appName} type="text" onChange={(event) => setUiConfigForm((current) => ({ ...current, appName: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Langue interface', 'Interface language', 'لغة الواجهة')}
                <select className="form-select" value={uiConfigForm.locale} onChange={(event) => setUiConfigForm((current) => ({ ...current, locale: event.target.value }))}>
                  <option value="fr">Francais</option>
                  <option value="en">English</option>
                  <option value="ar">العربية</option>
                </select>
              </label>
              <label>
                {tr(locale, 'Vue salle: identite affichee', 'Room view: displayed identity', 'عرض القاعة: الهوية المعروضة')}
                <select className="form-select" value={uiConfigForm.patientViewIdentityMode || 'name'} onChange={(event) => setUiConfigForm((current) => ({ ...current, patientViewIdentityMode: event.target.value }))}>
                  <option value="name">{tr(locale, 'Nom', 'Name', 'الاسم')}</option>
                  <option value="number">{tr(locale, 'Numero inscription', 'Registration number', 'رقم التسجيل')}</option>
                </select>
              </label>
              <label>
                {tr(locale, 'Langue appel patient', 'Patient calling language', 'لغة نداء المريض')}
                <select className="form-select" value={uiConfigForm.patientCallLanguage || 'both'} onChange={(event) => setUiConfigForm((current) => ({ ...current, patientCallLanguage: event.target.value }))}>
                  <option value="both">{tr(locale, 'Bilingue (Ar + Fr)', 'Bilingual (Ar + Fr)', 'ثنائي اللغة (عربي + فرنسي)')}</option>
                  <option value="ar">{tr(locale, 'Arabe', 'Arabic', 'العربية')}</option>
                  <option value="fr">{tr(locale, 'Francais', 'French', 'الفرنسية')}</option>
                  <option value="en">{tr(locale, 'Anglais', 'English', 'الإنجليزية')}</option>
                </select>
              </label>
              <label>
                {tr(locale, 'Logo (URL data image/* base64)', 'Logo (image/* data URL base64)', 'الشعار (رابط بيانات image/* base64)')}
                <input className="form-control" value={uiConfigForm.logoDataUrl} type="text" placeholder="data:image/png;base64,..." onChange={(event) => setUiConfigForm((current) => ({ ...current, logoDataUrl: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Effacer logo personnalise', 'Clear custom logo', 'مسح الشعار المخصص')}
                <select className="form-select" value={uiConfigForm.clearLogo ? '1' : '0'} onChange={(event) => setUiConfigForm((current) => ({ ...current, clearLogo: event.target.value === '1' }))}>
                  <option value="0">{tr(locale, 'Non', 'No', 'لا')}</option>
                  <option value="1">{tr(locale, 'Oui', 'Yes', 'نعم')}</option>
                </select>
              </label>
              <div className="settings-action-row">
                <button className="btn primary" type="button" onClick={saveUiConfig}>{tr(locale, 'Enregistrer branding + langue', 'Save branding + language', 'حفظ الهوية + اللغة')}</button>
                <button className="btn" type="button" onClick={refreshUIConfig}>{tr(locale, 'Recharger', 'Reload', 'إعادة التحميل')}</button>
              </div>
            </div>
          </div>

          <div className="form-card settings-admin-card">
            <h2>{tr(locale, 'Gestion utilisateurs', 'User management', 'إدارة المستخدمين')}</h2>
            <div className="form-grid compact">
              <label>
                {tr(locale, 'Identifiant', 'Username', 'اسم المستخدم')}
                <input className="form-control" value={newUser.username} type="text" autoComplete="off" onChange={(event) => setNewUser((current) => ({ ...current, username: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Mot de passe', 'Password', 'كلمة المرور')}
                <input className="form-control" value={newUser.password} type="password" autoComplete="new-password" onChange={(event) => setNewUser((current) => ({ ...current, password: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Role', 'Role', 'الدور')}
                <select className="form-select" value={newUser.role} onChange={(event) => setNewUser((current) => ({ ...current, role: event.target.value }))}>
                  <option value="user">{roleLabel(locale, 'user')}</option>
                  <option value="triage">{roleLabel(locale, 'triage')}</option>
                  <option value="reception">{roleLabel(locale, 'reception')}</option>
                  <option value="dechocage">{roleLabel(locale, 'dechocage')}</option>
                  <option value="admin">{roleLabel(locale, 'admin')}</option>
                </select>
              </label>
              <div className="settings-action-row">
                <button className="btn" type="button" onClick={() => setNewUser((current) => ({ ...current, password: buildStrongPassword() }))}>{tr(locale, 'Generer mot de passe', 'Generate password', 'توليد كلمة مرور')}</button>
                <button className="btn primary" type="button" onClick={createUser}>{tr(locale, 'Creer le compte', 'Create account', 'إنشاء الحساب')}</button>
                <button className="btn" type="button" onClick={refreshUsers}>{tr(locale, 'Rafraichir utilisateurs', 'Refresh users', 'تحديث المستخدمين')}</button>
              </div>
            </div>
          </div>

          <div className="form-card settings-admin-card">
            <h2>{tr(locale, 'Reinitialiser mot de passe', 'Reset password', 'إعادة تعيين كلمة المرور')}</h2>
            <div className="form-grid compact">
              <label>
                {tr(locale, 'Utilisateur', 'User', 'المستخدم')}
                <input className="form-control" value={resetPasswordForm.username} type="text" autoComplete="off" onChange={(event) => setResetPasswordForm((current) => ({ ...current, username: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Nouveau mot de passe', 'New password', 'كلمة مرور جديدة')}
                <input className="form-control" value={resetPasswordForm.newPassword} type="password" autoComplete="new-password" onChange={(event) => setResetPasswordForm((current) => ({ ...current, newPassword: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Confirmer', 'Confirm', 'تأكيد')}
                <input className="form-control" value={resetPasswordForm.confirmPassword} type="password" autoComplete="new-password" onChange={(event) => setResetPasswordForm((current) => ({ ...current, confirmPassword: event.target.value }))} />
              </label>
              <button className="btn" type="button" onClick={() => {
                const generated = buildStrongPassword();
                setResetPasswordForm((current) => ({ ...current, newPassword: generated, confirmPassword: generated }));
              }}>
                {tr(locale, 'Generer mot de passe', 'Generate password', 'توليد كلمة مرور')}
              </button>
              <button className="btn primary" type="button" onClick={resetUserPassword}>{tr(locale, 'Mettre a jour', 'Update', 'تحديث')}</button>
            </div>
          </div>

          <div className="table-wrap settings-wide">
            <table className="table table-sm align-middle mb-0">
              <thead>
                <tr>
                  <th>{tr(locale, 'Identifiant', 'Username', 'اسم المستخدم')}</th>
                  <th>{tr(locale, 'Role', 'Role', 'الدور')}</th>
                  <th>{tr(locale, 'Action', 'Action', 'الإجراء')}</th>
                </tr>
              </thead>
              <tbody>{renderUsers}</tbody>
            </table>
          </div>
        </div>
      ) : null}

      {activeSection === 'security' ? (
        <div className="settings-panel-grid">
          <div className="form-card settings-wide">
            <h2>{tr(locale, 'Configuration securite', 'Security configuration', 'إعدادات الأمان')}</h2>
            <div className="settings-stack">
              <div className="form-grid compact">
              <label>
                Admin bootstrap username
                <input className="form-control" value={securityConfigForm.adminInitUsername} type="text" onChange={(event) => setSecurityConfigForm((current) => ({ ...current, adminInitUsername: event.target.value }))} />
              </label>
              <label>
                Admin bootstrap password
                <input className="form-control" value={securityConfigForm.adminInitPassword} type="password" placeholder={securityConfigForm.adminInitPasswordConfigured ? tr(locale, 'Deja configure', 'Already configured', 'مضبوط مسبقًا') : tr(locale, 'Entrer mot de passe bootstrap', 'Enter bootstrap password', 'أدخل كلمة مرور التهيئة')} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, adminInitPassword: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Reinitialiser admin bootstrap password', 'Reset admin bootstrap password', 'إعادة ضبط كلمة مرور مسؤول التهيئة')}
                <select className="form-select" value={securityConfigForm.clearAdminInitPassword ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, clearAdminInitPassword: event.target.value === '1' }))}>
                  <option value="0">{tr(locale, 'Non', 'No', 'لا')}</option>
                  <option value="1">{tr(locale, 'Oui', 'Yes', 'نعم')}</option>
                </select>
              </label>
              <label>
                FORCE_SECURE_COOKIE
                <select className="form-select" value={securityConfigForm.forceSecureCookie ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, forceSecureCookie: event.target.value === '1' }))}>
                  <option value="1">true</option>
                  <option value="0">false</option>
                </select>
              </label>
              <label>
                TRUST_PROXY_HEADERS
                <select className="form-select" value={securityConfigForm.trustProxyHeaders ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, trustProxyHeaders: event.target.value === '1' }))}>
                  <option value="1">true</option>
                  <option value="0">false</option>
                </select>
              </label>
              <label>
                ENABLE_HSTS
                <select className="form-select" value={securityConfigForm.enableHsts ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, enableHsts: event.target.value === '1' }))}>
                  <option value="1">true</option>
                  <option value="0">false</option>
                </select>
              </label>
              <label>
                HSTS_MAX_AGE
                <input className="form-control" value={securityConfigForm.hstsMaxAge} type="number" min="0" onChange={(event) => setSecurityConfigForm((current) => ({ ...current, hstsMaxAge: event.target.value }))} />
              </label>
              <label>
                HSTS_INCLUDE_SUBDOMAINS
                <select className="form-select" value={securityConfigForm.hstsIncludeSubdomains ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, hstsIncludeSubdomains: event.target.value === '1' }))}>
                  <option value="1">true</option>
                  <option value="0">false</option>
                </select>
              </label>
              <label>
                HSTS_PRELOAD
                <select className="form-select" value={securityConfigForm.hstsPreload ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, hstsPreload: event.target.value === '1' }))}>
                  <option value="1">true</option>
                  <option value="0">false</option>
                </select>
              </label>
              <label>
                TRIAGE_SLA_MINUTES
                <input className="form-control" value={securityConfigForm.triageSlaMinutes} type="number" min="1" max="240" onChange={(event) => setSecurityConfigForm((current) => ({ ...current, triageSlaMinutes: event.target.value }))} />
              </label>
              <label>
                GOTIFY_TOKEN_ENC_KEY (base64)
                <input className="form-control" value={securityConfigForm.gotifyTokenEncKey} type="password" placeholder={securityConfigForm.gotifyTokenEncKeyConfigured ? tr(locale, 'Deja configure', 'Already configured', 'مضبوط مسبقًا') : tr(locale, 'Entrer cle base64', 'Enter base64 key', 'أدخل مفتاح base64')} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, gotifyTokenEncKey: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Proxy sortant actif', 'Outbound proxy enabled', 'تفعيل الوكيل الخارجي')}
                <select className="form-select" value={securityConfigForm.proxyEnabled ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, proxyEnabled: event.target.value === '1' }))}>
                  <option value="0">{tr(locale, 'Non', 'No', 'لا')}</option>
                  <option value="1">{tr(locale, 'Oui', 'Yes', 'نعم')}</option>
                </select>
              </label>
              <label>
                {tr(locale, 'Proxy URL (http://ip:port)', 'Proxy URL (http://ip:port)', 'رابط الوكيل (http://ip:port)')}
                <input className="form-control" value={securityConfigForm.proxyUrl} type="text" placeholder="http://proxy.example.com:8080" onChange={(event) => setSecurityConfigForm((current) => ({ ...current, proxyUrl: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Utilisateur proxy', 'Proxy username', 'اسم مستخدم الوكيل')}
                <input className="form-control" value={securityConfigForm.proxyUsername} type="text" onChange={(event) => setSecurityConfigForm((current) => ({ ...current, proxyUsername: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Mot de passe proxy', 'Proxy password', 'كلمة مرور الوكيل')}
                <input className="form-control" value={securityConfigForm.proxyPassword} type="password" placeholder={securityConfigForm.proxyPasswordConfigured ? tr(locale, 'Deja configure', 'Already configured', 'مضبوط مسبقًا') : tr(locale, 'Entrer mot de passe proxy', 'Enter proxy password', 'أدخل كلمة مرور الوكيل')} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, proxyPassword: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Reinitialiser mot de passe proxy', 'Reset proxy password', 'إعادة ضبط كلمة مرور الوكيل')}
                <select className="form-select" value={securityConfigForm.clearProxyPassword ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, clearProxyPassword: event.target.value === '1' }))}>
                  <option value="0">{tr(locale, 'Non', 'No', 'لا')}</option>
                  <option value="1">{tr(locale, 'Oui', 'Yes', 'نعم')}</option>
                </select>
              </label>
              <label>
                {tr(locale, 'Verification signature callback', 'Callback signature verification', 'التحقق من توقيع الاستدعاء')}
                <select className="form-select" value={securityConfigForm.alertCallbackSignatureRequired ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, alertCallbackSignatureRequired: event.target.value === '1' }))}>
                  <option value="1">{tr(locale, 'Obligatoire', 'Required', 'إلزامي')}</option>
                  <option value="0">{tr(locale, 'Desactive', 'Disabled', 'معطل')}</option>
                </select>
              </label>
              <label>
                {tr(locale, 'Secret callback alertes', 'Alert callback secret', 'سر استدعاء التنبيه')}
                <input className="form-control" value={securityConfigForm.alertCallbackSecret} type="password" placeholder={securityConfigForm.alertCallbackSecretConfigured ? tr(locale, 'Deja configure', 'Already configured', 'مضبوط مسبقًا') : tr(locale, 'Entrer secret partage', 'Enter shared secret', 'أدخل سرًا مشتركًا')} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, alertCallbackSecret: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'IP allowlist callback (CSV/CIDR)', 'Callback IP allowlist (CSV/CIDR)', 'قائمة سماح IP للاستدعاء (CSV/CIDR)')}
                <input className="form-control" value={securityConfigForm.alertCallbackIpAllowlist} type="text" placeholder="127.0.0.1,10.0.0.0/24" onChange={(event) => setSecurityConfigForm((current) => ({ ...current, alertCallbackIpAllowlist: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Reinitialiser secret callback', 'Reset callback secret', 'إعادة ضبط سر الاستدعاء')}
                <select className="form-select" value={securityConfigForm.clearAlertCallbackSecret ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, clearAlertCallbackSecret: event.target.value === '1' }))}>
                  <option value="0">{tr(locale, 'Non', 'No', 'لا')}</option>
                  <option value="1">{tr(locale, 'Oui', 'Yes', 'نعم')}</option>
                </select>
              </label>
              <label>
                {tr(locale, 'Reinitialiser GOTIFY_TOKEN_ENC_KEY', 'Reset GOTIFY_TOKEN_ENC_KEY', 'إعادة ضبط GOTIFY_TOKEN_ENC_KEY')}
                <select className="form-select" value={securityConfigForm.clearGotifyTokenEncKey ? '1' : '0'} onChange={(event) => setSecurityConfigForm((current) => ({ ...current, clearGotifyTokenEncKey: event.target.value === '1' }))}>
                  <option value="0">{tr(locale, 'Non', 'No', 'لا')}</option>
                  <option value="1">{tr(locale, 'Oui', 'Yes', 'نعم')}</option>
                </select>
              </label>
              <div className="settings-action-row">
                <button className="btn primary" type="button" onClick={saveSecurityConfig}>{tr(locale, 'Enregistrer securite', 'Save security', 'حفظ الأمان')}</button>
                <button className="btn" type="button" onClick={refreshSecurityConfig}>{tr(locale, 'Recharger config', 'Reload config', 'إعادة تحميل الإعدادات')}</button>
                <button className="btn" type="button" onClick={refreshSecurityHealth}>{tr(locale, 'Audit instantane', 'Run audit now', 'تدقيق فوري')}</button>
              </div>
              </div>
            </div>
          </div>

          <div className="form-card">
            <h2>{tr(locale, 'Sante securite', 'Security health', 'صحة الأمان')}</h2>
            <div className="form-grid compact">
              <div>
                <span className={`security-chip ${healthStatus}`}>{tr(locale, 'Statut global', 'Global status', 'الحالة العامة')}: {healthStatus.toUpperCase()}</span>
                {securityHealth?.error ? <p className="small-note">{escapeText(securityHealth.error)}</p> : null}
                {!securityHealth?.loaded ? <p className="small-note">{tr(locale, 'Aucune mesure chargee pour le moment.', 'No security measurements loaded yet.', 'لم يتم تحميل قياسات الأمان بعد.')}</p> : null}
              </div>
              <button className="btn" type="button" onClick={refreshSecurityHealth}>{tr(locale, 'Rafraichir audit securite', 'Refresh security audit', 'تحديث تدقيق الأمان')}</button>
            </div>
          </div>

          <div className="table-wrap settings-wide">
            <table className="table table-sm align-middle mb-0">
              <thead>
                <tr>
                  <th>{tr(locale, 'Controle', 'Check', 'التحقق')}</th>
                  <th>{tr(locale, 'Statut', 'Status', 'الحالة')}</th>
                  <th>{tr(locale, 'Detail', 'Details', 'التفاصيل')}</th>
                  <th>{tr(locale, 'Recommandation', 'Recommendation', 'التوصية')}</th>
                </tr>
              </thead>
              <tbody>
                {checks.length ? checks.map((entry, index) => (
                  <tr key={`${entry.name || 'check'}-${index}`}>
                    <td>{escapeText(entry.name || '-')}</td>
                    <td>
                      <span className={`security-chip inline ${(entry.status || 'unknown').toLowerCase()}`}>
                        {escapeText((entry.status || 'unknown').toUpperCase())}
                      </span>
                    </td>
                    <td>{escapeText(entry.details || '-')}</td>
                    <td>{escapeText(entry.recommendation || '-')}</td>
                  </tr>
                )) : (
                  <tr><td colSpan="4"><div className="empty">{tr(locale, 'Aucun controle securite disponible.', 'No security checks available.', 'لا توجد فحوصات أمان متاحة.')}</div></td></tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}

      {activeSection === 'integrations' ? (
        <div className="settings-panel-grid">
          <div className="form-card settings-focus-card">
            <h2>{tr(locale, 'Integration Gotify', 'Gotify integration', 'تكامل Gotify')}</h2>
            <p className="small-note">{tr(locale, 'Configurer URL, token et priorite. Le token est requis si integration active.', 'Configure URL, token and priority. Token is required when integration is enabled.', 'قم بتكوين الرابط والرمز والأولوية. الرمز مطلوب عند تفعيل التكامل.')}</p>
            <div className="form-grid compact">
              <label>
                {tr(locale, 'Activation', 'Enable', 'تفعيل')}
                <select className="form-select" value={gotifyForm.enabled ? '1' : '0'} onChange={(event) => setGotifyForm((current) => ({ ...current, enabled: event.target.value === '1' }))}>
                  <option value="0">{tr(locale, 'Desactive', 'Disabled', 'معطل')}</option>
                  <option value="1">{tr(locale, 'Active', 'Enabled', 'مفعل')}</option>
                </select>
              </label>
              <label>
                {tr(locale, 'URL serveur', 'Server URL', 'رابط الخادم')}
                <input className="form-control" value={gotifyForm.url} type="text" placeholder="https://gotify.example.com" onChange={(event) => setGotifyForm((current) => ({ ...current, url: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Token application', 'Application token', 'رمز التطبيق')}
                <input className="form-control" value={gotifyForm.token} type="password" placeholder={gotifyForm.tokenConfigured ? tr(locale, 'Token deja configure', 'Token already configured', 'الرمز مضبوط مسبقًا') : tr(locale, 'Entrer un token', 'Enter a token', 'أدخل رمزًا')} onChange={(event) => setGotifyForm((current) => ({ ...current, token: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Priorite', 'Priority', 'الأولوية')}
                <input className="form-control" value={gotifyForm.priority} type="number" min="1" max="10" onChange={(event) => setGotifyForm((current) => ({ ...current, priority: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Reinitialiser token', 'Reset token', 'إعادة ضبط الرمز')}
                <select className="form-select" value={gotifyForm.clearToken ? '1' : '0'} onChange={(event) => setGotifyForm((current) => ({ ...current, clearToken: event.target.value === '1' }))}>
                  <option value="0">{tr(locale, 'Non', 'No', 'لا')}</option>
                  <option value="1">{tr(locale, 'Oui', 'Yes', 'نعم')}</option>
                </select>
              </label>
              <div className="settings-action-row">
                <button className="btn primary" type="button" onClick={saveGotifySettings}>{tr(locale, 'Enregistrer Gotify', 'Save Gotify', 'حفظ Gotify')}</button>
                <button className="btn" type="button" onClick={testGotifySettings}>{tr(locale, 'Tester Gotify', 'Test Gotify', 'اختبار Gotify')}</button>
                <button className="btn" type="button" onClick={refreshGotifySettings}>{tr(locale, 'Recharger', 'Reload', 'إعادة التحميل')}</button>
              </div>
            </div>
            <p className="small-note">{tr(locale, 'Conseil: sauvegarder puis tester pour valider URL/token.', 'Tip: save then test to validate URL/token.', 'نصيحة: احفظ ثم اختبر للتحقق من الرابط/الرمز.')}</p>
          </div>

          <div className="form-card settings-focus-card">
            <h2>{tr(locale, 'Import patient (JSON)', 'Patient import (JSON)', 'استيراد المرضى (JSON)')}</h2>
            <p className="small-note">{tr(locale, 'Format attendu: {"patients": [{"registrationNumber": "...", "name": "...", "patientType": "medical", "triageScore": 2}]}', 'Expected format: {"patients": [{"registrationNumber": "...", "name": "...", "patientType": "medical", "triageScore": 2}]}', 'التنسيق المتوقع: {"patients": [{"registrationNumber": "...", "name": "...", "patientType": "medical", "triageScore": 2}]}')}</p>
            <div className="form-grid compact">
              <label>
                {tr(locale, 'Source import', 'Import source', 'مصدر الاستيراد')}
                <input className="form-control" value={patientImportForm.source} type="text" onChange={(event) => setPatientImportForm((current) => ({ ...current, source: event.target.value }))} />
              </label>
              <label>
                {tr(locale, 'Payload JSON', 'JSON payload', 'بيانات JSON')}
                <textarea className="form-control" rows="8" value={patientImportForm.json} onChange={(event) => setPatientImportForm((current) => ({ ...current, json: event.target.value }))} />
              </label>
              <div className="settings-action-row">
                <button className="btn primary" type="button" onClick={importPatients}>{tr(locale, 'Importer patients', 'Import patients', 'استيراد المرضى')}</button>
              </div>
            </div>
          </div>

          <div className="form-card settings-focus-card">
            <h2>{tr(locale, 'Canaux SMS / WhatsApp', 'SMS / WhatsApp channels', 'قنوات SMS / WhatsApp')}</h2>
            <p className="small-note">{tr(locale, 'Configurer les webhooks de sortie et les destinataires. Chaque envoi enregistre un statut d accusé.', 'Configure outbound webhooks and recipients. Each send stores an acknowledgment status.', 'قم بتكوين webhooks الصادرة والمستلمين. كل إرسال يسجل حالة تأكيد.')}</p>
            <div className="form-grid compact">
              <label>
                {tr(locale, 'SMS actif', 'SMS enabled', 'تفعيل SMS')}
                <select className="form-select" value={alertChannelsForm.sms.enabled ? '1' : '0'} onChange={(event) => setAlertChannelsForm((current) => ({ ...current, sms: { ...current.sms, enabled: event.target.value === '1' } }))}>
                  <option value="0">{tr(locale, 'Non', 'No', 'لا')}</option>
                  <option value="1">{tr(locale, 'Oui', 'Yes', 'نعم')}</option>
                </select>
              </label>
              <label>
                {tr(locale, 'SMS webhook URL', 'SMS webhook URL', 'رابط SMS webhook')}
                <input className="form-control" value={alertChannelsForm.sms.webhookUrl} type="text" placeholder="https://gateway.example.com/sms" onChange={(event) => setAlertChannelsForm((current) => ({ ...current, sms: { ...current.sms, webhookUrl: event.target.value } }))} />
              </label>
              <label>
                {tr(locale, 'Destinataire SMS', 'SMS recipient', 'مستلم SMS')}
                <input className="form-control" value={alertChannelsForm.sms.recipient} type="text" placeholder="+212600000000" onChange={(event) => setAlertChannelsForm((current) => ({ ...current, sms: { ...current.sms, recipient: event.target.value } }))} />
              </label>

              <label>
                {tr(locale, 'WhatsApp actif', 'WhatsApp enabled', 'تفعيل WhatsApp')}
                <select className="form-select" value={alertChannelsForm.whatsapp.enabled ? '1' : '0'} onChange={(event) => setAlertChannelsForm((current) => ({ ...current, whatsapp: { ...current.whatsapp, enabled: event.target.value === '1' } }))}>
                  <option value="0">{tr(locale, 'Non', 'No', 'لا')}</option>
                  <option value="1">{tr(locale, 'Oui', 'Yes', 'نعم')}</option>
                </select>
              </label>
              <label>
                {tr(locale, 'WhatsApp webhook URL', 'WhatsApp webhook URL', 'رابط WhatsApp webhook')}
                <input className="form-control" value={alertChannelsForm.whatsapp.webhookUrl} type="text" placeholder="https://gateway.example.com/whatsapp" onChange={(event) => setAlertChannelsForm((current) => ({ ...current, whatsapp: { ...current.whatsapp, webhookUrl: event.target.value } }))} />
              </label>
              <label>
                {tr(locale, 'Destinataire WhatsApp', 'WhatsApp recipient', 'مستلم WhatsApp')}
                <input className="form-control" value={alertChannelsForm.whatsapp.recipient} type="text" placeholder="+212600000000" onChange={(event) => setAlertChannelsForm((current) => ({ ...current, whatsapp: { ...current.whatsapp, recipient: event.target.value } }))} />
              </label>

              <div className="settings-action-row">
                <button className="btn primary" type="button" onClick={saveAlertChannelsSettings}>{tr(locale, 'Enregistrer canaux', 'Save channels', 'حفظ القنوات')}</button>
                <button className="btn" type="button" onClick={testAlertChannelsSettings}>{tr(locale, 'Tester canaux', 'Test channels', 'اختبار القنوات')}</button>
                <button className="btn" type="button" onClick={refreshAlertChannelsSettings}>{tr(locale, 'Recharger canaux', 'Reload channels', 'إعادة تحميل القنوات')}</button>
                <button className="btn" type="button" onClick={() => refreshAlertNotifications({ announce: true })}>{tr(locale, 'Rafraichir accusés', 'Refresh acknowledgments', 'تحديث التأكيدات')}</button>
                <button className="btn" type="button" onClick={acknowledgeAllAlertNotifications}>{tr(locale, 'Tout accuser', 'Acknowledge all', 'تأكيد الكل')}</button>
              </div>
            </div>
          </div>

          <div className="table-wrap settings-wide">
            <table className="table table-sm align-middle mb-0">
              <thead>
                <tr>
                  <th>{tr(locale, 'Canal', 'Channel', 'القناة')}</th>
                  <th>{tr(locale, 'Destinataire', 'Recipient', 'المستلم')}</th>
                  <th>{tr(locale, 'Message', 'Message', 'الرسالة')}</th>
                  <th>{tr(locale, 'Statut', 'Status', 'الحالة')}</th>
                  <th>{tr(locale, 'Heure', 'Time', 'الوقت')}</th>
                  <th>{tr(locale, 'Action', 'Action', 'الإجراء')}</th>
                </tr>
              </thead>
              <tbody>
                {Array.isArray(alertNotifications) && alertNotifications.length ? alertNotifications.map((entry) => (
                  <tr key={entry.id || `${entry.channel}-${entry.createdAt}`}>
                    <td>{escapeText((entry.channel || '-').toUpperCase())}</td>
                    <td>{escapeText(entry.recipient || '-')}</td>
                    <td>{escapeText(entry.message || '-')}</td>
                    <td>{escapeText(entry.status || '-')}</td>
                    <td>{entry.createdAt ? new Date(entry.createdAt).toLocaleString(locale) : '-'}</td>
                    <td>
                      {String(entry.status || '').toLowerCase() !== 'acknowledged' ? (
                        <button className="mini-btn" type="button" onClick={() => acknowledgeAlertNotification(entry.id)}>
                          {tr(locale, 'Accuser', 'Acknowledge', 'تأكيد')}
                        </button>
                      ) : (
                        <span className="small-note">{tr(locale, 'OK', 'OK', 'تم')}</span>
                      )}
                    </td>
                  </tr>
                )) : (
                  <tr><td colSpan="6"><div className="empty">{tr(locale, 'Aucune notification sortante.', 'No outbound notifications.', 'لا توجد إشعارات صادرة.')}</div></td></tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}

      {activeSection === 'operations' ? (
        <div className="settings-panel-grid">
          <div className="form-card">
            <h2>{tr(locale, 'Sauvegarde / restauration', 'Backup / restore', 'النسخ الاحتياطي / الاستعادة')}</h2>
            <div className="settings-action-row">
              <button className="btn primary" type="button" onClick={createBackup}>{tr(locale, 'Sauvegarde 1 clic', 'One-click backup', 'نسخ احتياطي بنقرة واحدة')}</button>
              <button className="btn" type="button" onClick={restoreLatestBackup}>{tr(locale, 'Restaurer derniere sauvegarde', 'Restore latest backup', 'استعادة آخر نسخة احتياطية')}</button>
              <button className="btn" type="button" onClick={exportAuditCsv}>{tr(locale, 'Exporter audit CSV', 'Export audit CSV', 'تصدير تدقيق CSV')}</button>
              <button className="btn" type="button" onClick={exportFHIRBundle}>{tr(locale, 'Exporter bundle FHIR', 'Export FHIR bundle', 'تصدير حزمة FHIR')}</button>
              <button className="btn" type="button" onClick={refreshAuditLogs}>{tr(locale, 'Rafraichir journal', 'Refresh log', 'تحديث السجل')}</button>
            </div>
            <p className="small-note">{tr(locale, 'Derniere sauvegarde', 'Latest backup', 'آخر نسخة احتياطية')}: {lastBackupFile ? escapeText(lastBackupFile) : tr(locale, 'Aucune', 'None', 'لا يوجد')}</p>
          </div>

          <div className="table-wrap settings-wide">
            <table className="table table-sm align-middle mb-0">
              <thead>
                <tr>
                  <th>{tr(locale, 'Heure', 'Time', 'الوقت')}</th>
                  <th>{tr(locale, 'Utilisateur', 'User', 'المستخدم')}</th>
                  <th>{tr(locale, 'Action', 'Action', 'الإجراء')}</th>
                  <th>{tr(locale, 'Objet', 'Object', 'العنصر')}</th>
                </tr>
              </thead>
              <tbody>
                {auditLogs.length ? auditLogs.map((entry) => (
                  <tr key={entry.id || `${entry.createdAt}-${entry.action}-${entry.entityKey}`}>
                    <td>{entry.createdAt ? new Date(entry.createdAt).toLocaleString(locale) : '-'}</td>
                    <td>{escapeText(entry.username || 'system')}</td>
                    <td>{escapeText(entry.action)}</td>
                    <td>{escapeText(entry.entityKey || entry.entity)}</td>
                  </tr>
                )) : (
                  <tr><td colSpan="4"><div className="empty">{tr(locale, 'Aucune action journalisee.', 'No actions logged.', 'لا توجد إجراءات مسجلة.')}</div></td></tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}
    </div>
  );
}
