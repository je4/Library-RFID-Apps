/**
 * Encode Controller Module for RFID Desk
 * Controls sequential ISO 28560-2 RFID tag encoding, multi-part state machine,
 * barcode input validation, overwrite protection dialogs, and waiting states.
 */

import { playBeep } from './audio.js';

let isEncodingRunning = false;
let userSelectedPartsManually = false;
let currentPartNo = 1;
let totalParts = 1;
let currentBarcode = '';
let currentISIL = '';
let currentAFI = '07';
let encodedUIDs = new Set();
let pendingTagUID = null;
let pendingTagOldItemID = '';

function getElement(id) {
    return document.getElementById(id);
}

export function showEncodeNotification(message, type = 'info', autoHideMs = 6000) {
    const actionNotification = getElement('action-notification');
    const actionNotificationText = getElement('action-notification-text');
    if (!actionNotification || !actionNotificationText) return;

    actionNotificationText.textContent = message;
    actionNotification.className = 'px-4 py-2.5 text-xs font-medium border-b transition-all flex items-center justify-between ' +
        (type === 'error' ? 'app-notification-error' : (type === 'success' ? 'app-notification-success' : 'bg-blue-50 dark:bg-blue-950/40 text-blue-700 dark:text-blue-300 border-blue-200 dark:border-blue-800'));
    actionNotification.classList.remove('hidden');

    if (autoHideMs > 0) {
        setTimeout(() => {
            actionNotification.classList.add('hidden');
        }, autoHideMs);
    }
}

