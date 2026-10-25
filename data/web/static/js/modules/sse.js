/**
 * SSE & Stream Lifecycle Module for RFID Desk
 * Handles SSE connection state indicators, deduplication, row pruning, and table counter updates.
 */

import { playBeep } from './audio.js';
import { applyFilter } from './table-actions.js';

const MAX_TAG_EVENTS = 100;
let lastLoanTagCount = 0;
let lastHandledEventId = null;
let lastHandledTimestamp = 0;

export function onSSEOpen() {
    const statusContainer = document.getElementById('sse-status-container');
    const statusDot = document.getElementById('sse-status-dot');
    const statusText = document.getElementById('sse-status-text');
    if (statusContainer) {
        statusContainer.classList.remove('connecting');
        statusContainer.classList.add('connected');
    }
    if (statusDot) {
        statusDot.className = 'w-2 h-2 rounded-full pulse-indicator';
    }
    if (statusText) {
        statusText.textContent = 'SSE Verbunden';
    }
}

export function onSSEError() {
    const statusContainer = document.getElementById('sse-status-container');
    const statusDot = document.getElementById('sse-status-dot');
    const statusText = document.getElementById('sse-status-text');
    if (statusContainer) {
        statusContainer.classList.remove('connected');
        statusContainer.classList.add('connecting');
    }
    if (statusDot) {
        statusDot.className = 'w-2 h-2 rounded-full';
    }
    if (statusText) {
        statusText.textContent = 'Verbindung wird wiederhergestellt...';
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
        badge.textContent = 'Lesefeld frei';
        badge.className = 'text-xs font-normal px-2 py-0.5 rounded app-badge-status-cleared transition-all';
        setTimeout(() => {
            badge.textContent = 'Bereit';
            badge.className = 'text-xs font-normal px-2 py-0.5 rounded app-badge-status transition-all';
        }, 3000);
    }
}

export function onTagReceived() {
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
}

export function handleSSEMessage(evt) {
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
        lastLoanTagCount = 0;
    } else if (eventName === 'loan_tags' || eventName === 'loan_updated') {
        onLoanTagsReceived();
    }
}

export function initSSE() {
    document.addEventListener('htmx:sse:open', onSSEOpen);
    document.addEventListener('htmx:sseOpen', onSSEOpen);

    document.addEventListener('htmx:sse:error', onSSEError);
    document.addEventListener('htmx:sseError', onSSEError);

    document.body.addEventListener('htmx:sse:message', handleSSEMessage);
    document.body.addEventListener('htmx:sseMessage', handleSSEMessage);

    const afterSwapHandler = () => {
        pruneTags();
        updateCounters();
        updateLoanCounters();
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

    pruneTags();
    updateCounters();
}
