import { tr } from '../lib/i18n';

export default function SimpleModal({ title, text, onCancel, onConfirm, cancelText, confirmText, locale }) {
  return (
    <div className="modal-backdrop open" role="dialog" aria-modal="true">
      <div className="modal section-card">
        <h2>{title}</h2>
        {text ? <p className="small-note">{text}</p> : null}
        <div className="modal-actions">
          <button className="btn" type="button" onClick={onCancel}>{cancelText || tr(locale, 'Annuler', 'Cancel', 'إلغاء')}</button>
          <button className="btn primary" type="button" onClick={onConfirm}>{confirmText || tr(locale, 'Confirmer', 'Confirm', 'تأكيد')}</button>
        </div>
      </div>
    </div>
  );
}
