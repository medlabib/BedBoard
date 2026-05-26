export default function SimpleModal({ title, text, onCancel, onConfirm, cancelText, confirmText }) {
  return (
    <div className="modal-backdrop open" role="dialog" aria-modal="true">
      <div className="modal section-card">
        <h2>{title}</h2>
        {text ? <p className="small-note">{text}</p> : null}
        <div className="modal-actions">
          <button className="btn" type="button" onClick={onCancel}>{cancelText || 'Annuler'}</button>
          <button className="btn primary" type="button" onClick={onConfirm}>{confirmText || 'Confirmer'}</button>
        </div>
      </div>
    </div>
  );
}
