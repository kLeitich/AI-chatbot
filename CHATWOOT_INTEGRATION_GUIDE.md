# Booka Omnichannel Chatwoot & WhatsApp Integration Guide

## 1. Architecture Summary

Booka acts as the **System of Record** (bookings, availability, doctor schedules, branches, patient records, and multi-tenant isolation), while **Chatwoot** acts as the **Omnichannel Messaging & Agent Handover Layer** for WhatsApp Business connectivity.

### Key Architectural Guarantees
- **Multi-Tenant Inbox Routing**: Incoming messages on `/webhooks/chatwoot` identify the exact healthcare provider (`Tenant`) exclusively via verified `chatwoot_inbox_id` (and `chatwoot_account_id`) mapping in the database—never relying on heuristics or phone number guesses.
- **Persistent AI Sessions (`chat_sessions` table)**: Every conversation persists workflow state, drafted booking parameters, patient identity, and human handover state across server restarts and retries.
- **Controlled Tool Execution**: The AI assistant (Groq LLM function-calling with deterministic state-machine fallback) interacts with Booka solely through strict tenant-scoped Go tools (`backend/chatwoot_tools.go`). It cannot fabricate slots, bypass anti-collision checks, or read cross-tenant data.
- **Webhook Idempotency (`chatwoot_processed_messages`)**: Deduplicates retried webhooks by message ID so patients never receive duplicate replies or duplicate bookings.
- **Non-Blocking Processing**: Production webhook calls return `200 OK` immediately while AI tool execution and reply delivery run asynchronously in a locked goroutine per conversation.

---

## 2. Chatwoot & WhatsApp Setup Steps

### Step 1: Create WhatsApp Inboxes in Chatwoot
1. In your Chatwoot Dashboard, navigate to **Settings > Inboxes > Add Inbox**.
2. Choose **WhatsApp** (WhatsApp Cloud API or Twilio) and connect the clinic's WhatsApp Business number.
3. Note the **Account ID** (found in the Chatwoot browser URL `/app/accounts/<ID>/...`) and **Inbox ID** (`/settings/inboxes/<INBOX_ID>`).

### Step 2: Generate a Chatwoot API Access Token
1. In Chatwoot, go to **Profile Settings** (bottom-left user avatar) > **Access Token**.
2. Copy the **API Access Token** (or create an Agent Bot token for that account).

### Step 3: Map Inbox to Provider in Booka Admin Dashboard
1. Log into the clinic's Booka Admin Dashboard (`/t/<clinic-slug>/admin/integrations`).
2. Toggle **WhatsApp Omnichannel (via Chatwoot)** to **ON**.
3. Fill in:
   - **Chatwoot Base URL** (e.g., `https://app.chatwoot.com` or your self-hosted Chatwoot URL)
   - **WhatsApp Business Number** (e.g., `+254712345678`)
   - **Chatwoot Account ID**
   - **Mapped WhatsApp Inbox ID**
   - **API Access Token**
   - *(Optional)* **Webhook Signing Secret**
4. Click **Save All Settings**, then click **Test Chatwoot & Inbox Connection** to verify Booka reaches the Chatwoot API and recognizes the mapped inbox.

### Step 4: Configure Webhook in Chatwoot
1. In Chatwoot, go to **Settings > Integrations > Webhooks > Add new webhook**.
2. Enter the **Global Inbox Router URL**:
   ```text
   https://your-backend-url/webhooks/chatwoot
   ```
   *(Or use the provider-specific URL: `https://your-backend-url/t/<clinic-slug>/webhooks/chatwoot`)*
3. Subscribe to events:
   - `message_created`
   - `conversation_status_changed`
   - `conversation_updated`

---

## 3. Environment Variables

Ensure these variables are set in your deployment environment (Render / backend `.env`):

```env
GROQ_API_KEY=gsk_...
GROQ_MODEL=llama-3.3-70b-versatile
APP_TIMEZONE=Africa/Nairobi

# Optional global fallbacks (overrides per-tenant when tenant keys are blank)
CHATWOOT_BASE_URL=https://app.chatwoot.com
CHATWOOT_API_TOKEN=your_chatwoot_token
```

---

## 4. Controlled AI Tools Catalog (`backend/chatwoot_tools.go`)

| Tool Name | Function |
|---|---|
| `search_providers` | Searches active marketplace providers (`marketplace_enabled=true`) |
| `get_provider_details` | Retrieves clinic hours, phone, address, cancellation & rescheduling policies |
| `search_doctors` | Filters active doctors by specialty, name, or branch |
| `get_doctor_details` | Retrieves doctor bio, working days, hours, branch, and consultation fee |
| `search_specialties` | Lists specialties offered by the clinic |
| `get_provider_branches` | Lists physical hospital/clinic branches |
| `get_available_slots` | Queries real free slots (`availableTimeSlots`) including Google Calendar blocks |
| `get_consultation_fee` | Retrieves exact consultation fee |
| `create_appointment` | Transactionally validates and creates booking (`bookSlot`) + Google Calendar sync |
| `get_patient_appointments` | Fetches upcoming appointments matching patient's WhatsApp phone number |
| `cancel_appointment` | Cancels appointment safely with ownership check & Google Calendar cleanup |
| `reschedule_appointment` | Reschedules appointment transactionally to a new date & time |
| `get_provider_directions` | Returns main facility and branch addresses |
| `transfer_to_human` | Freezes AI state, sets conversation open in Chatwoot, adds internal summary note |

---

## 5. Human Handover & Resumption Flow

1. When a patient sends emergency phrases (`DetectEmergency`) or handover keywords (`human`, `agent`, `receptionist`, `speak to someone`), OR when the AI determines clinical advice/escalation is required:
   - `chat_sessions.human_handover` is set to `true`.
   - Automated AI responses pause for that conversation.
   - Chatwoot receives a private internal note summarizing the patient's phone, name, and reason.
   - Chatwoot toggles conversation status to `open`.
2. **Resuming AI**:
   - Either mark the conversation **Resolved** in Chatwoot (`conversation_status_changed`), OR click **Resume AI Bot** in Booka Admin Dashboard under **WhatsApp Conversation Sessions Monitor**.

---

## 6. Appointment Reminders

- The background worker (`backend/chatwoot_reminders.go`) ticks every 10 minutes.
- Sends automated WhatsApp notifications **24 hours** and **3 hours** prior to confirmed/pending appointments originating from Chatwoot (`chatwoot_conversation_id`).
- Also dispatches instant WhatsApp updates whenever clinic staff confirm, reschedule, or cancel an appointment from the Booka Admin Dashboard.
