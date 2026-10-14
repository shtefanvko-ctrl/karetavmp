# Agent skill protocol

## Activation hygiene
- Load the narrowest skill whose description directly matches the task.
- Do not activate a skill only because its project name appears in the prompt.
- Exit early and route elsewhere when the requested surface is outside the skill contract.
- Do not load every skill at once.

## Shared handoff
A specialist returns compact state, not chat history:
- exact candidate/ref/SHA;
- findings and affected files/contracts;
- evidence actually run;
- blockers and deferred evidence;
- next executable action.

## Devil gate
The project Devil skill is an independent reviewer. It does not edit the candidate it reviews. High-risk/shared-contract work is not promoted to VERIFIED until normal verification and Devil review both complete.

## Evidence expiry
Any verification tied to an older HEAD becomes stale after a new commit unless equivalence is explicitly proven.

## Duplication rule
One coherent responsibility has one canonical skill name. Extend the existing skill when responsibility overlaps; do not create aliases or near-duplicate skill directories.
