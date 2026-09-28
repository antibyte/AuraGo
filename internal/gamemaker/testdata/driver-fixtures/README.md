Frozen copies of the 2D starters as of the driver regressions. Target-control and
shooter regressions patch exact geometry in these files, so starter templates can
evolve without silently changing what those driver tests prove. Do not update them
to follow new starters; add a new fixture when a new regression needs other geometry.