export function updatePartsSelection(partsCount, isManual = true) {
    const partsInput = getElement('encode-parts-input');
    const partsSelect = getElement('encode-parts-select');
    const partPills = document.querySelectorAll('.encode-part-btn');

    if (isManual) {
        userSelectedPartsManually = true;
    }

    const count = parseInt(partsCount, 10) || 1;
    if (partsInput) {
        partsInput.value = count;
    }

    // Update pill buttons 1..5
    partPills.forEach((pill) => {
        const pillVal = parseInt(pill.getAttribute('data-parts'), 10);
        if (pillVal === count) {
            pill.classList.add('app-part-active');
            pill.setAttribute('aria-checked', 'true');
        } else {
            pill.classList.remove('app-part-active');
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

export function setAFIStatus(afiValue) {
    const afiInput = getElement('encode-afi-input');
    const afiCheckbox = getElement('encode-afi-checkbox');
    const unsecureBtn = getElement('encode-afi-btn-unsecure');
    const secureBtn = getElement('encode-afi-btn-secure');

    const normalized = (afiValue === '07' || afiValue === 'secure' || afiValue === 'gesichert' || afiValue === true) ? '07' : 'C2';

    if (afiInput) {
        afiInput.value = normalized;
    }

    if (afiCheckbox) {
        afiCheckbox.checked = (normalized === '07');
    }

    if (unsecureBtn) {
        if (normalized === 'C2') {
            unsecureBtn.classList.add('app-afi-active-unsecure');
            unsecureBtn.setAttribute('aria-checked', 'true');
        } else {
            unsecureBtn.classList.remove('app-afi-active-unsecure');
            unsecureBtn.setAttribute('aria-checked', 'false');
        }
    }

    if (secureBtn) {
        if (normalized === '07') {
            secureBtn.classList.add('app-afi-active-secure');
            secureBtn.setAttribute('aria-checked', 'true');
        } else {
            secureBtn.classList.remove('app-afi-active-secure');
            secureBtn.setAttribute('aria-checked', 'false');
        }
    }
}

export function getActiveTransponders() {
    const rows = Array.from(document.querySelectorAll('#loan-table-body tr[data-uid]'));
    return rows.map((row) => {
        const uid = row.getAttribute('data-uid') || '';
        const rawItemID = row.getAttribute('data-itemid') || '';
        const itemID = rawItemID.trim();
        return {
            uid: uid.trim(),
            itemId: itemID,
            hasExistingBarcode: itemID.length > 0 && !itemID.toLowerCase().includes('keine')
        };
    }).filter(t => t.uid.length > 0);
}

function updateAutoParts() {
    if (userSelectedPartsManually || isEncodingRunning) return;
    const activeTags = getActiveTransponders();
    const count = activeTags.length > 0 ? activeTags.length : 1;
    updatePartsSelection(count, false);
}

function showProgressModal(title, message, stepBadge, statusText) {
    const modal = getElement('encode-modal-overlay');
    if (!modal) return;

    const titleEl = getElement('encode-modal-title');
    const msgEl = getElement('encode-modal-message');
    const badgeEl = getElement('encode-modal-step-badge');
    const statusEl = getElement('encode-modal-status-text');

    if (titleEl && title) titleEl.textContent = title;
    if (msgEl && message) msgEl.textContent = message;
    if (badgeEl && stepBadge) badgeEl.textContent = stepBadge;
    if (statusEl && statusText) statusEl.textContent = statusText;

    modal.classList.add('active');
    modal.setAttribute('aria-hidden', 'false');
}

function hideProgressModal() {
    const modal = getElement('encode-modal-overlay');
    if (modal) {
        modal.classList.remove('active');
        modal.setAttribute('aria-hidden', 'true');
    }
}

function showOverwriteModal(uid, oldItemID) {
    const modal = getElement('encode-overwrite-overlay');
    if (!modal) return;

    pendingTagUID = uid;
    pendingTagOldItemID = oldItemID;

    const uidEl = getElement('overwrite-uid');
    const oldItemEl = getElement('overwrite-old-item');

    if (uidEl) uidEl.textContent = uid;
    if (oldItemEl) oldItemEl.textContent = oldItemID;

    modal.classList.add('active');
    modal.setAttribute('aria-hidden', 'false');
}

function hideOverwriteModal() {
    const modal = getElement('encode-overwrite-overlay');
    if (modal) {
        modal.classList.remove('active');
        modal.setAttribute('aria-hidden', 'true');
    }
    pendingTagUID = null;
    pendingTagOldItemID = '';
}

export function stopEncoding(userMessage = 'Kodierung abgebrochen.') {
    isEncodingRunning = false;
    hideProgressModal();
    hideOverwriteModal();
    if (userMessage) {
        showEncodeNotification(userMessage, 'info');
    }
}

async function writeSingleTag(uid, partNo, parts, barcode, isil, afi) {
    showProgressModal(
        'Medium wird kodiert...',
        `Schreibe Barcode ${barcode} auf Transponder ${uid}... Bitte nicht entfernen.`,
        `Teil ${partNo} von ${parts}`,
        'RFID-Schreibbefehl aktiv...'
    );

    try {
        const resp = await fetch('encode/write', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json'
            },
            body: JSON.stringify({
                uid: uid,
                itemId: barcode,
                isil: isil,
                parts: parts,
                partNo: partNo,
                afi: afi
            })
        });

        const result = await resp.json();
        if (resp.ok && result.status === 'success') {
            playBeep();
            encodedUIDs.add(uid);
            currentPartNo++;

            if (currentPartNo > totalParts) {
                // All parts successfully encoded!
                finishEncoding();
            } else {
                // Short transition delay before processing next part
                setTimeout(() => {
                    processNextTag();
                }, 400);
            }
        } else {
            const errorMsg = result.error || 'Fehler beim Schreiben des Transponders';
            showEncodeNotification(`Fehler bei Teil ${partNo}: ${errorMsg}`, 'error', 10000);
            stopEncoding(`Kodierung bei Teil ${partNo} fehlgeschlagen.`);
        }
    } catch (err) {
        console.error('WriteTag error:', err);
        showEncodeNotification(`Netzwerk- oder Serverfehler: ${err.message}`, 'error', 10000);
        stopEncoding(`Fehler bei Kodierung von Teil ${partNo}.`);
    }
}

function finishEncoding() {
    isEncodingRunning = false;
    playBeep();

    showProgressModal(
        'Kodierung erfolgreich!',
        `Alle ${totalParts} Teile für Barcode ${currentBarcode} wurden erfolgreich programmiert.`,
        `Fertiggestellt (${totalParts}/${totalParts})`,
        'Vorgang abgeschlossen'
    );

    showEncodeNotification(`Medium ${currentBarcode} (${totalParts} Teile) erfolgreich kodiert.`, 'success', 8000);

    // Auto-close modal after 2 seconds
    setTimeout(() => {
        hideProgressModal();
        const itemInput = getElement('encode-item-id');
        if (itemInput) {
            itemInput.value = '';
            itemInput.focus();
            const startBtn = getElement('btn-encode-start');
            if (startBtn) startBtn.disabled = true;
        }
        userSelectedPartsManually = false;
        updateAutoParts();
    }, 2200);
}

function processNextTag() {
    if (!isEncodingRunning) return;

    if (currentPartNo > totalParts) {
        finishEncoding();
        return;
    }

    const activeTags = getActiveTransponders();
    // Find next unencoded active transponder
    const nextTag = activeTags.find(t => t.uid && !encodedUIDs.has(t.uid));

    if (nextTag) {
        // If tag already contains a barcode different from the current one, prompt for overwrite
        if (nextTag.hasExistingBarcode && nextTag.itemId !== currentBarcode) {
            hideProgressModal();
            showOverwriteModal(nextTag.uid, nextTag.itemId);
        } else {
            // Write tag
            writeSingleTag(nextTag.uid, currentPartNo, totalParts, currentBarcode, currentISIL, currentAFI);
        }
    } else {
        // Waiting state for additional media
        showProgressModal(
            `Warte auf Medium ${currentPartNo} von ${totalParts}...`,
            `Bitte nächstes Medium (Teil ${currentPartNo} von ${totalParts}) auf das RFID-Lesegerät legen.`,
            `Warte auf Teil ${currentPartNo} von ${totalParts}`,
            'Warte auf Transponder...'
        );
    }
}

export function startEncoding() {
    const itemInput = getElement('encode-item-id');
    const isilInput = getElement('encode-isil');
    const afiInput = getElement('encode-afi-input');
    const partsInput = getElement('encode-parts-input');

    if (!itemInput) return;

    currentBarcode = itemInput.value.trim();
    if (!currentBarcode) {
        showEncodeNotification('Bitte geben Sie einen Barcode / ItemID ein.', 'error');
        itemInput.focus();
        return;
    }

    currentISIL = isilInput ? isilInput.value.trim() : '';
    const afiCheckbox = getElement('encode-afi-checkbox');
    if (afiCheckbox) {
        currentAFI = afiCheckbox.checked ? '07' : 'C2';
    } else {
        currentAFI = afiInput ? (afiInput.value.trim() || '07') : '07';
    }
    totalParts = partsInput ? (parseInt(partsInput.value, 10) || 1) : 1;
    currentPartNo = 1;
    encodedUIDs.clear();
    isEncodingRunning = true;

    showProgressModal(
        'Kodierung startet...',
        `Vorbereitung für Teil 1 von ${totalParts}...`,
        `Teil 1 von ${totalParts}`,
        'Initialisierung...'
    );

    setTimeout(() => {
        processNextTag();
    }, 200);
}

export function initEncodeController() {
    const itemInput = getElement('encode-item-id');
    const startBtn = getElement('btn-encode-start');
    const partsPills = document.querySelectorAll('.encode-part-btn');
    const partsSelect = getElement('encode-parts-select');
    const unsecureBtn = getElement('encode-afi-btn-unsecure');
    const secureBtn = getElement('encode-afi-btn-secure');
    const afiCheckbox = getElement('encode-afi-checkbox');
    const afiInput = getElement('encode-afi-input');
    const stopModalBtn = getElement('btn-encode-modal-stop');
    const confirmOverwriteBtn = getElement('btn-overwrite-confirm');
    const skipOverwriteBtn = getElement('btn-overwrite-skip');
    const cancelOverwriteBtn = getElement('btn-overwrite-cancel');

    // Only run on pages that have the encode form
    if (!itemInput && !startBtn) return;

    // AFI Checkbox event listener
    if (afiCheckbox) {
        afiCheckbox.addEventListener('change', () => {
            setAFIStatus(afiCheckbox.checked ? '07' : 'C2');
        });
    }

    // AFI Segmented Control event listeners (if present)
    if (unsecureBtn) {
        unsecureBtn.addEventListener('click', () => {
            setAFIStatus('C2');
        });
        unsecureBtn.addEventListener('keydown', (e) => {
            if (e.key === ' ' || e.key === 'Enter') {
                e.preventDefault();
                setAFIStatus('C2');
            }
        });
    }

    if (secureBtn) {
        secureBtn.addEventListener('click', () => {
            setAFIStatus('07');
        });
        secureBtn.addEventListener('keydown', (e) => {
            if (e.key === ' ' || e.key === 'Enter') {
                e.preventDefault();
                setAFIStatus('07');
            }
        });
    }

    if (afiCheckbox) {
        setAFIStatus(afiCheckbox.checked ? '07' : 'C2');
    } else if (afiInput && afiInput.value) {
        setAFIStatus(afiInput.value);
    }

    // Barcode input validation & start button activation
    if (itemInput && startBtn) {
        const validateInput = () => {
            const val = itemInput.value.trim();
            startBtn.disabled = val.length === 0;
        };

        itemInput.addEventListener('input', validateInput);
        itemInput.addEventListener('change', validateInput);
        itemInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                validateInput();
                if (!startBtn.disabled) {
                    startEncoding();
                }
            }
        });
        validateInput();
    }

    // Parts pills listeners
    partsPills.forEach((pill) => {
        pill.addEventListener('click', () => {
            const parts = pill.getAttribute('data-parts');
            updatePartsSelection(parts, true);
        });
    });

    // Parts dropdown listener
    if (partsSelect) {
        partsSelect.addEventListener('change', () => {
            if (partsSelect.value) {
                updatePartsSelection(partsSelect.value, true);
            }
        });
    }

    // Start Encoding Button
    if (startBtn) {
        startBtn.addEventListener('click', () => {
            startEncoding();
        });
    }

    // Stop / Abort Button in Progress Modal
    if (stopModalBtn) {
        stopModalBtn.addEventListener('click', () => {
            stopEncoding('Kodierung vom Benutzer gestoppt.');
        });
    }

    // Overwrite Dialog Buttons
    if (confirmOverwriteBtn) {
        confirmOverwriteBtn.addEventListener('click', () => {
            const uid = pendingTagUID;
            hideOverwriteModal();
            if (uid && isEncodingRunning) {
                writeSingleTag(uid, currentPartNo, totalParts, currentBarcode, currentISIL, currentAFI);
            }
        });
    }

    if (skipOverwriteBtn) {
        skipOverwriteBtn.addEventListener('click', () => {
            const uid = pendingTagUID;
            hideOverwriteModal();
            if (uid) {
                encodedUIDs.add(uid);
            }
            if (isEncodingRunning) {
                processNextTag();
            }
        });
    }

    if (cancelOverwriteBtn) {
        cancelOverwriteBtn.addEventListener('click', () => {
            stopEncoding('Kodierung abgebrochen.');
        });
    }

    // Auto-detect active parts on initial load
    updateAutoParts();

    // Hook into HTMX and SSE table swaps to trigger next step or auto-part count
    document.body.addEventListener('htmx:afterSwap', (evt) => {
        if (evt.detail && (evt.detail.target.id === 'loan-table-body' || evt.detail.target.closest('#loan-table-body'))) {
            updateAutoParts();
            if (isEncodingRunning) {
                processNextTag();
            }
        }
    });

    document.body.addEventListener('htmx:sseMessage', () => {
        setTimeout(() => {
            updateAutoParts();
            if (isEncodingRunning) {
                processNextTag();
            }
        }, 50);
    });

    // MutationObserver fallback to catch table updates
    const tableBody = getElement('loan-table-body');
    if (tableBody) {
        const observer = new MutationObserver(() => {
            updateAutoParts();
            if (isEncodingRunning) {
                processNextTag();
            }
        });
        observer.observe(tableBody, { childList: true, subtree: true });
    }
}
