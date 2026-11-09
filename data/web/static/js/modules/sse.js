/**
 * SSE & Stream Lifecycle Module for RFID Desk
 * Handles SSE connection state indicators, deduplication, row pruning, table counter updates,
 * stream freeze/pause toggle, and loan table enhancements.
 */

import { playBeep, playSuccessSound, playWarningSound, playErrorSound } from './audio.js';
import { applyFilter, updateLoanTableEnhancements } from './table-actions.js';
import { t } from './i18n.js';

const MAX_TAG_EVENTS = 100;
let lastLoanTagCount = 0;
let lastHandledEventId = null;
let lastHandledTimestamp = 0;
let isStreamPaused = false;

export function isStreamFrozen() {
    return isStreamPaused;
}

export function onSSEOpen() {
    const statusContainer = document.getElementById('sse-status-container');
    const statusDot = document.getElementById('sse-status-dot');
    const statusText = document.getElementById('sse-status-text');
    if (statusContainer) {
        statusContainer.classList.remove('badge-warning');
        statusContainer.classList.add('badge-success');
    }
    if (statusDot) {
        statusDot.className = 'w-2 h-2 rounded-full bg-success pulse-indicator';
    }
    if (statusText) {
        statusText.textContent = t('sse.connected');
    }
}

export function onSSEError() {
    const statusContainer = document.getElementById('sse-status-container');
    const statusDot = document.getElementById('sse-status-dot');
    const statusText = document.getElementById('sse-status-text');
    if (statusContainer) {
        statusContainer.classList.remove('badge-success');
        statusContainer.classList.add('badge-warning');
    }
    if (statusDot) {
        statusDot.className = 'w-2 h-2 rounded-full bg-warning';
    }
    if (statusText) {
        statusText.textContent = t('sse.reconnecting');
    }
}

export function pruneTags() {
    const rows = document.querySelectorAll('#tags-table-body tr.tag-row');
    if (rows.length > MAX_TAG_EVENTS) {
        for (let i = MAX_TAG_EVENTS; i < rows.length; i++) {
            rows[i].remove();
        }
    }
}

export function removeEmptyPlaceholder() {
    const placeholder = document.getElementById('no-tags-row');
    if (placeholder) {
        placeholder.remove();
    }
}

export function updateCounters() {
    const rows = document.querySelectorAll('#tags-table-body tr.tag-row');
    const countBadge = document.getElementById('tag-count-badge');
    const statsTotal = document.getElementById('stats-total-tags');
    if (countBadge) {
        countBadge.textContent = rows.length;
    }
    if (statsTotal) {
        statsTotal.textContent = rows.length;
    }
}

export function updateLoanCounters() {
    const loanRows = document.querySelectorAll('#loan-table-body tr.tag-row');
    const loanCountBadge = document.getElementById('loan-tag-count-badge');
    const statsActive = document.getElementById('stats-active-tags');
    if (loanCountBadge) {
        loanCountBadge.textContent = loanRows.length;
    }
    if (statsActive) {
        statsActive.textContent = loanRows.length;
    }
}

export function showClearedNotification() {
    const badge = document.getElementById('field-status-badge');
    if (badge) {
        badge.textContent = t('sse.cleared');
        badge.className = 'badge badge-sm badge-warning badge-outline transition-all';
        setTimeout(() => {
            badge.textContent = t('sse.ready');
            badge.className = 'badge badge-sm badge-outline transition-all';
        }, 3000);
    }
}

export function onTagReceived() {
    if (isStreamPaused) return;
    playBeep();
    removeEmptyPlaceholder();
    pruneTags();
    updateCounters();
    applyFilter();
}

export function onLoanTagsReceived() {
    const loanRows = document.querySelectorAll('#loan-table-body tr.tag-row');
    const count = loanRows.length;
    if (count > 0 && count !== lastLoanTagCount) {
        playBeep();
    }
    lastLoanTagCount = count;
    updateLoanCounters();
    updateLoanTableEnhancements();
}

export function onBarcodeReceived(rawBarcode) {
    if (!rawBarcode) return;
    const barcode = String(rawBarcode).trim();
    if (!barcode) return;

    playBeep();

    const encodeItemInput = document.getElementById('encode-item-id');
    if (encodeItemInput) {
        encodeItemInput.value = barcode;
        encodeItemInput.dispatchEvent(new Event('input', { bubbles: true }));
        encodeItemInput.dispatchEvent(new Event('change', { bubbles: true }));
        encodeItemInput.focus();
        if (typeof encodeItemInput.select === 'function') {
            encodeItemInput.select();
        }

        const startBtn = document.getElementById('btn-encode-start');
        if (startBtn && !startBtn.disabled) {
            startBtn.click();
        }
    }
}

