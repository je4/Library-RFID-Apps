# RFID Transponder Encoding & Multi-Tag Workflows (ISO 28560-2)

This documentation specifies the architecture, state machines, and sequential workflows for encoding ISO 28560-2 compliant RFID library transponders in the **Library Desk** system. The design maximizes encoding throughput and operational reliability while eliminating race conditions and tag collision errors during multi-part and sequential tag operations.

---

### 1. System Overview & Architecture

The encoding subsystem is structured as a distributed, event-driven pipeline comprising three decoupled layers:
1. **Frontend (Browser / ES6 Modules)**: State machine, UI controller, modal lifecycle manager, and debounced HTMX/SSE event listeners.
2. **Backend Server (Go / Gin Engine)**: `EncodeService`, ISO 28560-2 validation engine, reader routing, and in-memory `Hub` synchronization.
3. **RFID Daemon & Hardware**: Transponder communication over ISO 15693 / ISO 28560-2 protocol blocks and AFI security bytes via WebSocket broker.

```mermaid
flowchart TD
    subgraph Frontend["Frontend Layer (ES6 Modules)"]
        UI["EncodeUI (DOM / Modals / Controls)"]
        Orch["EncodeOrchestrator (Workflow & Debouncing)"]
        SM["EncodeStateMachine (Locks / UID Sets)"]
        API["EncodeApiClient (HTTP Fetch)"]

        Orch --> UI
        Orch --> SM
        Orch --> API
    end

    subgraph Backend["Backend Layer (Go / Gin Engine)"]
        Ctrl["Web & API Controller (/encode/write)"]
        Svc["EncodeService (Validation / Routing)"]
        Hub["desk.Hub (In-Memory State & SSE Broadcast)"]

        Ctrl --> Svc
        Svc --> Hub
    end

    subgraph Hardware["Hardware & Daemon Layer"]
        RfidClient["libraryclient.Client (WebSocket Broker)"]
        Daemon["rfid-headless / Reader Antenna Pad"]

        Svc --> RfidClient
        RfidClient --> Daemon
    end

    API -- "POST /encode/write" --> Ctrl
    Hub -. "SSE Event: loan_tags" .-> Orch
```

---

### 2. Module & Class Architecture

#### 2.1 Frontend Modules (`data/web/static/js/modules/encode/`)

* **`encode-state.js` (`EncodeStateMachine`, `EncodeState`)**:
  * Manages formal lifecycle states: `IDLE`, `PREPARING`, `WRITING`, `WAITING_TAG`, `AWAITING_OVERWRITE`, `COMPLETED`, `ERROR`.
  * Enforces mutual exclusion via `isWriteInFlight` and `isBusy()` locks, preventing concurrent duplicate writes.
  * Tracks programmed tags in `encodedUIDs` with normalized (uppercase) strings to avoid case-mismatch re-encoding.
  * Tracks step progress (`currentPartNo` and `totalParts`).
  * Emits structured diagnostic messages via `console.debug`.

* **`encode-api.js` (`EncodeApiClient`)**:
  * Encapsulates asynchronous HTTP endpoints `/encode/write` and `/encode/clear`.
  * Handles JSON serialization, HTTP status codes, performance metrics, and network error recovery.

* **`encode-ui.js` (`EncodeUI`)**:
  * Manages native `<dialog>` modals (Progress/Waiting modal and Overwrite Confirmation modal).
  * Manages status notification banners with auto-dismiss timers.
  * Extracts currently active transponders from the DOM (`getActiveTransponders()`).
  * Synchronizes AFI security status toggle controls (`07` secured vs `C2` unsecured) and multi-part pill/dropdown selectors.

* **`encode-orchestrator.js` (`EncodeOrchestrator`)**:
  * Coordinates UI interactions, API requests, and state machine transitions.
  * Implements debounced listeners (60ms) for SSE `loan_tags` broadcasts, HTMX swaps, and DOM mutations.
  * Controls audio chime triggers via `playBeep()`.

* **`encode-controller.js`**:
  * Serves as the public ES6 facade module integrated by `desk.js`.

#### 2.2 Backend Service (`pkg/web/`)

* **`pkg/web/encode_service.go` (`EncodeService`)**:
  * `ValidateAndNormalize(*WriteTagRequestPayload)`: Validates Item ID presence, normalizes ISIL country prefixes (`CH`), validates part numbers (`1 <= partNo <= parts`), and normalizes AFI hex values.
  * `ResolveTarget(channel, uid)`: Resolves target reader ID and daemon channel from active Hub transponders or fallback defaults.
  * `ExecuteWriteTag(ctx, channel, payload)`: Executes the write command against the RFID daemon, updates Hub in-memory state, and broadcasts real-time UI updates.

---

### 3. State Machine Lifecycle

