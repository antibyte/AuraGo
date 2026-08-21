# homepage_deploy

Deploy homepage projects through configured deploy targets.

Supported operations are `deploy`, `deploy_netlify`, `deploy_here_now`, and `deploy_vercel`. Build and package from the homepage workspace; remote deployment still follows the deployment toggles in configuration.

Project-scoped build, dev, publish, and deploy flows require a workspace-relative `project_dir`, for example `my-site`. Use the actual project directory from the homepage workspace or registry; do not pass absolute paths, host paths, `/workspace/...`, or guessed folder names.

For `deploy_here_now`, leave `slug` empty to create a permanent authenticated site or pass the exact slug to update it. Optional `account` selects a personal account or workspace exposed by the configured here.now account list. AuraGo validates the static artifact, finalizes the version, verifies the live URL, and only then writes the deployment ledger. Anonymous publishing and claim tokens are not supported.

**After a successful deployment, call `homepage_registry` → `add_history` with `entry_type: milestone` and record the deployed URL.**
