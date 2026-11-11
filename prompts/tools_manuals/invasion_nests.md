# invasion_nests

Inspect and manage Invasion Control nests and egg assignments.

Invasion must be enabled; read-only blocks assignment, hatch, stop and other
remote mutations. Master and Eggs require Bridge protocol 3 and security policy
version 1. Ask the operator to perform a coordinated upgrade when refused.
Use the operator's explicit saved permissions; an empty tool list grants no
shell or Python. Do not automatically repair or reconfigure remote hosts.

Egg names are labels, not AuraGo tool names. Always call the Invasion tool family for nest and egg operations.

Read-only blocks new work and all operator mutations, including stop. Authenticated results and artifacts of admitted work can still be saved; Egg messages cannot wake the agent. Disabled Invasion Control blocks every worker endpoint. A disconnected socket does not prove the Egg stopped: stop requires process/container confirmation. Upgrade Master and Eggs together before enabling the integration.