```mermaid
stateDiagram-v2
    [*] --> IDLE
    
    IDLE --> PREPARING: User enters Barcode & clicks Start
    PREPARING --> WRITING: Target tag available & verified
    PREPARING --> WAITING_TAG: No unencoded tag on antenna pad
    PREPARING --> AWAITING_OVERWRITE: Tag has existing different Item ID

    WAITING_TAG --> PREPARING: New transponder detected via SSE / DOM
    WAITING_TAG --> IDLE: User cancels / stops session

    AWAITING_OVERWRITE --> WRITING: User confirms "Overwrite"
    AWAITING_OVERWRITE --> PREPARING: User selects "Skip"
    AWAITING_OVERWRITE --> IDLE: User selects "Cancel"

    WRITING --> PREPARING: Write succeeded (more parts remaining)
    WRITING --> COMPLETED: Write succeeded (all parts finished)
    WRITING --> ERROR: Write failed (timeout / antenna error)

    ERROR --> IDLE: Abort session
    COMPLETED --> IDLE: Auto-reset after timeout / notification
```

---

### 4. Sequential Encoding Workflows

#### Workflow 1: Single-Part Item (1/1)

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant UI as EncodeUI
    participant Orch as EncodeOrchestrator
    participant API as EncodeApiClient
    participant Svc as Server (EncodeService)
    participant Reader as RFID Reader Pad

    User->>UI: Input Barcode (e.g. 31415)
    User->>UI: Click "Kodierung starten"
    UI->>Orch: startEncoding()
    Orch->>UI: Show Progress Modal ("Kodierung startet...")
    
    Orch->>UI: getActiveTransponders()
    UI-->>Orch: [Tag E004... (empty)]
    
    Orch->>API: writeTag(UID=E004..., ItemID=31415, Part=1/1)
    API->>Svc: POST /encode/write
    Svc->>Reader: Write ISO 28560-2 Blocks & AFI 07
    Reader-->>Svc: Success OK
    Svc-->>API: { status: "success", uid: "E004..." }
    API-->>Orch: Write Success
    
    Orch->>UI: Play Audio Chime & Show Success Banner
    Orch->>UI: Clear Barcode Input & Dismiss Modal
```

---

#### Workflow 2: Sequential Multi-Part Media (One-by-One Tag Placement)
*Example: 3-part set (Part 1/3, Part 2/3, Part 3/3) where each medium is placed, encoded, removed, and replaced sequentially.*

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Orch as EncodeOrchestrator
    participant SM as EncodeStateMachine
    participant API as EncodeApiClient
    participant Svc as Server (EncodeService)
    participant Hub as desk.Hub (SSE)

    User->>Orch: Start Multi-Part Encoding (Parts = 3, Barcode = 31415)
    
    Note over Orch,SM: Step 1: Encode Part 1 of 3
    Orch->>API: writeTag(UID_A, Part=1/3)
    API->>Svc: POST /encode/write
    Svc-->>API: Success OK
    API-->>Orch: Success
    Orch->>SM: recordSuccess(UID_A) -> currentPartNo = 2
    
    Note over Orch,SM: Step 2: Waiting for User to Swap Tag
    Orch->>Orch: processNextTag() -> UID_A already encoded
    Orch->>SM: setWaitingForTag()
    Note over Orch: Modal: "Warte auf Medium 2 von 3..."
    
    User->>User: Remove Tag UID_A and place Tag UID_B
    Hub-->>Orch: SSE Broadcast (Tag UID_B detected)
    Orch->>Orch: Debounced handleTransponderUpdate()
    
    Note over Orch,SM: Step 3: Encode Part 2 of 3
    Orch->>API: writeTag(UID_B, Part=2/3)
    API->>Svc: POST /encode/write
    Svc-->>API: Success OK
    API-->>Orch: Success
    Orch->>SM: recordSuccess(UID_B) -> currentPartNo = 3
    
    Note over Orch,SM: Step 4: Encode Part 3 of 3
    User->>User: Remove Tag UID_B and place Tag UID_C
    Hub-->>Orch: SSE Broadcast (Tag UID_C detected)
    Orch->>API: writeTag(UID_C, Part=3/3)
    API->>Svc: POST /encode/write
    Svc-->>API: Success OK
    API-->>Orch: Success
    Orch->>SM: recordSuccess(UID_C) -> currentPartNo = 4 > 3
    
    Note over Orch,SM: All parts finished -> COMPLETED
    Orch->>Orch: finishEncoding() -> Success Banner & Form Reset
```

---

#### Workflow 3: Simultaneous Multi-Tag Placement (Batch on Reader Pad)
*Example: 3 tags are resting simultaneously on the reader antenna pad.*

