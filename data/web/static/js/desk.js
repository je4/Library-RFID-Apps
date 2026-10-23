/* RFID Desk Frontend Interactions (HTMX 4) */

document.addEventListener('DOMContentLoaded', () => {
    let beepEnabled = false;
    let audioCtx = null;

    // Toggle audio beep
    const soundToggle = document.getElementById('sound-toggle');
    if (soundToggle) {
        soundToggle.addEventListener('click', () => {
            beepEnabled = !beepEnabled;
            soundToggle.classList.toggle('text-blue-400', beepEnabled);
            soundToggle.classList.toggle('text-gray-400', !beepEnabled);
            soundToggle.classList.toggle('border-blue-500', beepEnabled);
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
            statusContainer.className = 'flex items-center space-x-2 px-3 py-1.5 bg-emerald-950/60 border border-emerald-800/80 rounded-lg text-xs';
        }
        if (statusDot) {
            statusDot.className = 'w-2 h-2 rounded-full bg-emerald-400 pulse-indicator';
        }
        if (statusText) {
            statusText.textContent = 'SSE Verbunden';
            statusText.className = 'text-xs font-medium text-emerald-400';
        }
    }

    function onSSEError() {
        const statusContainer = document.getElementById('sse-status-container');
        const statusDot = document.getElementById('sse-status-dot');
        const statusText = document.getElementById('sse-status-text');
        if (statusContainer) {
            statusContainer.className = 'flex items-center space-x-2 px-3 py-1.5 bg-amber-950/60 border border-amber-800/80 rounded-lg text-xs';
        }
        if (statusDot) {
            statusDot.className = 'w-2 h-2 rounded-full bg-amber-400';
        }
        if (statusText) {
            statusText.textContent = 'Verbindung wird wiederhergestellt...';
            statusText.className = 'text-xs font-medium text-amber-400';
        }
    }

    document.addEventListener('htmx:sse:open', onSSEOpen);
    document.addEventListener('htmx:sseOpen', onSSEOpen);

    document.addEventListener('htmx:sse:error', onSSEError);
    document.addEventListener('htmx:sseError', onSSEError);

    // Handle tag additions & counter updates via HTMX 4 swap / SSE events
    function onTagReceived() {
        playBeep();
        removeEmptyPlaceholder();
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
            badge.className = 'text-xs font-normal px-2 py-0.5 rounded bg-gray-800 text-gray-300 border border-gray-700 transition-all';
            setTimeout(() => {
                badge.textContent = 'Bereit';
                badge.className = 'text-xs font-normal px-2 py-0.5 rounded bg-blue-950 text-blue-300 border border-blue-800 transition-all';
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

    // Copy UID to clipboard helper
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('.copy-uid-btn');
        if (btn) {
            const uid = btn.getAttribute('data-uid');
            if (uid && navigator.clipboard) {
                navigator.clipboard.writeText(uid).then(() => {
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
        const observer = new MutationObserver(() => {
            if (tableBody.querySelector('tr.tag-row')) {
                removeEmptyPlaceholder();
            }
            updateCounters();
            applyFilter();
        });
        observer.observe(tableBody, { childList: true });
    }

    updateCounters();
});
