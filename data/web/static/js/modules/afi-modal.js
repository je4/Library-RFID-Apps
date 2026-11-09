/**
 * AFI Actions & Modal Feedback Module for RFID Desk
 * Controls visual progress modal, button state, countdown safeguards, and HTMX event hooks.
 */

import { t } from './i18n.js';

const AFI_TIMEOUT_MS = 10000; // 10 seconds timeout safeguard
let afiTimeoutTimer = null;
let afiCountdownInterval = null;
let isAfiOperationRunning = false;

export function getAfiModalOverlay() {
    return document.getElementById('afi-modal-overlay');
}

export function showAfiNotification(message, type = 'info', autoHideMs = 6000) {
    const actionNotification = document.getElementById('action-notification');
    const actionNotificationText = document.getElementById('action-notification-text');
    if (!actionNotification || !actionNotificationText) return;
    actionNotificationText.textContent = message;
    const alertClass = type === 'error' ? 'alert-error' : (type === 'success' ? 'alert-success' : 'alert-info');
    actionNotification.className = `alert ${alertClass} text-xs p-3 shadow-sm rounded-lg flex items-center justify-between ` +
        (type === 'error' ? 'app-notification-error' : (type === 'success' ? 'app-notification-success' : ''));
    actionNotification.classList.remove('hidden');

    if (autoHideMs > 0) {
        setTimeout(() => {
            actionNotification.classList.add('hidden');
        }, autoHideMs);
    }
}

export function setAfiButtonsDisabled(disabled) {
    const afiButtons = document.querySelectorAll('#btn-unsecure, #btn-secure, #btn-clear-tag, button[hx-post*="loan/afi"], button[data-hx-post*="loan/afi"], button[hx-post*="loan/clear"], button[data-hx-post*="loan/clear"], button[hx-post*="encode/clear"]');
    afiButtons.forEach((btn) => {
        btn.disabled = disabled;
        if (disabled) {
            btn.classList.add('is-busy');
        } else {
            btn.classList.remove('is-busy');
        }
    });
}

export function showAfiModal(title, desc, status) {
    const overlay = getAfiModalOverlay();
    if (!overlay) return;

    isAfiOperationRunning = true;
    const titleEl = document.getElementById('afi-modal-title');
    const msgEl = document.getElementById('afi-modal-message');
    const statusEl = document.getElementById('afi-modal-status-text');
    const hintEl = document.getElementById('afi-modal-timeout-hint');

    if (titleEl && title) titleEl.textContent = title;
    if (msgEl && desc) msgEl.textContent = desc;
    if (statusEl && status) statusEl.textContent = status;

    let secondsRemaining = Math.round(AFI_TIMEOUT_MS / 1000);
    if (hintEl) {
        hintEl.textContent = t('afi.timeoutHint', { seconds: secondsRemaining });
    }

    if (typeof overlay.showModal === 'function' && !overlay.open) {
        try { overlay.showModal(); } catch (_) { overlay.setAttribute('open', ''); }
    }
    overlay.setAttribute('aria-hidden', 'false');
    overlay.classList.add('active', 'modal-open');
    setAfiButtonsDisabled(true);

    // Clear existing timers
    if (afiTimeoutTimer) clearTimeout(afiTimeoutTimer);
    if (afiCountdownInterval) clearInterval(afiCountdownInterval);

    // Countdown display
    afiCountdownInterval = setInterval(() => {
        secondsRemaining--;
        if (secondsRemaining > 0 && hintEl) {
            hintEl.textContent = t('afi.timeoutHint', { seconds: secondsRemaining });
        } else {
            clearInterval(afiCountdownInterval);
        }
    }, 1000);

    // Timeout handler
    afiTimeoutTimer = setTimeout(() => {
        if (isAfiOperationRunning) {
            hideAfiModal();
            showAfiNotification(
                t('afi.timeout', { seconds: AFI_TIMEOUT_MS / 1000 }),
                'error',
                10000
            );
        }
    }, AFI_TIMEOUT_MS);
}

export function hideAfiModal() {
    isAfiOperationRunning = false;
    if (afiTimeoutTimer) {
        clearTimeout(afiTimeoutTimer);
        afiTimeoutTimer = null;
    }
    if (afiCountdownInterval) {
        clearInterval(afiCountdownInterval);
        afiCountdownInterval = null;
    }
    const overlay = getAfiModalOverlay();
    if (overlay) {
        if (typeof overlay.close === 'function' && overlay.open) {
            try { overlay.close(); } catch (_) { overlay.removeAttribute('open'); }
        }
        overlay.classList.remove('active', 'modal-open');
        overlay.setAttribute('aria-hidden', 'true');
    }
    setAfiButtonsDisabled(false);
}

