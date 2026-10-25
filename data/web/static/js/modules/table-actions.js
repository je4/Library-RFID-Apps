/**
 * Table Actions Module for RFID Desk
 * Handles filtering, clipboard copy operations, and relative timestamp updates.
 */

export function applyFilter() {
    const filterInput = document.getElementById('tag-filter-input');
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

export function updateTimeAgoLabels() {
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
}

export function initTableActions() {
    // Filter input
    const filterInput = document.getElementById('tag-filter-input');
    if (filterInput) {
        filterInput.addEventListener('input', applyFilter);
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

    // Copy UID / ItemID to clipboard helper (event delegation)
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
    setInterval(updateTimeAgoLabels, 5000);
}