export function flashWSActivity() {
    const bullet = document.getElementById('ws-activity-bullet');
    if (!bullet) return;

    bullet.classList.remove('active', 'flash');
    // Force DOM reflow to restart CSS transition/animation
    void bullet.offsetWidth;
    bullet.classList.add('active', 'flash');

    if (bullet._wsActivityTimeout) {
        clearTimeout(bullet._wsActivityTimeout);
    }
    bullet._wsActivityTimeout = setTimeout(() => {
        bullet.classList.remove('active', 'flash');
    }, 600);
}

export function handleSSEMessage(evt) {
    flashWSActivity();

    const eventName = evt.detail?.eventName;
    if (!eventName) return;

    const now = Date.now();
    const eventKey = `${eventName}:${evt.detail?.data?.length || 0}`;
    if (eventKey === lastHandledEventId && (now - lastHandledTimestamp) < 30) {
        return;
    }
    lastHandledEventId = eventKey;
    lastHandledTimestamp = now;

    if (eventName === 'tag_read') {
        onTagReceived();
    } else if (eventName === 'tags_cleared') {
        showClearedNotification();
        updateCounters();
        updateLoanCounters();
        updateLoanTableEnhancements();
        lastLoanTagCount = 0;
    } else if (eventName === 'loan_tags' || eventName === 'loan_updated') {
        onLoanTagsReceived();
    } else if (eventName === 'barcode_read') {
        onBarcodeReceived(evt.detail?.data);
    }
}

export function initStreamPause() {
    const pauseBtn = document.getElementById('btn-stream-pause');
    const pauseLabel = document.getElementById('stream-pause-label');
    const pauseIcon = document.getElementById('stream-pause-icon');
    if (!pauseBtn) return;

    pauseBtn.addEventListener('click', () => {
        isStreamPaused = !isStreamPaused;
        if (isStreamPaused) {
            if (pauseLabel) pauseLabel.textContent = t('sse.paused');
            if (pauseIcon) {
                pauseIcon.className = 'w-2 h-2 rounded-full bg-warning';
            }
            pauseBtn.classList.add('btn-warning', 'btn-outline');
            pauseBtn.classList.remove('btn-ghost');
            pauseBtn.setAttribute('title', t('sse.resume'));
        } else {
            if (pauseLabel) pauseLabel.textContent = t('sse.live');
            if (pauseIcon) {
                pauseIcon.className = 'w-2 h-2 rounded-full bg-success pulse-indicator';
            }
            pauseBtn.classList.remove('btn-warning', 'btn-outline');
            pauseBtn.classList.add('btn-ghost');
            pauseBtn.setAttribute('title', t('sse.pause'));
        }
    });
}

export function initSSE() {
    document.addEventListener('htmx:sseOpen', onSSEOpen);
    document.addEventListener('htmx:sseError', onSSEError);

    document.body.addEventListener('htmx:sseMessage', handleSSEMessage);

    // Stream Pause interceptor for live tag reads in desk view
    document.body.addEventListener('htmx:beforeSwap', (evt) => {
        if (isStreamPaused && evt.detail?.target?.id === 'tags-table-body') {
            evt.detail.shouldSwap = false;
        }
    });

    const afterSwapHandler = () => {
        pruneTags();
        updateCounters();
        updateLoanCounters();
        updateLoanTableEnhancements();
        applyFilter();
    };

    document.body.addEventListener('htmx:after:swap', afterSwapHandler);
    document.body.addEventListener('htmx:afterSwap', afterSwapHandler);

    // MutationObserver to auto-update counters, filter and placeholder whenever table changes
    const tableBody = document.getElementById('tags-table-body');
    if (tableBody) {
        let isPruning = false;
        const observer = new MutationObserver(() => {
            if (isPruning) return;
            if (tableBody.querySelector('tr.tag-row')) {
                removeEmptyPlaceholder();
            }
            isPruning = true;
            pruneTags();
            isPruning = false;
            updateCounters();
            applyFilter();
        });
        observer.observe(tableBody, { childList: true });
    }

    const loanTableBody = document.getElementById('loan-table-body');
    if (loanTableBody) {
        const loanObserver = new MutationObserver(() => {
            updateLoanCounters();
            updateLoanTableEnhancements();
        });
        loanObserver.observe(loanTableBody, { childList: true });
    }

    initStreamPause();
    pruneTags();
    updateCounters();
    updateLoanCounters();
    updateLoanTableEnhancements();
}
