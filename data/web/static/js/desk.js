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

    document.body.addEventListener('htmx:sse:message', (evt) => {
        const eventName = evt.detail?.eventName;
        if (eventName === 'tag_read') {
            onTagReceived();
        } else if (eventName === 'tags_cleared') {
            showClearedNotification();
        }
    });

    document.body.addEventListener('htmx:sseMessage', (evt) => {
        const eventName = evt.detail?.eventName;
        if (eventName === 'tag_read') {
            onTagReceived();
        } else if (eventName === 'tags_cleared') {
            showClearedNotification();
        }
    });

    document.body.addEventListener('htmx:after:swap', () => {
        pruneTags();
        updateCounters();
        applyFilter();
    });

    document.body.addEventListener('htmx:afterSwap', () => {
        pruneTags();
        updateCounters();
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

    pruneTags();
    updateCounters();
});