export function handleAfiRequestStart(elt, url) {
    let path = url || (elt && typeof elt.getAttribute === 'function' ? (elt.getAttribute('hx-post') || elt.getAttribute('data-hx-post') || elt.getAttribute('action') || '') : '');
    if (!path || (!path.includes('loan/afi') && !path.includes('loan/clear') && !path.includes('encode/clear'))) return;

    let isClear = path.includes('clear');
    let isUnsecure = path.includes('unsecure') || path.includes('AFI=C2') || path.includes('afi=C2') || path.includes('afi=c2');
    let isSecure = path.includes('secure') || path.includes('AFI=07') || path.includes('afi=07');

    let targetUid = '';
    if (path.includes('uid=')) {
        try {
            const queryPart = path.split('?')[1];
            if (queryPart) {
                const urlParams = new URLSearchParams(queryPart);
                targetUid = urlParams.get('uid') || '';
            }
        } catch (e) {}
    }
    if (!targetUid && elt && typeof elt.closest === 'function') {
        const row = elt.closest('tr');
        if (row) {
            targetUid = row.getAttribute('data-uid') || '';
        }
    }

    let title = t('afi.title');
    let desc = t('afi.description');
    let status = t('afi.status');

    if (isClear) {
        if (targetUid) {
            title = t('afi.clearTitle');
            desc = t('afi.clearUid', { uid: targetUid });
            status = t('afi.clearUidStatus', { uid: targetUid });
        } else {
            title = t('afi.clearTitle');
            desc = t('afi.clearAll');
            status = t('afi.clearStatus');
        }
    } else if (isUnsecure) {
        if (targetUid) {
            title = t('afi.unsecureTitle');
            desc = t('afi.unsecureUid', { uid: targetUid });
            status = t('afi.unsecureUidStatus', { uid: targetUid });
        } else {
            title = t('afi.unsecureAllTitle');
            desc = t('afi.unsecureAll');
            status = t('afi.unsecureStatus');
        }
    } else if (isSecure) {
        if (targetUid) {
            title = t('afi.secureTitle');
            desc = t('afi.secureUid', { uid: targetUid });
            status = t('afi.secureUidStatus', { uid: targetUid });
        } else {
            title = t('afi.secureAllTitle');
            desc = t('afi.secureAll');
            status = t('afi.secureStatus');
        }
    }

    showAfiModal(title, desc, status);
}

export function handleAfiRequestEnd(elt, url, isError) {
    let path = url || (elt && typeof elt.getAttribute === 'function' ? (elt.getAttribute('hx-post') || elt.getAttribute('data-hx-post') || elt.getAttribute('action') || '') : '');
    if (!path.includes('loan/afi') && !path.includes('loan/clear') && !path.includes('encode/clear') && !isAfiOperationRunning) return;

    hideAfiModal();
    if (isError) {
        showAfiNotification(t('afi.error'), 'error', 8000);
    }
}

export function extractAfiRequestInfo(evt) {
    if (!evt) return { elt: null, url: '', isError: false };
    const detail = evt.detail || {};
    const ctx = detail.ctx || {};
    const elt = ctx.sourceElement || ctx.element || detail.elt || detail.target || evt.target || null;
    let url = ctx.request?.action || ctx.action || detail.requestConfig?.path || detail.path || '';
    if (!url && elt && typeof elt.getAttribute === 'function') {
        url = elt.getAttribute('hx-post') || elt.getAttribute('data-hx-post') || elt.getAttribute('action') || '';
    }
    const isError = Boolean(
        detail.failed ||
        detail.error ||
        (detail.xhr && detail.xhr.status >= 400) ||
        (ctx.response && ctx.response.status >= 400)
    );
    return { elt, url, isError };
}

export function initAfiModal() {
    // 1. Direct click capture listener on document for instantaneous feedback
    document.addEventListener('click', (evt) => {
        const btn = evt.target?.closest?.('#btn-unsecure, #btn-secure, button[hx-post*="loan/afi"], button[data-hx-post*="loan/afi"], a[hx-post*="loan/afi"], button[hx-post*="loan/clear"], button[hx-post*="encode/clear"]');
        if (btn) {
            const url = btn.getAttribute('hx-post') || btn.getAttribute('data-hx-post') || '';
            handleAfiRequestStart(btn, url);
        }
    }, true);

    // 2. HTMX request hooks for AFI writing (support HTMX 4 and HTMX 1/2 event signatures)
    const onHtmxBeforeRequest = (evt) => {
        const info = extractAfiRequestInfo(evt);
        if (info.url && (info.url.includes('loan/afi') || info.url.includes('loan/clear') || info.url.includes('encode/clear'))) {
            handleAfiRequestStart(info.elt, info.url);
        }
    };

    const onHtmxAfterRequest = (evt) => {
        const info = extractAfiRequestInfo(evt);
        if (isAfiOperationRunning || (info.url && (info.url.includes('loan/afi') || info.url.includes('loan/clear') || info.url.includes('encode/clear')))) {
            handleAfiRequestEnd(info.elt, info.url, info.isError);
        }
    };

    const onHtmxError = (evt) => {
        const info = extractAfiRequestInfo(evt);
        if (isAfiOperationRunning || (info.url && (info.url.includes('loan/afi') || info.url.includes('loan/clear') || info.url.includes('encode/clear')))) {
            handleAfiRequestEnd(info.elt, info.url, true);
        }
    };

    // HTMX Lifecycle Events Registration
    document.body.addEventListener('htmx:before:request', onHtmxBeforeRequest);
    document.body.addEventListener('htmx:beforeRequest', onHtmxBeforeRequest);
    document.body.addEventListener('htmx:config:request', onHtmxBeforeRequest);

    document.body.addEventListener('htmx:after:request', onHtmxAfterRequest);
    document.body.addEventListener('htmx:afterRequest', onHtmxAfterRequest);
    document.body.addEventListener('htmx:after:swap', onHtmxAfterRequest);
    document.body.addEventListener('htmx:afterSwap', onHtmxAfterRequest);

    document.body.addEventListener('htmx:response:error', onHtmxError);
    document.body.addEventListener('htmx:responseError', onHtmxError);
    document.body.addEventListener('htmx:send:error', onHtmxError);
    document.body.addEventListener('htmx:sendError', onHtmxError);
    document.body.addEventListener('htmx:error', onHtmxError);
    document.body.addEventListener('htmx:timeout', (evt) => {
        onHtmxError(evt);
        showAfiNotification(t('afi.networkTimeout'), 'error', 10000);
    });
}
