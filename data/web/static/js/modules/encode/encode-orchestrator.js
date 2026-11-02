/**
 * Encode Orchestrator Module
 * Coordinates the State Machine, API Client, and UI View Controller
 * to provide race-condition-free sequential ISO 28560-2 RFID tag encoding.
 */

import { EncodeStateMachine, EncodeState } from './encode-state.js';
import { EncodeApiClient } from './encode-api.js';
import { EncodeUI } from './encode-ui.js';
import { playBeep } from '../audio.js';

export class EncodeOrchestrator {
    constructor(options = {}) {
        this.state = new EncodeStateMachine();
        this.api = new EncodeApiClient(options.basePath || '');
        this.ui = new EncodeUI();

        this.transitionTimer = null;
        this.updateDebounceTimer = null;
    }

    /**
     * Start encoding process with validation.
     */
    startEncoding() {
        if (this.state.isEncodingRunning) {
            return;
        }

        const barcode = this.ui.getBarcodeInputValue();
        if (!barcode) {
            this.ui.showNotification('Bitte geben Sie einen Barcode / ItemID ein.', 'error');
            const itemInput = this.ui.getElement('encode-item-id');
            if (itemInput) itemInput.focus();
            return;
        }

        const isil = this.ui.getISILInputValue();
        const afi = this.ui.getAFIInputValue();
        const parts = this.ui.getPartsInputValue();

        this.state.start(barcode, isil, afi, parts);

        this.ui.showProgressModal({
            title: 'Kodierung startet...',
            message: `Vorbereitung für Teil 1 von ${this.state.totalParts}...`,
            stepBadge: `Teil 1 von ${this.state.totalParts}`,
            statusText: 'Initialisierung...'
        });

        // Trigger first step
        setTimeout(() => {
            this.processNextTag();
        }, 150);
    }

    /**
     * Stop and abort the active encoding session.
     * @param {string} [userMessage='Kodierung abgebrochen.'] 
     */
    stopEncoding(userMessage = 'Kodierung abgebrochen.') {
        if (this.transitionTimer) {
            clearTimeout(this.transitionTimer);
            this.transitionTimer = null;
        }
        this.state.stop();
        this.ui.hideProgressModal();
        this.ui.hideOverwriteModal();
        if (userMessage) {
            this.ui.showNotification(userMessage, 'info');
        }
    }

    /**
     * Process next unencoded tag in sequence or transition to waiting state.
     */
    processNextTag() {
        if (!this.state.isEncodingRunning) {
            console.debug('[EncodeOrchestrator] processNextTag skipped: session not running');
            return;
        }

        // Concurrency guard: Do not trigger next step while a write is in flight or awaiting overwrite dialog
        if (this.state.isBusy()) {
            console.debug('[EncodeOrchestrator] processNextTag skipped: state is busy (inFlight: %s, state: %s)',
                this.state.isWriteInFlight, this.state.currentState);
            return;
        }

        if (this.state.isComplete()) {
            console.debug('[EncodeOrchestrator] All parts complete (%d/%d) -> finishing',
                this.state.currentPartNo - 1, this.state.totalParts);
            this.finishEncoding();
            return;
        }

        const activeTags = this.ui.getActiveTransponders();
        console.debug('[EncodeOrchestrator] Scanning active transponders (%d found):', activeTags.length, activeTags);

        // Look for candidate transponder that hasn't been encoded yet in this run
        const nextTag = activeTags.find(t => t.uid && !this.state.isTagAlreadyEncoded(t.uid));

        if (nextTag) {
            console.debug('[EncodeOrchestrator] Candidate transponder identified for part %d/%d: UID=%s, existingBarcode="%s"',
                this.state.currentPartNo, this.state.totalParts, nextTag.uid, nextTag.itemId);

            // Check for overwrite confirmation if tag has a different existing barcode
            if (nextTag.hasExistingBarcode && nextTag.itemId !== this.state.currentBarcode) {
                console.debug('[EncodeOrchestrator] Existing barcode conflict detected: tag "%s" vs target "%s". Prompting overwrite.',
                    nextTag.itemId, this.state.currentBarcode);
                this.state.setAwaitingOverwrite(nextTag.uid, nextTag.itemId);
                this.ui.hideProgressModal();
                this.ui.showOverwriteModal(nextTag.uid, nextTag.itemId);
            } else {
                this.executeWriteTag(nextTag.uid);
            }
        } else {
            // Waiting for user to place the next transponder on reader antenna pad
            console.debug('[EncodeOrchestrator] No unencoded transponder on antenna pad. Entering WAITING_TAG for part %d/%d',
                this.state.currentPartNo, this.state.totalParts);
            this.state.setWaitingForTag();
            this.ui.showProgressModal({
                title: `Warte auf Medium ${this.state.currentPartNo} von ${this.state.totalParts}...`,
                message: `Bitte nächstes Medium (Teil ${this.state.currentPartNo} von ${this.state.totalParts}) auf das RFID-Lesegerät legen.`,
                stepBadge: `Warte auf Teil ${this.state.currentPartNo} von ${this.state.totalParts}`,
                statusText: 'Warte auf Transponder...'
            });
        }
    }

