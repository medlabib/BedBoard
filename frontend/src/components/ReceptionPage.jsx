export default function ReceptionPage({ logout, openPatientPage }) {
  return (
    <div className="app">
      <div className="navbar">
        <div className="nav-left">
          <div className="brand-mark"><img src="/logo.svg" alt="Logo de l'hopital" /></div>
          <div className="nav-title">
            <strong>BedBoard - Reception</strong>
            <span>Acces limite a la vue patient</span>
          </div>
        </div>
        <div className="nav-actions">
          <button className="btn primary" type="button" onClick={openPatientPage}>Ouvrir la page patient</button>
          <button className="btn" type="button" onClick={logout}>Deconnexion</button>
        </div>
      </div>
    </div>
  );
}
