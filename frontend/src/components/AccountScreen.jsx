export default function AccountScreen({ passwordForm, setPasswordForm, changeOwnPassword, user }) {
  return (
    <div className="screen active">
      <div className="controls-grid">
        <div className="form-card">
          <h2>Changer mon mot de passe</h2>
          <div className="form-grid">
            <label>
              Mot de passe actuel
              <input className="form-control" value={passwordForm.currentPassword} type="password" onChange={(event) => setPasswordForm((current) => ({ ...current, currentPassword: event.target.value }))} />
            </label>
            <label>
              Nouveau mot de passe
              <input className="form-control" value={passwordForm.newPassword} type="password" onChange={(event) => setPasswordForm((current) => ({ ...current, newPassword: event.target.value }))} />
            </label>
            <label>
              Confirmer
              <input className="form-control" value={passwordForm.confirmPassword} type="password" onChange={(event) => setPasswordForm((current) => ({ ...current, confirmPassword: event.target.value }))} />
            </label>
            <button className="btn primary" type="button" onClick={changeOwnPassword}>Mettre a jour</button>
          </div>
        </div>
        <div className="form-card">
          <h2>Compte</h2>
          <p className="small-note">Connecte en tant que {String(user.username || '')}.</p>
        </div>
      </div>
    </div>
  );
}