```mermaid
sequenceDiagram
    autonumber
    participant Orch as EncodeOrchestrator
    participant SM as EncodeStateMachine
    participant API as EncodeApiClient
    participant Svc as Server (EncodeService)

    Note over Orch: 3 Tags on Pad: [UID_A, UID_B, UID_C]
    Orch->>SM: start(Parts = 3, Barcode = 31415)
    
    Note over Orch: Write Tag 1 (UID_A)
    Orch->>API: writeTag(UID_A, Part=1/3)
    API->>Svc: POST /encode/write
    Svc-->>API: Success OK
    Orch->>SM: recordSuccess(UID_A) -> currentPartNo = 2
    
    Note over Orch: Immediately pick next unencoded tag (UID_B)
    Orch->>API: writeTag(UID_B, Part=2/3)
    API->>Svc: POST /encode/write
    Svc-->>API: Success OK
    Orch->>SM: recordSuccess(UID_B) -> currentPartNo = 3
    
    Note over Orch: Immediately pick next unencoded tag (UID_C)
    Orch->>API: writeTag(UID_C, Part=3/3)
    API->>Svc: POST /encode/write
    Svc-->>API: Success OK
    Orch->>SM: recordSuccess(UID_C) -> currentPartNo = 4
    
    Note over Orch: All parts finished -> Workflow Complete
    Orch->>Orch: finishEncoding()
```

---

#### Workflow 4: Overwrite Protection Dialog

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Orch as EncodeOrchestrator
    participant UI as EncodeUI
    participant SM as EncodeStateMachine
    participant API as EncodeApiClient

    Orch->>UI: getActiveTransponders()
    UI-->>Orch: [Tag UID_X with existing ItemID = 99999]
    
    Note over Orch: Target Barcode is 31415 (Mismatch detected)
    Orch->>SM: setAwaitingOverwrite(UID_X, 99999)
    Orch->>UI: showOverwriteModal(UID_X, 99999)
    
    alt User clicks "Überschreiben" (Overwrite)
        User->>UI: Click "Überschreiben"
        UI->>Orch: confirmOverwrite()
        Orch->>API: writeTag(UID_X, Barcode=31415)
        API-->>Orch: Success
    else User clicks "Überspringen" (Skip)
        User->>UI: Click "Überspringen"
        UI->>Orch: skipOverwrite()
        Orch->>SM: skipPendingTag() -> Add UID_X to skipped set
        Orch->>Orch: processNextTag()
    else User clicks "Abbrechen" (Cancel)
        User->>UI: Click "Abbrechen"
        UI->>Orch: cancelOverwrite()
        Orch->>Orch: stopEncoding("Kodierung abgebrochen.")
    end
```

---

#### Workflow 5: Blanking / Clearing Transponders ("Tag leeren")

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant UI as EncodeUI
    participant API as EncodeApiClient
    participant Svc as Server (EncodeService)

    User->>UI: Click "Tag leeren"
    UI->>UI: Disable Clear Button & Show Info Banner
    UI->>API: clearTag(UID=E004..., AFI="C2")
    API->>Svc: POST /encode/clear (clear=true, AFI=C2)
    Svc->>Svc: Reset ItemID, ISIL, Country, Parts=1, Set AFI=C2
    Svc-->>API: { status: "success", message: "Transponder erfolgreich geleert." }
    API-->>UI: Clear Success
    UI->>UI: Play Audio Chime & Re-enable Button
```

---

### 5. Race Condition Elimination & Synchronization Guarantees

1. **In-Flight Write Lock**: `EncodeStateMachine.setWritingInFlight(true)` prevents subsequent triggers while an asynchronous network call is executing.
2. **Debounced Event Processing**: HTMX `afterSwap` events, SSE messages, and DOM `MutationObserver` triggers are debounced to 60ms to prevent rapid-fire redundant executions.
3. **UID Normalization**: All transponder UIDs are normalized (`strings.ToUpper(strings.TrimSpace(uid))`) on both frontend and backend to eliminate case-mismatch bugs.
4. **State Isolation**: When the overwrite modal is open (`AWAITING_OVERWRITE`), external reader scan events cannot dismiss or bypass the user confirmation prompt.

---

### 6. API Reference

#### `POST /:channel/encode/write` and `POST /api/v1/encode/write`
Programs ISO 28560-2 library metadata and AFI security status to a transponder.

**Request Payload**:
```json
{
  "uid": "E0040150E2A12345",
  "itemId": "31415926535",
  "isil": "CH-001596-0",
  "country": "CH",
  "parts": 3,
  "partNo": 1,
  "usageType": 1,
  "afi": "07",
  "clear": false
}
```

**Response Payload**:
```json
{
  "status": "success",
  "message": "Transponder E0040150E2A12345 erfolgreich beschrieben (Teil 1 von 3).",
  "uid": "E0040150E2A12345",
  "itemId": "31415926535",
  "isil": "001596-0",
  "country": "CH",
  "parts": 3,
  "partNo": 1,
  "usageType": 1,
  "afi": "07"
}
```

#### `POST /:channel/encode/clear` and `POST /api/v1/encode/clear`
Clears metadata and resets the AFI byte to unlocked state (`C2`).

**Request Payload**:
```json
{
  "uid": "E0040150E2A12345",
  "clear": true,
  "afi": "C2"
}
```
