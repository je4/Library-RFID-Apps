/**
 * Encode UI View Module
 * Encapsulates DOM element manipulation, modals, notification banners,
 * and form controls for RFID encoding.
 */

import { t } from '../i18n.js';

export class EncodeUI {
    constructor() {
        this.notificationTimeout = null;
    }

    getElement(id) {
        return document.getElementById(id);
    }

    /**
     * Show notification alert banner at top of table.
     * @param {string} message 
     * @param {'info'|'success'|'error'} [type='info'] 
     * @param {number} [autoHideMs=6000] 
     */
    showNotification(message, type = 'info', autoHideMs = 6000) {
        const actionNotification = this.getElement('action-notification');
        const actionNotificationText = this.getElement('action-notification-text');
        if (!actionNotification || !actionNotificationText) return;

        if (this.notificationTimeout) {
            clearTimeout(this.notificationTimeout);
            this.notificationTimeout = null;
        }

        actionNotificationText.textContent = message;
        const alertClass = type === 'error' ? 'alert-error' : (type === 'success' ? 'alert-success' : 'alert-info');
        actionNotification.className = `alert ${alertClass} text-xs p-3 shadow-sm rounded-lg flex items-center justify-between ` +
            (type === 'error' ? 'app-notification-error' : (type === 'success' ? 'app-notification-success' : ''));
        actionNotification.classList.remove('hidden');

        if (autoHideMs > 0) {
            this.notificationTimeout = setTimeout(() => {
                actionNotification.classList.add('hidden');
            }, autoHideMs);
        }
    }

    /**
     * Hide notification alert banner.
     */
    hideNotification() {
        const actionNotification = this.getElement('action-notification');
        if (actionNotification) {
            actionNotification.classList.add('hidden');
        }
        if (this.notificationTimeout) {
            clearTimeout(this.notificationTimeout);
            this.notificationTimeout = null;
        }
    }

    /**
     * Display progress or waiting modal dialog.
     * @param {Object} options
     * @param {string} options.title
     * @param {string} options.message
     * @param {string} options.stepBadge
     * @param {string} options.statusText
     * @param {boolean} [options.isError=false]
     */
    showProgressModal({ title, message, stepBadge, statusText, isError = false }) {
        const modal = this.getElement('encode-modal-overlay');
        if (!modal) return;

        const titleEl = this.getElement('encode-modal-title');
        const msgEl = this.getElement('encode-modal-message');
        const badgeEl = this.getElement('encode-modal-step-badge');
        const statusEl = this.getElement('encode-modal-status-text');
        const spinnerRing = this.getElement('encode-spinner-ring');

        if (titleEl && title) titleEl.textContent = title;
        if (msgEl && message) msgEl.textContent = message;
        if (badgeEl && stepBadge) badgeEl.textContent = stepBadge;
        if (statusEl && statusText) statusEl.textContent = statusText;

        if (spinnerRing) {
            if (isError) {
                spinnerRing.classList.add('hidden');
            } else {
                spinnerRing.classList.remove('hidden');
            }
        }

        if (typeof modal.showModal === 'function' && !modal.open) {
            try { modal.showModal(); } catch (_) { modal.setAttribute('open', ''); }
        }
        modal.classList.add('active', 'modal-open');
        modal.setAttribute('aria-hidden', 'false');
    }

    /**
     * Close progress and waiting modal dialog.
     */
    hideProgressModal() {
        const modal = this.getElement('encode-modal-overlay');
        if (modal) {
            if (typeof modal.close === 'function' && modal.open) {
                try { modal.close(); } catch (_) { modal.removeAttribute('open'); }
            }
            modal.classList.remove('active', 'modal-open');
            modal.setAttribute('aria-hidden', 'true');
        }
    }

    /**
     * Display overwrite confirmation modal.
     * @param {string} uid 
     * @param {string} oldItemID 
     */
    showOverwriteModal(uid, oldItemID) {
        const modal = this.getElement('encode-overwrite-overlay');
        if (!modal) return;

        const uidEl = this.getElement('overwrite-uid');
        const oldItemEl = this.getElement('overwrite-old-item');

        if (uidEl) uidEl.textContent = uid || '';
        if (oldItemEl) oldItemEl.textContent = oldItemID || t('encode.unknown');

        if (typeof modal.showModal === 'function' && !modal.open) {
            try { modal.showModal(); } catch (_) { modal.setAttribute('open', ''); }
        }
        modal.classList.add('active', 'modal-open');
        modal.setAttribute('aria-hidden', 'false');
    }

    /**
     * Close overwrite confirmation modal.
     */
    hideOverwriteModal() {
        const modal = this.getElement('encode-overwrite-overlay');
        if (modal) {
            if (typeof modal.close === 'function' && modal.open) {
                try { modal.close(); } catch (_) { modal.removeAttribute('open'); }
            }
            modal.classList.remove('active', 'modal-open');
            modal.setAttribute('aria-hidden', 'true');
        }
    }

