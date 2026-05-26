export default function SettingsScreen({
  newUser,
  setNewUser,
  createUser,
  resetPasswordForm,
  setResetPasswordForm,
  resetUserPassword,
  createBackup,
  restoreLatestBackup,
  lastBackupFile,
  gotifyForm,
  setGotifyForm,
  saveGotifySettings,
  securityHealth,
  refreshSecurityHealth,
  renderUsers,
  auditLogs,
  escapeText,
}) {
  const healthStatus = String(securityHealth?.status || 'unknown').toLowerCase();
  const checks = Array.isArray(securityHealth?.checks) ? securityHealth.checks : [];

  return (
    <div className="screen active">
      <div className="controls-grid">
        <div className="form-card">
          <h2>Ajouter un utilisateur</h2>
          <div className="form-grid">
            <label>
              Identifiant
              <input className="form-control" value={newUser.username} type="text" onChange={(event) => setNewUser((current) => ({ ...current, username: event.target.value }))} />
            </label>
            <label>
              Mot de passe
              <input className="form-control" value={newUser.password} type="password" onChange={(event) => setNewUser((current) => ({ ...current, password: event.target.value }))} />
            </label>
            <label>
              Role
              <select className="form-select" value={newUser.role} onChange={(event) => setNewUser((current) => ({ ...current, role: event.target.value }))}>
                <option value="user">user</option>
                <option value="triage">triage</option>
                <option value="reception">reception</option>
                <option value="dechocage">dechocage</option>
                <option value="admin">admin</option>
              </select>
            </label>
            <button className="btn primary" type="button" onClick={createUser}>Creer</button>
          </div>
        </div>
        <div className="form-card">
          <h2>Changer mot de passe utilisateur</h2>
          <div className="form-grid">
            <label>
              Utilisateur
              <input className="form-control" value={resetPasswordForm.username} type="text" onChange={(event) => setResetPasswordForm((current) => ({ ...current, username: event.target.value }))} />
            </label>
            <label>
              Nouveau mot de passe
              <input className="form-control" value={resetPasswordForm.newPassword} type="password" onChange={(event) => setResetPasswordForm((current) => ({ ...current, newPassword: event.target.value }))} />
            </label>
            <label>
              Confirmer
              <input className="form-control" value={resetPasswordForm.confirmPassword} type="password" onChange={(event) => setResetPasswordForm((current) => ({ ...current, confirmPassword: event.target.value }))} />
            </label>
            <button className="btn primary" type="button" onClick={resetUserPassword}>Mettre a jour</button>
          </div>
        </div>
        <div className="form-card">
          <h2>Utilisateurs</h2>
          <p className="small-note">Les comptes crees peuvent se connecter directement depuis la barre du haut.</p>
        </div>
        <div className="form-card">
          <h2>Sauvegarde / restauration</h2>
          <div className="form-grid">
            <button className="btn primary" type="button" onClick={createBackup}>Sauvegarde 1 clic</button>
            <button className="btn" type="button" onClick={restoreLatestBackup}>Restaurer derniere sauvegarde</button>
          </div>
          <p className="small-note">Derniere sauvegarde: {lastBackupFile ? escapeText(lastBackupFile) : 'Aucune'}</p>
        </div>
        <div className="form-card">
          <h2>Integration Gotify</h2>
          <div className="form-grid">
            <label>
              Activation
              <select className="form-select" value={gotifyForm.enabled ? '1' : '0'} onChange={(event) => setGotifyForm((current) => ({ ...current, enabled: event.target.value === '1' }))}>
                <option value="0">Desactive</option>
                <option value="1">Active</option>
              </select>
            </label>
            <label>
              URL serveur
              <input className="form-control" value={gotifyForm.url} type="text" placeholder="https://gotify.example.com" onChange={(event) => setGotifyForm((current) => ({ ...current, url: event.target.value }))} />
            </label>
            <label>
              Token application
              <input className="form-control" value={gotifyForm.token} type="password" placeholder={gotifyForm.tokenConfigured ? 'Token deja configure' : 'Entrer un token'} onChange={(event) => setGotifyForm((current) => ({ ...current, token: event.target.value }))} />
            </label>
            <label>
              Priorite
              <input className="form-control" value={gotifyForm.priority} type="number" min="1" max="10" onChange={(event) => setGotifyForm((current) => ({ ...current, priority: event.target.value }))} />
            </label>
            <label>
              Reinitialiser token
              <select className="form-select" value={gotifyForm.clearToken ? '1' : '0'} onChange={(event) => setGotifyForm((current) => ({ ...current, clearToken: event.target.value === '1' }))}>
                <option value="0">Non</option>
                <option value="1">Oui</option>
              </select>
            </label>
            <button className="btn primary" type="button" onClick={saveGotifySettings}>Enregistrer Gotify</button>
          </div>
          <p className="small-note">Le token n'est jamais renvoye au frontend. Laisser vide conserve le token actuel.</p>
        </div>
        <div className="form-card">
          <h2>Sante securite</h2>
          <div className="form-grid compact">
            <div>
              <span className={`security-chip ${healthStatus}`}>Statut global: {healthStatus.toUpperCase()}</span>
              {securityHealth?.error ? <p className="small-note">{escapeText(securityHealth.error)}</p> : null}
              {!securityHealth?.loaded ? <p className="small-note">Aucune mesure chargee pour le moment.</p> : null}
            </div>
            <button className="btn" type="button" onClick={refreshSecurityHealth}>Rafraichir audit securite</button>
          </div>
          <p className="small-note">Source: GET /api/admin/security/health (admin seulement).</p>
        </div>
      </div>
      <div className="table-wrap">
        <table className="table table-sm align-middle mb-0">
          <thead>
            <tr>
              <th>Identifiant</th>
              <th>Role</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>{renderUsers}</tbody>
        </table>
      </div>
      <div className="table-wrap" style={{ marginTop: 16 }}>
        <table className="table table-sm align-middle mb-0">
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
                <td>{entry.createdAt ? new Date(entry.createdAt).toLocaleString() : '-'}</td>
                <td>{escapeText(entry.username || 'system')}</td>
                <td>{escapeText(entry.action)}</td>
                <td>{escapeText(entry.entityKey || entry.entity)}</td>
              </tr>
            )) : (
              <tr><td colSpan="4"><div className="empty">Aucune action journalisee.</div></td></tr>
            )}
          </tbody>
        </table>
      </div>
      <div className="table-wrap" style={{ marginTop: 16 }}>
        <table className="table table-sm align-middle mb-0">
          <thead>
            <tr>
              <th>Controle</th>
              <th>Statut</th>
              <th>Detail</th>
              <th>Recommandation</th>
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
              <tr><td colSpan="4"><div className="empty">Aucun controle securite disponible.</div></td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