    /**
     * Execute write operation for a specific tag UID.
     * @param {string} uid 
     */
    async executeWriteTag(uid) {
        if (!uid || !this.state.isEncodingRunning) return;

        this.state.setWritingInFlight(true);

        const currentPart = this.state.currentPartNo;
        const totalParts = this.state.totalParts;
        const barcode = this.state.currentBarcode;

        this.ui.showProgressModal({
            title: 'Medium wird kodiert...',
            message: `Schreibe Barcode ${barcode} auf Transponder ${uid}... Bitte nicht entfernen.`,
            stepBadge: `Teil ${currentPart} von ${totalParts}`,
            statusText: 'RFID-Schreibbefehl aktiv...'
        });

        const result = await this.api.writeTag({
            uid: uid,
            itemId: barcode,
            isil: this.state.currentISIL,
            parts: totalParts,
            partNo: currentPart,
            afi: this.state.currentAFI
        });

        if (result.success) {
            playBeep();
            this.state.recordSuccess(uid);

            if (this.state.isComplete()) {
                this.finishEncoding();
            } else {
                // Short transition delay before querying next tag
                this.transitionTimer = setTimeout(() => {
                    this.transitionTimer = null;
                    this.processNextTag();
                }, 350);
            }
        } else {
            this.state.setWritingInFlight(false);
            const errorMsg = result.error || 'Fehler beim Schreiben des Transponders';
            this.ui.showNotification(`Fehler bei Teil ${currentPart}: ${errorMsg}`, 'error', 10000);
            this.stopEncoding(`Kodierung bei Teil ${currentPart} fehlgeschlagen.`);
        }
    }

    /**
     * Finish encoding workflow with success notification and form reset.
     */
    finishEncoding() {
        const barcode = this.state.currentBarcode;
        const totalParts = this.state.totalParts;

        this.state.stop();
        playBeep();

        this.ui.showProgressModal({
            title: 'Kodierung erfolgreich!',
            message: `Alle ${totalParts} Teile für Barcode ${barcode} wurden erfolgreich programmiert.`,
            stepBadge: `Fertiggestellt (${totalParts}/${totalParts})`,
            statusText: 'Vorgang abgeschlossen'
        });

        this.ui.showNotification(`Medium ${barcode} (${totalParts} Teile) erfolgreich kodiert.`, 'success', 8000);

        setTimeout(() => {
            this.ui.hideProgressModal();
            this.ui.clearBarcodeField();
            this.state.userSelectedPartsManually = false;
            this.updateAutoParts();
        }, 2200);
    }

    /**
     * Confirm overwrite dialog for pending tag.
     */
    confirmOverwrite() {
        if (!this.state.pendingTag) return;
        const uid = this.state.pendingTag.rawUID || this.state.pendingTag.uid;
        this.state.clearPendingOverwrite();
        this.ui.hideOverwriteModal();

        if (uid && this.state.isEncodingRunning) {
            this.executeWriteTag(uid);
        }
    }

