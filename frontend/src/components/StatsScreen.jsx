import { tr } from '../lib/i18n';

export default function StatsScreen({ stats, locale }) {
  return (
    <div className="screen active">
      <div className="controls-grid">
        <div className="form-card">
          <h2>{tr(locale, 'Statistiques', 'Statistics', 'الإحصاءات')}</h2>
          <p className="small-note">{tr(locale, "Consultations, archivage, triage, capacite lits, et metriques d'exploitation.", 'Consultations, archive flow, triage, bed capacity, and operations metrics.', 'الاستشارات والأرشفة والفرز وسعة الأسرة ومؤشرات التشغيل.')}</p>
        </div>
      </div>
      <div className="table-wrap">
        <table className="table table-sm align-middle mb-0">
          <thead>
            <tr>
              <th>{tr(locale, 'Date', 'Date', 'التاريخ')}</th>
              <th>{tr(locale, 'Consultations', 'Consultations', 'الاستشارات')}</th>
            </tr>
          </thead>
          <tbody>
            {(stats.consultationsByDate || []).length ? (stats.consultationsByDate || []).map((item) => (
              <tr key={item.date}><td>{item.date}</td><td>{item.count}</td></tr>
            )) : (
              <tr><td colSpan="2"><div className="empty">{tr(locale, 'Aucune consultation enregistree.', 'No consultations recorded.', 'لا توجد استشارات مسجلة.')}</div></td></tr>
            )}
          </tbody>
        </table>
        <div className="stats-grid" style={{ marginTop: 16 }}>
          <div className="stat"><span>{tr(locale, 'Patients archives', 'Archived patients', 'المرضى المؤرشفون')}</span><strong>{stats.archivedPatients || 0}</strong></div>
          <div className="stat"><span>{tr(locale, 'Total consultations', 'Total consultations', 'إجمالي الاستشارات')}</span><strong>{stats.totalConsultations || 0}</strong></div>
          <div className="stat"><span>{tr(locale, 'Duree moyenne (min)', 'Average duration (min)', 'المدة المتوسطة (دقيقة)')}</span><strong>{Math.round(stats.avgConsultationMinutes) || 0}</strong></div>
          <div className="stat"><span>{tr(locale, 'Lits en alerte', 'Beds in alert', 'الأسرة في حالة إنذار')}</span><strong>{stats.alertBeds || 0}</strong></div>
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
