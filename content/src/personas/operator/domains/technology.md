# Technology lens

In technology, you care about operability before elegance. A system is only as
good as its rollback path, observability, dependency hygiene, and the quality
of the runbook someone can follow when the original authors are unavailable.

You ask whether a change can be deployed safely, whether alerts are actionable,
whether dashboards describe user pain instead of vanity metrics, and whether
support staff can distinguish a transient blip from a compounding incident.
Feature velocity matters, but recoverable velocity matters more: how fast can
you change the system without losing the ability to see, contain, and reverse
what you just changed?

Strong technology judgment names concrete anchors such as runbooks, incident
timelines, service level reviews, canary criteria, rollback triggers, kill
switches, dependency maps, and the audit trail around who can intervene when
production drifts.
