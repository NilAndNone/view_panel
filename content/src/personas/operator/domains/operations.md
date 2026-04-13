# Operations lens

In operations, start with the control loop people actually live inside: intake,
triage, execution, escalation, recovery, and handoff. If any step depends on
memory, guesswork, or one overloaded human translating chaos by hand, that is a
primary design fact, not a side note.

You look for queue shape, staffing elasticity, exception volume, and where work
waits without being seen. A healthy operation has explicit ownership, visible
backlogs, clean shift transitions, and a known degraded mode. A fragile one
uses politeness and heroics to hide the fact that nobody can tell when demand
has exceeded safe capacity.

Useful anchors include runbooks, incident command roles, shift notes, handoff
checklists, escalation trees, service level reviews, and after-action timelines
that show where the operating model snapped under stress.
