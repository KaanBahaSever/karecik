import { AlertTriangle } from 'lucide-react'
import Modal from './Modal.jsx'

/**
 * Confirmation dialog for irreversible actions such as deletion.
 *
 * @param {boolean}  open
 * @param {Function} onClose
 * @param {Function} onConfirm   - Called when the user confirms
 * @param {string}   title
 * @param {string}   message
 * @param {string}   confirmText - Label of the confirm button
 * @param {boolean}  busy        - Disables the buttons while the request runs
 */
export default function ConfirmModal({
  open,
  onClose,
  onConfirm,
  title = 'Emin misiniz?',
  message = 'Bu işlem geri alınamaz.',
  confirmText = 'Sil',
  busy = false,
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      width="max-w-md"
      footer={
        <>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={busy}>
            Vazgeç
          </button>
          <button
            type="button"
            className="btn bg-red-600 text-white hover:bg-red-700"
            onClick={onConfirm}
            disabled={busy}
          >
            {busy ? 'İşleniyor...' : confirmText}
          </button>
        </>
      }
    >
      <div className="flex gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-red-50">
          <AlertTriangle className="h-5 w-5 text-red-600" aria-hidden="true" />
        </div>
        <p className="pt-2 text-sm leading-relaxed text-gray-600">{message}</p>
      </div>
    </Modal>
  )
}
