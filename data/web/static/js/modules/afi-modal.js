/**
 * AFI Actions & Modal Feedback Module for RFID Desk
 * Controls visual progress modal, button state, countdown safeguards, and HTMX event hooks.
 */

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
    const afiButtons = document.querySelectorAll('#btn-unsecure, #btn-secure, button[hx-post*="loan/afi"], button[data-hx-post*="loan/afi"]');
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
        hintEl.textContent = `Timeout-Schutz aktiv (${secondsRemaining}s)`;
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
            hintEl.textContent = `Timeout-Schutz aktiv (${secondsRemaining}s)`;
        } else {
            clearInterval(afiCountdownInterval);
        }
    }, 1000);

    // Timeout handler
    afiTimeoutTimer = setTimeout(() => {
        if (isAfiOperationRunning) {
            hideAfiModal();
            showAfiNotification(
                'Zeitüberschreitung: Das Schreiben der AFI hat das Zeitlimit von ' + (AFI_TIMEOUT_MS / 1000) + 's überschritten. Bitte überprüfen Sie die Verbindung zum RFID-Reader und versuchen Sie es erneut.',
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
    if (!path || !path.includes('loan/afi')) return;

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

    let title = 'AFI wird geschrieben...';
    let desc = 'Bitte warten Sie, bis der Sicherheitsstatus auf dem Lesefeld aktualisiert wurde. Bitte keine Medien vom Lesegerät entfernen.';
    let status = 'RFID-Schreibbefehl aktiv...';

    if (isUnsecure) {
        if (targetUid) {
            title = 'Transponder wird entsichert...';
            desc = `AFI-Code C2 (Ausgeliehen / EAS deaktiviert) wird auf Transponder ${targetUid} geschrieben.`;
            status = `Entsichern (${targetUid})...`;
        } else {
            title = 'Medien werden entsichert...';
            desc = 'AFI-Code C2 (Ausgeliehen / EAS deaktiviert) wird auf alle aufliegenden Medien geschrieben.';
            status = 'Entsichern aller Medien...';
        }
    } else if (isSecure) {
        if (targetUid) {
            title = 'Transponder wird gesichert...';
            desc = `AFI-Code 07 (Präsenzbestand / EAS aktiv) wird auf Transponder ${targetUid} geschrieben.`;
            status = `Sichern (${targetUid})...`;
        } else {
            title = 'Medien werden gesichert...';
            desc = 'AFI-Code 07 (Präsenzbestand / EAS aktiv) wird auf alle aufliegenden Medien geschrieben.';
            status = 'Sichern aller Medien...';
        }
    }

    showAfiModal(title, desc, status);
}

export function handleAfiRequestEnd(elt, url, isError) {
    let path = url || (elt && typeof elt.getAttribute === 'function' ? (elt.getAttribute('hx-post') || elt.getAttribute('data-hx-post') || elt.getAttribute('action') || '') : '');
    if (!path.includes('loan/afi') && !isAfiOperationRunning) return;

    hideAfiModal();
    if (isError) {
        showAfiNotification('Fehler bei der Kommunikation mit dem RFID-Server.', 'error', 8000);
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
        const btn = evt.target?.closest?.('#btn-unsecure, #btn-secure, button[hx-post*="loan/afi"], button[data-hx-post*="loan/afi"], a[hx-post*="loan/afi"]');
        if (btn) {
            const url = btn.getAttribute('hx-post') || btn.getAttribute('data-hx-post') || '';
            handleAfiRequestStart(btn, url);
        }
    }, true);

    // 2. HTMX request hooks for AFI writing (support HTMX 4 and HTMX 1/2 event signatures)
    const onHtmxBeforeRequest = (evt) => {
        const info = extractAfiRequestInfo(evt);
        if (info.url && info.url.includes('loan/afi')) {
            handleAfiRequestStart(info.elt, info.url);
        }
    };

    const onHtmxAfterRequest = (evt) => {
        const info = extractAfiRequestInfo(evt);
        if (isAfiOperationRunning || (info.url && info.url.includes('loan/afi'))) {
            handleAfiRequestEnd(info.elt, info.url, info.isError);
        }
    };

    const onHtmxError = (evt) => {
        const info = extractAfiRequestInfo(evt);
        if (isAfiOperationRunning || (info.url && info.url.includes('loan/afi'))) {
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
        showAfiNotification('Zeitüberschreitung der Netzwerkanfrage (HTMX Timeout).', 'error', 10000);
    });
}
