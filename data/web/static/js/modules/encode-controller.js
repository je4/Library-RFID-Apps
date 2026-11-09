/**
 * Encode Controller Facade Module for RFID Desk
 * Delegates to modular EncodeOrchestrator, EncodeStateMachine, EncodeUI, and EncodeApiClient.
 */

import { EncodeOrchestrator } from './encode/encode-orchestrator.js';
import { EncodeStateMachine, EncodeState } from './encode/encode-state.js';
import { EncodeApiClient } from './encode/encode-api.js';
import { EncodeUI } from './encode/encode-ui.js';
import { t } from './i18n.js';

// Singleton orchestrator instance
let orchestratorInstance = null;

export function getOrchestrator() {
    if (!orchestratorInstance) {
        orchestratorInstance = new EncodeOrchestrator();
    }
    return orchestratorInstance;
}

export function showEncodeNotification(message, type = 'info', autoHideMs = 6000) {
    getOrchestrator().ui.showNotification(message, type, autoHideMs);
}

export function updatePartsSelection(partsCount, isManual = true) {
    const orchestrator = getOrchestrator();
    if (isManual) {
        orchestrator.state.userSelectedPartsManually = true;
    }
    orchestrator.ui.updatePartsSelection(partsCount);
}

export function setAFIStatus(afiValue) {
    getOrchestrator().ui.setAFIStatus(afiValue);
}

export function getActiveTransponders() {
    return getOrchestrator().ui.getActiveTransponders();
}

export function startEncoding() {
    getOrchestrator().startEncoding();
}

export function stopEncoding(userMessage = t('encode.cancelled')) {
    getOrchestrator().stopEncoding(userMessage);
}

export function clearActiveTags() {
    return getOrchestrator().clearActiveTags();
}

export function initEncodeController() {
    const orchestrator = getOrchestrator();
    orchestrator.init();
}

export {
    EncodeOrchestrator,
    EncodeStateMachine,
    EncodeState,
    EncodeApiClient,
    EncodeUI
};
