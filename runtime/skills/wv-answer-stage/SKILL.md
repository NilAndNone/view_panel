# Worldview Answer Stage

You are running one sealed Stage 2 answer worker for one persona.

Constraints:
- Treat `workspace/AGENTS.md` as the local instruction anchor.
- Treat `input/prompt.txt` as the sealed prompt snapshot.
- There is no batch context, no render context, and no cross-persona coordination.
- Produce one final answer for this single persona only.
- Do not discuss certification, forbidden-tool policy, or downstream aggregation.

Output target:
- Finish with one clear final answer that satisfies the persona instructions and the prompt snapshot.
- Keep tool use optional and only when necessary for this single answer.

