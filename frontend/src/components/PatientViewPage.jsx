import { tr } from '../lib/i18n';

export default function PatientViewPage({
  openMainPage,
  patientPanel,
  locale,
  brandName,
  brandLogo,
}) {
  return (
    <div className="app patient-page-shell">
      <div className="navbar">
        <div className="nav-left">
          <div className="brand-mark"><img src={brandLogo || '/logo.svg'} alt={tr(locale, "Logo de l'hopital", 'Hospital logo', 'شعار المستشفى')} /></div>
          <div className="nav-title">
            <strong>{brandName} - {tr(locale, 'Vue salle', 'Room view', 'عرض القاعة')}</strong>
            <span>{tr(locale, "Affichage salle d'attente", 'Waiting room display', 'شاشة غرفة الانتظار')}</span>
          </div>
        </div>
        <div className="nav-actions">
          <button className="btn" type="button" onClick={openMainPage}>{tr(locale, 'Retour tableau', 'Back to board', 'العودة إلى اللوحة')}</button>
        </div>
      </div>
      <div className="section-card patient-page-card">
        <div className="patient-full">
          {patientPanel}
        </div>
      </div>
    </div>
  );
}
