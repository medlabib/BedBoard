import { tr } from '../lib/i18n';

export default function StatsScreen({ stats, locale }) {
  const statusEntries = Object.entries(stats.patientsByStatus || {});
  const typeEntries = Object.entries(stats.patientsByType || {});
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
          <div className="stat"><span>{tr(locale, 'Attente moyenne triage (min)', 'Avg wait to triage (min)', 'متوسط انتظار الفرز (دقيقة)')}</span><strong>{Math.round(stats.avgWaitToTriageMinutes) || 0}</strong></div>
          <div className="stat"><span>{tr(locale, 'Attente moyenne assignation (min)', 'Avg wait to assignment (min)', 'متوسط انتظار التخصيص (دقيقة)')}</span><strong>{Math.round(stats.avgWaitToAssignMinutes) || 0}</strong></div>
          <div className="stat"><span>{tr(locale, 'Breach SLA triage', 'Triage SLA breaches', 'تجاوزات SLA للفرز')}</span><strong>{stats.triageSlaBreaches || 0}</strong></div>
          <div className="stat"><span>{tr(locale, 'Lits en alerte', 'Beds in alert', 'الأسرة في حالة إنذار')}</span><strong>{stats.alertBeds || 0}</strong></div>
          <div className="stat"><span>Triage 0</span><strong>{stats.triageByLevel?.['0'] || 0}</strong></div>
          <div className="stat"><span>Triage 1</span><strong>{stats.triageByLevel?.['1'] || 0}</strong></div>
          <div className="stat"><span>Triage 2</span><strong>{stats.triageByLevel?.['2'] || 0}</strong></div>
          <div className="stat"><span>Triage 3</span><strong>{stats.triageByLevel?.['3'] || 0}</strong></div>
          <div className="stat"><span>Triage 4</span><strong>{stats.triageByLevel?.['4'] || 0}</strong></div>
        </div>

        <div className="controls-grid" style={{ marginTop: 16 }}>
          <div className="form-card">
            <h2>{tr(locale, 'Consultations par heure', 'Consultations by hour', 'الاستشارات حسب الساعة')}</h2>
            <div className="table-wrap">
              <table className="table table-sm align-middle mb-0">
                <thead>
                  <tr>
                    <th>{tr(locale, 'Heure', 'Hour', 'الساعة')}</th>
                    <th>{tr(locale, 'Consultations', 'Consultations', 'الاستشارات')}</th>
                  </tr>
                </thead>
                <tbody>
                  {(stats.consultationsByHour || []).length ? (stats.consultationsByHour || []).map((item) => (
                    <tr key={item.hour}><td>{item.hour}</td><td>{item.count}</td></tr>
                  )) : (
                    <tr><td colSpan="2"><div className="empty">{tr(locale, 'Aucune donnee horaire.', 'No hourly data.', 'لا توجد بيانات حسب الساعة.')}</div></td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <div className="form-card">
            <h2>{tr(locale, 'Patients par statut', 'Patients by status', 'المرضى حسب الحالة')}</h2>
            <div className="table-wrap">
              <table className="table table-sm align-middle mb-0">
                <thead>
                  <tr>
                    <th>{tr(locale, 'Statut', 'Status', 'الحالة')}</th>
                    <th>{tr(locale, 'Total', 'Total', 'الإجمالي')}</th>
                  </tr>
                </thead>
                <tbody>
                  {statusEntries.length ? statusEntries.map(([key, value]) => (
                    <tr key={key}><td>{key}</td><td>{value}</td></tr>
                  )) : (
                    <tr><td colSpan="2"><div className="empty">{tr(locale, 'Aucun statut disponible.', 'No status data.', 'لا توجد بيانات حالة.')}</div></td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <div className="form-card">
            <h2>{tr(locale, 'Patients par type', 'Patients by type', 'المرضى حسب النوع')}</h2>
            <div className="table-wrap">
              <table className="table table-sm align-middle mb-0">
                <thead>
                  <tr>
                    <th>{tr(locale, 'Type', 'Type', 'النوع')}</th>
                    <th>{tr(locale, 'Total', 'Total', 'الإجمالي')}</th>
                  </tr>
                </thead>
                <tbody>
                  {typeEntries.length ? typeEntries.map(([key, value]) => (
                    <tr key={key}><td>{key}</td><td>{value}</td></tr>
                  )) : (
                    <tr><td colSpan="2"><div className="empty">{tr(locale, 'Aucun type disponible.', 'No type data.', 'لا توجد بيانات نوع.')}</div></td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
