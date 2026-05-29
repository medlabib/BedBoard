import { tr } from '../lib/i18n';

export default function ReceptionPage({ logout, openPatientPage, locale, brandName, brandLogo }) {
  return (
    <div className="app">
      <div className="navbar">
        <div className="nav-left">
          <div className="brand-mark"><img src={brandLogo || '/logo.svg'} alt={tr(locale, "Logo de l'hopital", 'Hospital logo', 'شعار المستشفى')} /></div>
          <div className="nav-title">
            <strong>{brandName} - {tr(locale, 'Reception', 'Reception', 'استقبال')}</strong>
            <span>{tr(locale, 'Acces limite a la vue patient', 'Access limited to patient view', 'الوصول محدود لعرض المرضى')}</span>
          </div>
        </div>
        <div className="nav-actions">
          <button className="btn primary" type="button" onClick={openPatientPage}>{tr(locale, 'Ouvrir la page patient', 'Open patient page', 'فتح صفحة المرضى')}</button>
          <button className="btn" type="button" onClick={logout}>{tr(locale, 'Deconnexion', 'Logout', 'تسجيل الخروج')}</button>
        </div>
      </div>
    </div>
  );
}
