import { tr } from '../lib/i18n';

export default function AccountScreen({ passwordForm, setPasswordForm, changeOwnPassword, user, locale }) {
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
        </div>
      </div>
    </div>
  );
}