    /**
     * Set AFI switch/segmented controls status.
     * @param {string|boolean} afiValue 
     */
    setAFIStatus(afiValue) {
        const afiInput = this.getElement('encode-afi-input');
        const afiCheckbox = this.getElement('encode-afi-checkbox');
        const afiIcon = this.getElement('encode-afi-icon');
        const afiLabelText = this.getElement('encode-afi-label-text');
        const unsecureBtn = this.getElement('encode-afi-btn-unsecure');
        const secureBtn = this.getElement('encode-afi-btn-secure');
        const statusBadge = this.getElement('encode-afi-status-badge');

        const isSecure = (afiValue === '07' || afiValue === 'secure' || afiValue === 'gesichert' || afiValue === true || afiValue === 'true');
        const normalized = isSecure ? '07' : 'C2';

        if (afiInput) {
            afiInput.value = normalized;
        }

        if (statusBadge) {
            if (normalized === '07') {
                statusBadge.textContent = t('encode.securedBadge');
                statusBadge.className = 'badge badge-xs badge-info font-mono';
            } else {
                statusBadge.textContent = t('encode.unsecuredBadge');
                statusBadge.className = 'badge badge-xs badge-warning font-mono';
            }
        }

        if (afiCheckbox) {
            afiCheckbox.checked = isSecure;
        }

        if (afiLabelText) {
            afiLabelText.textContent = t(isSecure ? 'encode.secured' : 'encode.unsecured');
        }

        if (afiIcon) {
            if (isSecure) {
                afiIcon.classList.add('icon-lock-closed', 'text-primary');
                afiIcon.classList.remove('icon-lock-open', 'text-warning');
            } else {
                afiIcon.classList.add('icon-lock-open', 'text-warning');
                afiIcon.classList.remove('icon-lock-closed', 'text-primary');
            }
        }

        if (unsecureBtn) {
            if (normalized === 'C2') {
                unsecureBtn.classList.add('btn-primary', 'app-afi-active');
                unsecureBtn.classList.remove('btn-ghost');
                unsecureBtn.setAttribute('aria-checked', 'true');
            } else {
                unsecureBtn.classList.remove('btn-primary', 'app-afi-active');
                unsecureBtn.classList.add('btn-ghost');
                unsecureBtn.setAttribute('aria-checked', 'false');
            }
        }

        if (secureBtn) {
            if (normalized === '07') {
                secureBtn.classList.add('btn-primary', 'app-afi-active');
                secureBtn.classList.remove('btn-ghost');
                secureBtn.setAttribute('aria-checked', 'true');
            } else {
                secureBtn.classList.remove('btn-primary', 'app-afi-active');
                secureBtn.classList.add('btn-ghost');
                secureBtn.setAttribute('aria-checked', 'false');
            }
        }
    }

    /**
     * Update parts selector UI (pills 1..5 and dropdown 6..12).
     * @param {number|string} partsCount 
     */
    updatePartsSelection(partsCount) {
        const partsInput = this.getElement('encode-parts-input');
        const partsSelect = this.getElement('encode-parts-select');
        const partPills = document.querySelectorAll('.encode-part-btn');

        const count = parseInt(partsCount, 10) || 1;
        if (partsInput) {
            partsInput.value = count;
        }

        // Update pill buttons 1..5
        partPills.forEach((pill) => {
            const pillVal = parseInt(pill.getAttribute('data-parts'), 10);
            if (pillVal === count) {
                pill.classList.add('app-part-active', 'btn-primary');
                pill.classList.remove('btn-ghost');
                pill.setAttribute('aria-checked', 'true');
            } else {
                pill.classList.remove('app-part-active', 'btn-primary');
                pill.classList.add('btn-ghost');
                pill.setAttribute('aria-checked', 'false');
            }
        });

        // Update dropdown 6..12
        if (partsSelect) {
            if (count >= 6 && count <= 12) {
                partsSelect.value = String(count);
            } else {
                partsSelect.value = '';
            }
        }
    }

    /**
     * Extract active transponders currently listed in the DOM table.
     * @returns {Array<{uid: string, itemId: string, hasExistingBarcode: boolean}>}
     */
    getActiveTransponders() {
        const rows = Array.from(document.querySelectorAll('#loan-table-body tr[data-uid]'));
        return rows.map((row) => {
            const uid = (row.getAttribute('data-uid') || '').trim();
            const rawItemID = (row.getAttribute('data-itemid') || '').trim();
            const hasExisting = rawItemID.length > 0 && !rawItemID.toLowerCase().includes('keine');
            return {
                uid: uid,
                itemId: rawItemID,
                hasExistingBarcode: hasExisting
            };
        }).filter(t => t.uid.length > 0);
    }

    getBarcodeInputValue() {
        const input = this.getElement('encode-item-id');
        return input ? input.value.trim() : '';
    }

    getISILInputValue() {
        const input = this.getElement('encode-isil');
        return input ? input.value.trim() : '';
    }

    getAFIInputValue() {
        const checkbox = this.getElement('encode-afi-checkbox');
        if (checkbox) {
            return checkbox.checked ? '07' : 'C2';
        }
        const input = this.getElement('encode-afi-input');
        return input ? (input.value.trim() || '07') : '07';
    }

    getPartsInputValue() {
        const input = this.getElement('encode-parts-input');
        return input ? (parseInt(input.value, 10) || 1) : 1;
    }

    setStartButtonDisabled(disabled) {
        const btn = this.getElement('btn-encode-start');
        if (btn) {
            btn.disabled = Boolean(disabled);
        }
    }

    clearBarcodeField() {
        const input = this.getElement('encode-item-id');
        if (input) {
            input.value = '';
            input.focus();
        }
        this.setStartButtonDisabled(true);
    }
}
