# CPA Usage

CPA Usage provides a human-readable usage dashboard on top of CPA usage data.

## Language

**CPA Key**:
A raw key known by CPA and used to attribute usage events.
_Avoid_: Credential, account

**Key Alias**:
A global user-defined human-readable name for a **CPA Key**.
_Avoid_: Credential alias, model alias, redacted alias

**Cost**:
The calculated spend derived from token usage and local model pricing.
_Avoid_: Price

## Relationships

- A **CPA Key** may have zero or one global **Key Alias**
- A **Key Alias** belongs to exactly one **CPA Key**
- Multiple **CPA Keys** may share the same **Key Alias**.
- Usage attribution remains attached to the **CPA Key**, not to the **Key Alias**
- **Key Aliases** are stored by CPA Usage and are not written back to CPA.
- A **Key Alias** remains available for historical usage even if the **CPA Key** is no longer active in CPA.
- The Credentials view is where users manage **Key Aliases**.
- The first alias management version supports direct editing only, not bulk import or export.
- Saved **Key Alias** edits should immediately affect current dashboard labels without waiting for the next CPA sync.
- Search and filters can match both **Key Alias** and **CPA Key**.
- When a **Key Alias** exists, it is the primary display label and the masked **CPA Key** is secondary traceability text.
- The analytics page is organized around total usage first, then breakdowns by dimensions such as **Key Alias**, model, provider, and health.
- Token volume and **Cost** are peer measurement categories in the dashboard.
- The default analytics breakdown dimensions are **Key Alias**, model, and time.
- Request health appears as a stability breakdown within analytics, not as the primary dashboard story.
- First-version insights are deterministic metrics and warnings, not AI-generated summaries.

## Example dialogue

> **Dev:** "When usage events show a raw **CPA Key**, should the dashboard display the **Key Alias** instead?"
> **Domain expert:** "Yes, the key is still the source of truth, but humans should read the alias."

## Flagged ambiguities

- "key alias" means a human-readable label for a **CPA Key**, not a generated redaction token or a model alias.
