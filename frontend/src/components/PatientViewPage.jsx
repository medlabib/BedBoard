export default function PatientViewPage({
  callLanguage,
  setCallLanguage,
  callLanguageOptions,
  currentPatient,
  speakPatientCall,
  openMainPage,
  patientPanel,
}) {
  return (
    <div className="app patient-page-shell">
      <div className="navbar">
        <div className="nav-left">
          <div className="brand-mark"><img src="/logo.svg" alt="Logo de l'hopital" /></div>
          <div className="nav-title">
            <strong>BedBoard - Vue patient</strong>
            <span>Affichage salle d'attente</span>
          </div>
        </div>
        <div className="nav-actions">
          <select className="form-select" value={callLanguage} onChange={(event) => setCallLanguage(event.target.value)}>
            {callLanguageOptions.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
          <button className="btn" type="button" onClick={() => { if (currentPatient) speakPatientCall(currentPatient); }}>Rappeler patient</button>
          <button className="btn" type="button" onClick={openMainPage}>Retour tableau</button>
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
