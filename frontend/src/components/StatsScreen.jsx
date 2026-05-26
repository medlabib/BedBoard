export default function StatsScreen({ stats }) {
  return (
    <div className="screen active">
      <div className="controls-grid">
        <div className="form-card">
          <h2>Statistiques</h2>
          <p className="small-note">Consultations, archivage, triage, capacite lits, et metriques d'exploitation.</p>
        </div>
      </div>
      <div className="table-wrap">
        <table className="table table-sm align-middle mb-0">
          <thead>
            <tr>
              <th>Date</th>
              <th>Consultations</th>
            </tr>
          </thead>
          <tbody>
            {(stats.consultationsByDate || []).length ? (stats.consultationsByDate || []).map((item) => (
              <tr key={item.date}><td>{item.date}</td><td>{item.count}</td></tr>
            )) : (
              <tr><td colSpan="2"><div className="empty">Aucune consultation enregistree.</div></td></tr>
            )}
          </tbody>
        </table>
        <div className="stats-grid" style={{ marginTop: 16 }}>
          <div className="stat"><span>Patients archives</span><strong>{stats.archivedPatients || 0}</strong></div>
          <div className="stat"><span>Total consultations</span><strong>{stats.totalConsultations || 0}</strong></div>
          <div className="stat"><span>Duree moyenne (min)</span><strong>{Math.round(stats.avgConsultationMinutes) || 0}</strong></div>
          <div className="stat"><span>Lits en alerte</span><strong>{stats.alertBeds || 0}</strong></div>
          <div className="stat"><span>Triage 0</span><strong>{stats.triageByLevel?.['0'] || 0}</strong></div>
          <div className="stat"><span>Triage 1</span><strong>{stats.triageByLevel?.['1'] || 0}</strong></div>
          <div className="stat"><span>Triage 2</span><strong>{stats.triageByLevel?.['2'] || 0}</strong></div>
          <div className="stat"><span>Triage 3</span><strong>{stats.triageByLevel?.['3'] || 0}</strong></div>
          <div className="stat"><span>Triage 4</span><strong>{stats.triageByLevel?.['4'] || 0}</strong></div>
        </div>
      </div>
    </div>
  );
}
