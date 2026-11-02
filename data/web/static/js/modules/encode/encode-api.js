/**
 * Encode API Client Module
 * Handles HTTP network communication for tag encoding and tag clearing operations.
 */

export class EncodeApiClient {
    constructor(basePath = '') {
        this.basePath = basePath;
    }

    /**
     * Write metadata (ISO 28560-2) to a transponder.
     * @param {Object} payload 
     * @param {string} payload.uid
     * @param {string} payload.itemId
     * @param {string} payload.isil
     * @param {number} payload.parts
     * @param {number} payload.partNo
     * @param {string} payload.afi
     * @param {boolean} [payload.clear]
     * @returns {Promise<{success: boolean, data?: any, error?: string}>}
     */
    async writeTag(payload) {
        try {
            const url = this.resolveUrl('encode/write');
            const reqBody = {
                uid: payload.uid || '',
                itemId: payload.itemId || '',
                isil: payload.isil || '',
                country: payload.country || '',
                parts: payload.parts || 1,
                partNo: payload.partNo || 1,
                afi: payload.afi || '07',
                clear: Boolean(payload.clear)
            };

            console.debug('[EncodeApiClient] Dispatching POST %s:', url, reqBody);
            const startTime = performance.now();

            const resp = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Accept': 'application/json'
                },
                body: JSON.stringify(reqBody)
            });

            const elapsed = Math.round(performance.now() - startTime);

            let result = {};
            try {
                result = await resp.json();
            } catch (jsonErr) {
                console.error('[EncodeApiClient] Failed to parse JSON response (%d ms):', elapsed, jsonErr);
                return {
                    success: false,
                    error: `Ungültige Serverantwort (HTTP ${resp.status}): ${jsonErr.message}`
                };
            }

            if (resp.ok && result.status === 'success') {
                console.debug('[EncodeApiClient] Write success in %d ms:', elapsed, result);
                return {
                    success: true,
                    data: result
                };
            }

            const errorMsg = result.error || result.message || `HTTP ${resp.status}: Fehler beim Schreiben`;
            console.warn('[EncodeApiClient] Write returned failure in %d ms:', elapsed, errorMsg, result);
            return {
                success: false,
                error: errorMsg,
                data: result
            };
        } catch (netErr) {
            console.error('[EncodeApiClient] Network error during write:', netErr);
            return {
                success: false,
                error: `Netzwerkfehler: ${netErr.message || 'Verbindung zum Server fehlgeschlagen'}`
            };
        }
    }

    /**
     * Clear all metadata and unlock (AFI: C2) a transponder.
     * @param {string} uid 
     * @param {string} [afi='C2']
     * @returns {Promise<{success: boolean, data?: any, error?: string}>}
     */
    async clearTag(uid, afi = 'C2') {
        return this.writeTag({
            uid: uid,
            clear: true,
            itemId: '',
            isil: '',
            country: '',
            parts: 1,
            partNo: 1,
            afi: afi
        });
    }

    /**
     * Resolve relative endpoint URL taking current page channel routing into account.
     * @param {string} endpoint 
     * @returns {string}
     */
    resolveUrl(endpoint) {
        if (this.basePath) {
            return `${this.basePath.replace(/\/+$/, '')}/${endpoint.replace(/^\/+/, '')}`;
        }
        return endpoint;
    }
}
