(() => {
    let api;

    function getClosestSSEContainer(elt) {
        if (!elt) return null;
        if (elt.hasAttribute && (elt.hasAttribute('sse-connect') || elt.hasAttribute('data-sse-connect') || elt.hasAttribute('hx-sse'))) {
            return elt;
        }
        return elt.closest ? elt.closest('[sse-connect], [data-sse-connect], [hx-sse]') : null;
    }

    function getConnectUrl(elt) {
        if (!elt) return null;
        let url = (api && api.attributeValue ? api.attributeValue(elt, 'sse-connect') : null) || elt.getAttribute('sse-connect') || elt.getAttribute('data-sse-connect');
        if (!url) {
            let hxSse = (api && api.attributeValue ? api.attributeValue(elt, 'hx-sse') : null) || elt.getAttribute('hx-sse');
            if (hxSse && hxSse.includes('connect:')) {
                let match = hxSse.match(/connect:([^\s,;]+)/);
                if (match) url = match[1];
            }
        }
        return url ? url.trim() : null;
    }

    function initSSEConnection(container) {
        let connectUrl = getConnectUrl(container);
        if (!connectUrl) return;

        let state = container._htmx_sse || (container._htmx_sse = {});
        if (state.sseSource) return; // already initialized

        try {
            console.log('[htmx 4 sse] Connecting to SSE endpoint:', connectUrl);
            let source = new EventSource(connectUrl);
            state.sseSource = source;
            state.sseListeners = [];

            source.addEventListener('open', (evt) => {
                console.log('[htmx 4 sse] SSE connected to:', connectUrl);
                let detail = { source, event: evt };
                if (api && api.triggerHtmxEvent) {
                    api.triggerHtmxEvent(container, 'htmx:sse:open', detail);
                    api.triggerHtmxEvent(container, 'htmx:sseOpen', detail);
                }
                container.dispatchEvent(new CustomEvent('htmx:sse:open', { bubbles: true, detail }));
                container.dispatchEvent(new CustomEvent('htmx:sseOpen', { bubbles: true, detail }));
            });

            source.addEventListener('error', (evt) => {
                console.warn('[htmx 4 sse] SSE connection error/reconnecting for:', connectUrl);
                let detail = { source, event: evt };
                if (api && api.triggerHtmxEvent) {
                    api.triggerHtmxEvent(container, 'htmx:sse:error', detail);
                    api.triggerHtmxEvent(container, 'htmx:sseError', detail);
                }
                container.dispatchEvent(new CustomEvent('htmx:sse:error', { bubbles: true, detail }));
                container.dispatchEvent(new CustomEvent('htmx:sseError', { bubbles: true, detail }));
            });

            bindSSEElements(container, source);
        } catch (err) {
            console.error('[htmx 4 sse] Error initializing EventSource for:', connectUrl, err);
        }
    }

    function bindSSEElements(container, source) {
        let targets = [];
        if (container.querySelectorAll) {
            targets = Array.from(container.querySelectorAll('[sse-swap], [data-sse-swap]'));
        }
        if (container.hasAttribute && (container.hasAttribute('sse-swap') || container.hasAttribute('data-sse-swap'))) {
            targets.push(container);
        }

        targets.forEach((targetElt) => {
            let swapAttr = (api && api.attributeValue ? api.attributeValue(targetElt, 'sse-swap') : null) || targetElt.getAttribute('sse-swap') || targetElt.getAttribute('data-sse-swap');
            if (!swapAttr) return;

            let eventNames = swapAttr.split(',').map(s => s.trim()).filter(Boolean);
            eventNames.forEach((eventName) => {
                let listenerKey = '_sse_bound_' + eventName;
                if (targetElt[listenerKey]) return; // prevent duplicate listener
                targetElt[listenerKey] = true;

                let onMessage = async (evt) => {
                    let rawHtml = evt.data;
                    if (!rawHtml) return;

                    let swapStyle = ((api && api.attributeValue ? api.attributeValue(targetElt, 'hx-swap') : null) || targetElt.getAttribute('hx-swap') || 'afterbegin').trim();

                    let ctx = {
                        sourceElement: targetElt,
                        target: targetElt,
                        swap: swapStyle,
                        text: rawHtml,
                        response: { status: 200, headers: {} },
                    };

                    let swapped = false;
                    try {
                        if (api && api.swap) {
                            await api.swap(ctx);
                            swapped = true;
                        } else if (window.htmx && window.htmx.swap) {
                            await window.htmx.swap(ctx);
                            swapped = true;
                        }
                    } catch (swapErr) {
                        console.warn('[htmx 4 sse] htmx.swap failed, using fallback swap:', swapErr);
                    }

                    if (!swapped) {
                        performFallbackSwap(targetElt, rawHtml, swapStyle);
                    }

                    let detail = { eventName, data: rawHtml, source: evt.target };
                    if (api && api.triggerHtmxEvent) {
                        api.triggerHtmxEvent(targetElt, 'htmx:sse:message', detail);
                        api.triggerHtmxEvent(targetElt, 'htmx:sseMessage', detail);
                    }
                    targetElt.dispatchEvent(new CustomEvent('htmx:sse:message', { bubbles: true, detail }));
                    targetElt.dispatchEvent(new CustomEvent('htmx:sseMessage', { bubbles: true, detail }));
                };

                source.addEventListener(eventName, onMessage);
                if (container._htmx_sse && container._htmx_sse.sseListeners) {
                    container._htmx_sse.sseListeners.push({ eventName, listener: onMessage, targetElt });
                }
            });
        });
    }

    function performFallbackSwap(target, html, swapStyle) {
        let style = (swapStyle || 'afterbegin').toLowerCase().trim();
        if (style.startsWith('afterbegin')) {
            target.insertAdjacentHTML('afterbegin', html);
        } else if (style.startsWith('beforeend')) {
            target.insertAdjacentHTML('beforeend', html);
        } else if (style.startsWith('outerhtml')) {
            target.outerHTML = html;
        } else {
            target.innerHTML = html;
        }

        if (window.htmx && window.htmx.process) {
            window.htmx.process(target);
        }
    }

    function processRoot(root) {
        if (!root) return;
        let containers = [];
        if (root.querySelectorAll) {
            containers = Array.from(root.querySelectorAll('[sse-connect], [data-sse-connect], [hx-sse]'));
        }
        if (root.hasAttribute && (root.hasAttribute('sse-connect') || root.hasAttribute('data-sse-connect') || root.hasAttribute('hx-sse'))) {
            containers.push(root);
        }
        containers.forEach(initSSEConnection);
    }

    // Register HTMX Extension
    if (typeof htmx !== 'undefined' && htmx.registerExtension) {
        htmx.registerExtension('sse', {
            init: (internalAPI) => {
                api = internalAPI;
            },

            htmx_after_process: (elt) => {
                processRoot(elt);
            },

            htmx_after_init: (elt) => {
                let connectUrl = getConnectUrl(elt);
                if (connectUrl) {
                    initSSEConnection(elt);
                } else {
                    let parent = getClosestSSEContainer(elt);
                    if (parent && parent._htmx_sse && parent._htmx_sse.sseSource) {
                        bindSSEElements(elt, parent._htmx_sse.sseSource);
                    }
                }
            },

            htmx_before_cleanup: (elt) => {
                if (elt._htmx_sse && elt._htmx_sse.sseSource) {
                    if (elt._htmx_sse.sseListeners) {
                        elt._htmx_sse.sseListeners.forEach(({ eventName, listener }) => {
                            elt._htmx_sse.sseSource.removeEventListener(eventName, listener);
                        });
                    }
                    elt._htmx_sse.sseSource.close();
                    delete elt._htmx_sse.sseSource;
                    delete elt._htmx_sse.sseListeners;
                }
            }
        });
    }

    // Auto-initialize on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            processRoot(document.body);
        });
    } else {
        processRoot(document.body);
    }
})();
