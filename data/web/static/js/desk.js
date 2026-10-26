/* RFID Desk Frontend Interactions (HTMX 4) - ES Module Entry Point */

import { initAudio } from './modules/audio.js';
import { initTableActions } from './modules/table-actions.js';
import { initIsilFlyout } from './modules/isil-flyout.js';
import { initAfiModal } from './modules/afi-modal.js';
import { initSSE } from './modules/sse.js';
import { initThemeSelector } from './modules/theme-selector.js';
import { initEncodeController } from './modules/encode-controller.js';

export function initDesk() {
    initAudio();
    initTableActions();
    initIsilFlyout();
    initAfiModal();
    initSSE();
    initThemeSelector();
    initEncodeController();
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initDesk);
} else {
    initDesk();
}