    /**
     * Skip pending tag in overwrite dialog.
     */
    skipOverwrite() {
        if (!this.state.pendingTag) return;
        this.state.skipPendingTag();
        this.ui.hideOverwriteModal();

        if (this.state.isEncodingRunning) {
            this.processNextTag();
        }
    }

    /**
     * Cancel overwrite dialog and stop session.
     */
    cancelOverwrite() {
        this.state.clearPendingOverwrite();
        this.stopEncoding('Kodierung abgebrochen.');
    }

    /**
     * Clear / wipe active transponders resting on reader.
     */
    async clearActiveTags() {
        if (this.state.isEncodingRunning) {
            return;
        }

        const activeTags = this.ui.getActiveTransponders();
        if (activeTags.length === 0) {
            this.ui.showNotification('Kein aktiver Transponder auf dem Lesegerät gefunden. Bitte ein Medium auflegen.', 'error');
            return;
        }

        const clearBtn = this.ui.getElement('btn-clear-tag');
        if (clearBtn) clearBtn.disabled = true;

        this.ui.showNotification('Transponder wird geleert (Felder löschen, AFI entsichern)...', 'info');

        let successCount = 0;
        let errorCount = 0;
        let lastError = '';

        for (const tag of activeTags) {
            const result = await this.api.clearTag(tag.uid, 'C2');
            if (result.success) {
                successCount++;
            } else {
                errorCount++;
                lastError = result.error || 'Fehler beim Leeren';
            }
        }

        if (clearBtn) clearBtn.disabled = false;

        if (successCount > 0) {
            playBeep();
            if (errorCount === 0) {
                this.ui.showNotification(
                    successCount === 1
                        ? 'Transponder erfolgreich geleert und entsichert (AFI: C2).'
                        : `Alle ${successCount} Transponder erfolgreich geleert und entsichert (AFI: C2).`,
                    'success'
                );
            } else {
                this.ui.showNotification(`${successCount} Transponder geleert, ${errorCount} fehlgeschlagen (${lastError}).`, 'error');
            }
        } else {
            this.ui.showNotification(`Fehler beim Leeren des Transponders: ${lastError}`, 'error', 8000);
        }
    }

    /**
     * Auto-detect and sync parts count from active transponders when not manually set.
     */
    updateAutoParts() {
        if (this.state.userSelectedPartsManually || this.state.isEncodingRunning) return;
        const activeTags = this.ui.getActiveTransponders();
        const count = activeTags.length > 0 ? activeTags.length : 1;
        this.ui.updatePartsSelection(count);
    }

    /**
     * Debounced handler for table updates, SSE messages, and DOM mutations.
     */
    handleTransponderUpdate() {
        if (this.updateDebounceTimer) {
            clearTimeout(this.updateDebounceTimer);
        }
        this.updateDebounceTimer = setTimeout(() => {
            this.updateDebounceTimer = null;
            this.updateAutoParts();
            if (this.state.isEncodingRunning && !this.state.isBusy()) {
                this.processNextTag();
            }
        }, 60);
    }

