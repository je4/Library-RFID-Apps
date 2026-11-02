/**
 * Encode State Machine Module
 * Manages the state, progress, encoded transponders set, and concurrency locks
 * for sequential ISO 28560-2 RFID tag encoding.
 */

export const EncodeState = Object.freeze({
    IDLE: 'IDLE',
    PREPARING: 'PREPARING',
    WRITING: 'WRITING',
    WAITING_TAG: 'WAITING_TAG',
    AWAITING_OVERWRITE: 'AWAITING_OVERWRITE',
    COMPLETED: 'COMPLETED',
    ERROR: 'ERROR'
});

export class EncodeStateMachine {
    constructor() {
        this.currentState = EncodeState.IDLE;
        this.isEncodingRunning = false;
        this.isWriteInFlight = false;
        this.userSelectedPartsManually = false;

        this.currentPartNo = 1;
        this.totalParts = 1;
        this.currentBarcode = '';
        this.currentISIL = '';
        this.currentAFI = '07';

        // Normalized set of transponder UIDs that have been written in the active session
        this.encodedUIDs = new Set();

        // Pending tag candidate waiting for user overwrite confirmation
        this.pendingTag = null; // { uid: string, oldItemId: string }

        this.lastError = null;
    }

    /**
     * Start a new encoding session.
     * @param {string} barcode 
     * @param {string} isil 
     * @param {string} afi 
     * @param {number} totalParts 
     */
    start(barcode, isil, afi, totalParts = 1) {
        this.currentState = EncodeState.PREPARING;
        this.isEncodingRunning = true;
        this.isWriteInFlight = false;
        this.currentBarcode = (barcode || '').trim();
        this.currentISIL = (isil || '').trim();
        this.currentAFI = (afi || '07').trim();
        this.totalParts = Math.max(1, parseInt(totalParts, 10) || 1);
        this.currentPartNo = 1;
        this.encodedUIDs.clear();
        this.pendingTag = null;
        this.lastError = null;

        console.debug('[EncodeStateMachine] Session started:', {
            barcode: this.currentBarcode,
            isil: this.currentISIL,
            afi: this.currentAFI,
            totalParts: this.totalParts
        });
    }

    /**
     * Stop and reset the active encoding session.
     */
    stop() {
        console.debug('[EncodeStateMachine] Session stopped (previously in state %s, part %d/%d)',
            this.currentState, this.currentPartNo, this.totalParts);
        this.currentState = EncodeState.IDLE;
        this.isEncodingRunning = false;
        this.isWriteInFlight = false;
        this.pendingTag = null;
    }

    /**
     * Mark an asynchronous write operation as currently in flight.
     */
    setWritingInFlight(inFlight) {
        this.isWriteInFlight = Boolean(inFlight);
        if (this.isWriteInFlight) {
            this.currentState = EncodeState.WRITING;
            console.debug('[EncodeStateMachine] State -> WRITING (In-flight lock engaged)');
        }
    }

    /**
     * Check whether a write operation is currently processing or awaiting modal confirmation.
     * @returns {boolean}
     */
    isBusy() {
        return this.isWriteInFlight || this.currentState === EncodeState.AWAITING_OVERWRITE;
    }

    /**
     * Record a successful write for the given tag UID and advance the part counter.
     * @param {string} uid 
     */
    recordSuccess(uid) {
        const norm = this.normalizeUID(uid);
        if (norm) {
            this.encodedUIDs.add(norm);
        }
        const previousPart = this.currentPartNo;
        this.currentPartNo++;
        this.isWriteInFlight = false;
        this.pendingTag = null;

        if (this.isComplete()) {
            this.currentState = EncodeState.COMPLETED;
            console.debug('[EncodeStateMachine] Success for part %d/%d (UID: %s). All parts completed -> COMPLETED',
                previousPart, this.totalParts, norm);
        } else {
            this.currentState = EncodeState.PREPARING;
            console.debug('[EncodeStateMachine] Success for part %d/%d (UID: %s). Advancing to part %d/%d -> PREPARING',
                previousPart, this.totalParts, norm, this.currentPartNo, this.totalParts);
        }
    }

    /**
     * Set state to awaiting overwrite confirmation for a specific tag.
     * @param {string} uid 
     * @param {string} oldItemId 
     */
    setAwaitingOverwrite(uid, oldItemId) {
        this.currentState = EncodeState.AWAITING_OVERWRITE;
        this.pendingTag = {
            uid: this.normalizeUID(uid),
            rawUID: uid,
            oldItemId: (oldItemId || '').trim()
        };
        console.debug('[EncodeStateMachine] State -> AWAITING_OVERWRITE for tag %s (existing barcode: %s)',
            this.pendingTag.uid, this.pendingTag.oldItemId);
    }

    /**
     * Clear pending overwrite state without advancing part counter.
     */
    clearPendingOverwrite() {
        console.debug('[EncodeStateMachine] Cleared pending overwrite tag');
        this.pendingTag = null;
        if (this.isEncodingRunning && !this.isComplete()) {
            this.currentState = EncodeState.PREPARING;
        }
    }

    /**
     * Skip the pending tag and add it to encoded/skipped set.
     */
    skipPendingTag() {
        if (this.pendingTag && this.pendingTag.uid) {
            console.debug('[EncodeStateMachine] Skipping tag %s without writing', this.pendingTag.uid);
            this.encodedUIDs.add(this.pendingTag.uid);
        }
        this.clearPendingOverwrite();
    }

    /**
     * Check if all parts have been encoded.
     * @returns {boolean}
     */
    isComplete() {
        return this.currentPartNo > this.totalParts;
    }

    /**
     * Set state to waiting for the next physical transponder to be placed.
     */
    setWaitingForTag() {
        this.currentState = EncodeState.WAITING_TAG;
        this.isWriteInFlight = false;
        console.debug('[EncodeStateMachine] State -> WAITING_TAG for part %d/%d',
            this.currentPartNo, this.totalParts);
    }

    /**
     * Check if a tag UID has already been encoded or skipped in the current session.
     * @param {string} uid 
     * @returns {boolean}
     */
    isTagAlreadyEncoded(uid) {
        if (!uid) return false;
        return this.encodedUIDs.has(this.normalizeUID(uid));
    }

    /**
     * Normalize UID for consistent case-insensitive comparison.
     * @param {string} uid 
     * @returns {string}
     */
    normalizeUID(uid) {
        return (uid || '').trim().toUpperCase();
    }
}
