/* RFID Desk Frontend Interactions (HTMX 4) */

document.addEventListener('DOMContentLoaded', () => {
    let beepEnabled = false;
    let audioCtx = null;

    // Toggle audio beep
    const soundToggle = document.getElementById('sound-toggle');
    if (soundToggle) {
        soundToggle.addEventListener('click', () => {
            beepEnabled = !beepEnabled;
            soundToggle.classList.toggle('active', beepEnabled);
            const iconEnable = soundToggle.querySelector('.sound-icon-enable');
            const iconDisable = soundToggle.querySelector('.sound-icon-disable');
            if (iconEnable && iconDisable) {
                if (beepEnabled) {
                    // Sound is ON -> display symbol to turn sound OFF
                    iconEnable.classList.add('hidden');
                    iconDisable.classList.remove('hidden');
                    soundToggle.setAttribute('title', 'Signalton ausschalten');
                } else {
                    // Sound is OFF -> display symbol to turn sound ON
                    iconEnable.classList.remove('hidden');
                    iconDisable.classList.add('hidden');
                    soundToggle.setAttribute('title', 'Signalton einschalten');
                }
            }
        });
    }

    function playBeep() {
        if (!beepEnabled) return;
        try {
            if (!audioCtx) {
                audioCtx = new (window.AudioContext || window.webkitAudioContext)();
            }
            if (audioCtx.state === 'suspended') {
                audioCtx.resume();
            }
            const osc = audioCtx.createOscillator();
            const gain = audioCtx.createGain();
            osc.type = 'sine';
            osc.frequency.setValueAtTime(880, audioCtx.currentTime); // 880 Hz
            gain.gain.setValueAtTime(0.08, audioCtx.currentTime);
            gain.gain.exponentialRampToValueAtTime(0.0001, audioCtx.currentTime + 0.12);
            osc.connect(gain);
            gain.connect(audioCtx.destination);
            osc.start();
            osc.stop(audioCtx.currentTime + 0.12);
        } catch (e) {
            console.debug('Audio play error:', e);
        }
    }

    // SSE connection state handlers (HTMX 4 colon syntax & legacy compatibility)
    function onSSEOpen() {
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

    function onSSEError() {
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

    document.addEventListener('htmx:sse:open', onSSEOpen);
    document.addEventListener('htmx:sseOpen', onSSEOpen);

    document.addEventListener('htmx:sse:error', onSSEError);
    document.addEventListener('htmx:sseError', onSSEError);

    const MAX_TAG_EVENTS = 100;

    // Prune rows in transponder table to maintain at most MAX_TAG_EVENTS (100)
    function pruneTags() {
        const rows = document.querySelectorAll('#tags-table-body tr.tag-row');
        if (rows.length > MAX_TAG_EVENTS) {
            for (let i = MAX_TAG_EVENTS; i < rows.length; i++) {
                rows[i].remove();
            }
        }
    }

    // Handle tag additions & counter updates via HTMX 4 swap / SSE events
    function onTagReceived() {
        playBeep();
        removeEmptyPlaceholder();
        pruneTags();
        updateCounters();
        applyFilter();
    }

    function onLoanTagsReceived() {
        playBeep();
        updateLoanCounters();
    }

    document.body.addEventListener('htmx:sse:message', (evt) => {
        const eventName = evt.detail?.eventName;
        if (eventName === 'tag_read') {
            onTagReceived();
        } else if (eventName === 'tags_cleared') {
            showClearedNotification();
            updateCounters();
            updateLoanCounters();
        } else if (eventName === 'loan_tags' || eventName === 'loan_updated') {
            onLoanTagsReceived();
        }
    });

    document.body.addEventListener('htmx:sseMessage', (evt) => {
        const eventName = evt.detail?.eventName;
        if (eventName === 'tag_read') {
            onTagReceived();
        } else if (eventName === 'tags_cleared') {
            showClearedNotification();
            updateCounters();
            updateLoanCounters();
        } else if (eventName === 'loan_tags' || eventName === 'loan_updated') {
            onLoanTagsReceived();
        }
    });

    document.body.addEventListener('htmx:after:swap', () => {
        pruneTags();
        updateCounters();
        updateLoanCounters();
        applyFilter();
    });

    document.body.addEventListener('htmx:afterSwap', () => {
        pruneTags();
        updateCounters();
        updateLoanCounters();
        applyFilter();
    });

    function removeEmptyPlaceholder() {
        const placeholder = document.getElementById('no-tags-row');
        if (placeholder) {
            placeholder.remove();
        }
    }

    function updateCounters() {
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

    function updateLoanCounters() {
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

    function showClearedNotification() {
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

    // Copy all Item-IDs button for loan & return page
    const copyAllBtn = document.getElementById('btn-copy-all-itemids');
    if (copyAllBtn) {
        copyAllBtn.addEventListener('click', () => {
            const itemElements = document.querySelectorAll('#loan-table-body .loan-item-id, #loan-table-body tr.tag-row[data-itemid]');
            const itemIds = [];
            itemElements.forEach((el) => {
                let id = el.getAttribute('data-itemid') || el.textContent;
                id = id ? id.trim() : '';
                if (id && id !== 'Keine ItemID' && !itemIds.includes(id)) {
                    itemIds.push(id);
                }
            });

            const copyLabel = document.getElementById('copy-all-label');
            const originalLabel = copyLabel ? copyLabel.textContent : 'Item-IDs kopieren';

            if (itemIds.length === 0) {
                if (copyLabel) copyLabel.textContent = 'Keine IDs vorhanden';
                setTimeout(() => {
                    if (copyLabel) copyLabel.textContent = originalLabel;
                }, 2000);
                return;
            }

            const clipboardText = itemIds.join('\n');
            if (navigator.clipboard) {
                navigator.clipboard.writeText(clipboardText).then(() => {
                    if (copyLabel) copyLabel.textContent = `Kopiert (${itemIds.length})`;
                    copyAllBtn.classList.add('copied');
                    setTimeout(() => {
                        if (copyLabel) copyLabel.textContent = originalLabel;
                        copyAllBtn.classList.remove('copied');
                    }, 2000);
                }).catch((err) => {
                    console.error('Clipboard copy failed:', err);
                });
            }
        });
    }

    // Filter functionality
    const filterInput = document.getElementById('tag-filter-input');
    if (filterInput) {
        filterInput.addEventListener('input', applyFilter);
    }

    function applyFilter() {
        const query = (filterInput ? filterInput.value : '').toLowerCase().trim();
        const rows = document.querySelectorAll('#tags-table-body tr.tag-row');
        rows.forEach((row) => {
            const text = row.textContent.toLowerCase();
            if (!query || text.includes(query)) {
                row.style.display = '';
            } else {
                row.style.display = 'none';
            }
        });
    }

    // Copy UID / ItemID to clipboard helper
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('.copy-uid-btn, .copy-itemid-btn');
        if (btn) {
            const textToCopy = btn.getAttribute('data-uid') || btn.getAttribute('data-itemid') || btn.getAttribute('data-copy');
            if (textToCopy && navigator.clipboard) {
                navigator.clipboard.writeText(textToCopy).then(() => {
                    const originalHtml = btn.innerHTML;
                    btn.innerHTML = '<span class="text-emerald-400 text-xs">Kopiert!</span>';
                    setTimeout(() => {
                        btn.innerHTML = originalHtml;
                    }, 1500);
                });
            }
        }
    });

    // Intelligent repositioning of ISIL flyout on mouseover (viewport boundary detection)
    document.addEventListener('mouseover', (e) => {
        const wrapper = e.target.closest('.app-isil-info-wrapper, .app-isil-container');
        if (!wrapper) return;
        const flyout = wrapper.querySelector('.app-isil-flyout');
        if (!flyout) return;

        const rect = wrapper.getBoundingClientRect();
        const flyoutHeight = flyout.offsetHeight || 260;
        const spaceBelow = window.innerHeight - rect.bottom;

        if (spaceBelow < flyoutHeight + 20 && rect.top > flyoutHeight + 20) {
            flyout.classList.add('flyout-top');
        } else {
            flyout.classList.remove('flyout-top');
        }

        if (rect.left + 380 > window.innerWidth) {
            flyout.classList.add('flyout-right');
        } else {
            flyout.classList.remove('flyout-right');
        }
    });

    // Auto-refresh relative time labels every 5 seconds
    setInterval(() => {
        const timeElements = document.querySelectorAll('.time-ago');
        const now = Math.floor(Date.now() / 1000);
        timeElements.forEach((el) => {
            const ts = parseInt(el.getAttribute('data-timestamp'), 10);
            if (ts > 0) {
                const diff = now - Math.floor(ts / 1000);
                if (diff < 5) {
                    el.textContent = 'gerade eben';
                } else if (diff < 60) {
                    el.textContent = 'vor ' + diff + 's';
                } else if (diff < 3600) {
                    el.textContent = 'vor ' + Math.floor(diff / 60) + 'm';
                } else {
                    el.textContent = 'vor ' + Math.floor(diff / 3600) + 'h';
                }
            }
        });
    }, 5000);

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

    // ==========================================================================
    // AFI Write Operation Visual Feedback & Timeout Handling
    // ==========================================================================
    const AFI_TIMEOUT_MS = 10000; // 10 seconds timeout safeguard
    let afiTimeoutTimer = null;
    let afiCountdownInterval = null;
    let isAfiOperationRunning = false;

    function getAfiModalOverlay() {
        return document.getElementById('afi-modal-overlay');
    }

    function showAfiNotification(message, type = 'info', autoHideMs = 6000) {
        const actionNotification = document.getElementById('action-notification');
        const actionNotificationText = document.getElementById('action-notification-text');
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

    function setAfiButtonsDisabled(disabled) {
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

    function showAfiModal(title, desc, status) {
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

        overlay.setAttribute('aria-hidden', 'false');
        overlay.classList.add('active');
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

    function hideAfiModal() {
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
            overlay.classList.remove('active');
            overlay.setAttribute('aria-hidden', 'true');
        }
        setAfiButtonsDisabled(false);
    }

    function handleAfiRequestStart(elt, url) {
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

    function handleAfiRequestEnd(elt, url, isError) {
        let path = url || (elt && typeof elt.getAttribute === 'function' ? (elt.getAttribute('hx-post') || elt.getAttribute('data-hx-post') || elt.getAttribute('action') || '') : '');
        if (!path.includes('loan/afi') && !isAfiOperationRunning) return;

        hideAfiModal();
        if (isError) {
            showAfiNotification('Fehler bei der Kommunikation mit dem RFID-Server.', 'error', 8000);
        }
    }

    function extractAfiRequestInfo(evt) {
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

    pruneTags();
    updateCounters();
});