    /**
     * Bind all DOM listeners and initialize controllers.
     */
    init() {
        const itemInput = this.ui.getElement('encode-item-id');
        const startBtn = this.ui.getElement('btn-encode-start');
        const partsPills = document.querySelectorAll('.encode-part-btn');
        const partsSelect = this.ui.getElement('encode-parts-select');
        const unsecureBtn = this.ui.getElement('encode-afi-btn-unsecure');
        const secureBtn = this.ui.getElement('encode-afi-btn-secure');
        const afiCheckbox = this.ui.getElement('encode-afi-checkbox');
        const afiInput = this.ui.getElement('encode-afi-input');
        const stopModalBtn = this.ui.getElement('btn-encode-modal-stop');
        const confirmOverwriteBtn = this.ui.getElement('btn-overwrite-confirm');
        const skipOverwriteBtn = this.ui.getElement('btn-overwrite-skip');
        const cancelOverwriteBtn = this.ui.getElement('btn-overwrite-cancel');
        const clearBtn = this.ui.getElement('btn-clear-tag');

        if (!itemInput && !startBtn) return;

        // AFI Switch listeners
        if (afiCheckbox) {
            afiCheckbox.addEventListener('change', () => {
                this.ui.setAFIStatus(afiCheckbox.checked ? '07' : 'C2');
            });
        }

        if (unsecureBtn) {
            unsecureBtn.addEventListener('click', () => this.ui.setAFIStatus('C2'));
            unsecureBtn.addEventListener('keydown', (e) => {
                if (e.key === ' ' || e.key === 'Enter') {
                    e.preventDefault();
                    this.ui.setAFIStatus('C2');
                }
            });
        }

        if (secureBtn) {
            secureBtn.addEventListener('click', () => this.ui.setAFIStatus('07'));
            secureBtn.addEventListener('keydown', (e) => {
                if (e.key === ' ' || e.key === 'Enter') {
                    e.preventDefault();
                    this.ui.setAFIStatus('07');
                }
            });
        }

        if (afiCheckbox) {
            this.ui.setAFIStatus(afiCheckbox.checked ? '07' : 'C2');
        } else if (afiInput && afiInput.value) {
            this.ui.setAFIStatus(afiInput.value);
        }

        // Barcode input validation & Start button status
        if (itemInput && startBtn) {
            const validateInput = () => {
                const val = itemInput.value.trim();
                startBtn.disabled = val.length === 0;
            };

            itemInput.addEventListener('input', validateInput);
            itemInput.addEventListener('change', validateInput);
            itemInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter' || e.keyCode === 13) {
                    e.preventDefault();
                    validateInput();
                    if (!startBtn.disabled) {
                        this.startEncoding();
                    }
                }
            });
            validateInput();
        }

        // Parts selection listeners
        partsPills.forEach((pill) => {
            pill.addEventListener('click', () => {
                this.state.userSelectedPartsManually = true;
                const parts = pill.getAttribute('data-parts');
                this.ui.updatePartsSelection(parts);
            });
        });

        if (partsSelect) {
            partsSelect.addEventListener('change', () => {
                if (partsSelect.value) {
                    this.state.userSelectedPartsManually = true;
                    this.ui.updatePartsSelection(partsSelect.value);
                }
            });
        }

        // Start & Clear button listeners
        if (startBtn) {
            startBtn.addEventListener('click', () => this.startEncoding());
        }

        if (clearBtn) {
            clearBtn.addEventListener('click', () => this.clearActiveTags());
        }

        // Progress modal abort button
        if (stopModalBtn) {
            stopModalBtn.addEventListener('click', () => this.stopEncoding('Kodierung vom Benutzer gestoppt.'));
        }

        // Overwrite modal buttons
        if (confirmOverwriteBtn) {
            confirmOverwriteBtn.addEventListener('click', () => this.confirmOverwrite());
        }

        if (skipOverwriteBtn) {
            skipOverwriteBtn.addEventListener('click', () => this.skipOverwrite());
        }

        if (cancelOverwriteBtn) {
            cancelOverwriteBtn.addEventListener('click', () => this.cancelOverwrite());
        }

        // Auto-detect initial parts count
        this.updateAutoParts();

        // Listen for HTMX swaps, SSE messages, and DOM mutations
        document.body.addEventListener('htmx:afterSwap', (evt) => {
            if (evt.detail && (evt.detail.target.id === 'loan-table-body' || evt.detail.target.closest('#loan-table-body'))) {
                this.handleTransponderUpdate();
            }
        });

        document.body.addEventListener('htmx:sseMessage', () => {
            this.handleTransponderUpdate();
        });

        const tableBody = this.ui.getElement('loan-table-body');
        if (tableBody) {
            const observer = new MutationObserver(() => {
                this.handleTransponderUpdate();
            });
            observer.observe(tableBody, { childList: true, subtree: true });
        }
    }
}
