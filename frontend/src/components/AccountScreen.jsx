import { tr } from '../lib/i18n';

const shortcutOptions = [
  { value: 'Space', label: 'Space' },
  { value: 'Shift+Space', label: 'Shift+Space' },
  { value: 'KeyB', label: 'B' },
  { value: 'KeyP', label: 'P' },
  { value: 'KeyR', label: 'R' },
  { value: 'Slash', label: '/' },
  { value: 'KeyG', label: 'G' },
  { value: 'KeyH', label: 'H' },
  { value: 'KeyJ', label: 'J' },
];

export default function AccountScreen({
  passwordForm,
  setPasswordForm,
  changeOwnPassword,
  user,
  keyboardShortcuts,
  setKeyboardShortcuts,
  locale,
}) {
  const shortcuts = keyboardShortcuts || {};

  const updateShortcut = (name, value) => {
    if (!setKeyboardShortcuts) return;
    setKeyboardShortcuts((current) => ({ ...current, [name]: value }));
  };

  return (
    <div className="screen active">
      <div className="controls-grid">
        <div className="form-card">
          <h2>{tr(locale, 'Changer mon mot de passe', 'Change my password', 'تغيير كلمة مروري')}</h2>
          <div className="form-grid">
            <label>
              {tr(locale, 'Mot de passe actuel', 'Current password', 'كلمة المرور الحالية')}
              <input className="form-control" value={passwordForm.currentPassword} type="password" onChange={(event) => setPasswordForm((current) => ({ ...current, currentPassword: event.target.value }))} />
            </label>
            <label>
              {tr(locale, 'Nouveau mot de passe', 'New password', 'كلمة المرور الجديدة')}
              <input className="form-control" value={passwordForm.newPassword} type="password" onChange={(event) => setPasswordForm((current) => ({ ...current, newPassword: event.target.value }))} />
            </label>
            <label>
              {tr(locale, 'Confirmer', 'Confirm', 'تأكيد')}
              <input className="form-control" value={passwordForm.confirmPassword} type="password" onChange={(event) => setPasswordForm((current) => ({ ...current, confirmPassword: event.target.value }))} />
            </label>
            <button className="btn primary" type="button" onClick={changeOwnPassword}>{tr(locale, 'Mettre a jour', 'Update', 'تحديث')}</button>
          </div>
        </div>
        <div className="form-card">
          <h2>{tr(locale, 'Compte', 'Account', 'الحساب')}</h2>
          <p className="small-note">{tr(locale, 'Connecte en tant que', 'Logged in as', 'متصل باسم')} {String(user.username || '')}.</p>
          <h3>{tr(locale, 'Raccourcis clavier', 'Keyboard shortcuts', 'اختصارات لوحة المفاتيح')}</h3>
          <div className="form-grid compact" style={{ marginTop: 8 }}>
            <label>
              {tr(locale, 'Affecter triage max', 'Assign highest triage', 'تخصيص أعلى فرز')}
              <select className="form-select" value={shortcuts.assignHighest || 'Space'} onChange={(event) => updateShortcut('assignHighest', event.target.value)}>
                {shortcutOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </label>
            <label>
              {tr(locale, 'Affecter plus ancien non vu', 'Assign oldest unseen', 'تخصيص الأقدم غير المرئي')}
              <select className="form-select" value={shortcuts.assignOldest || 'Shift+Space'} onChange={(event) => updateShortcut('assignOldest', event.target.value)}>
                {shortcutOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </label>
            <label>
              {tr(locale, 'Ouvrir Lits', 'Open Beds', 'فتح الأسرة')}
              <select className="form-select" value={shortcuts.goBeds || 'KeyB'} onChange={(event) => updateShortcut('goBeds', event.target.value)}>
                {shortcutOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </label>
            <label>
              {tr(locale, 'Ouvrir Patients', 'Open Patients', 'فتح المرضى')}
              <select className="form-select" value={shortcuts.goPatients || 'KeyP'} onChange={(event) => updateShortcut('goPatients', event.target.value)}>
                {shortcutOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </label>
            <label>
              {tr(locale, 'Rafraichir', 'Refresh', 'تحديث')}
              <select className="form-select" value={shortcuts.refresh || 'KeyR'} onChange={(event) => updateShortcut('refresh', event.target.value)}>
                {shortcutOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </label>
            <label>
              {tr(locale, 'Aide raccourcis', 'Shortcuts help', 'مساعدة الاختصارات')}
              <select className="form-select" value={shortcuts.help || 'Slash'} onChange={(event) => updateShortcut('help', event.target.value)}>
                {shortcutOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </label>
          </div>
          <p className="small-note">{tr(locale, 'Palette commandes: Ctrl/Cmd + K.', 'Command palette: Ctrl/Cmd + K.', 'لوحة الأوامر: Ctrl/Cmd + K.')}</p>
        </div>
      </div>
    </div>
  );
}
